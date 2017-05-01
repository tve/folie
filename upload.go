package main

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	ACK        = 0x79
	NAK        = 0x1F
	GET_CMD    = 0x00
	GETID_CMD  = 0x02
	GO_CMD     = 0x21
	WRITE_CMD  = 0x31
	ERASE_CMD  = 0x43
	EXTERA_CMD = 0x44
	WRUNP_CMD  = 0x73
	RDUNP_CMD  = 0x92
)

var (
	checkSum byte   // upload protocol checksum
	pending  []byte // data received while waiting for line echo
	extended bool   // flash erase uses extended mode
)

// Uploader implements the STM32 usart boot protocol to upload new firmware.
func Uploader(data []byte) {
	// convert to binary if first few bytes look like they are in "ihex" format
	if len(data) > 11 && data[0] == ':' {
		_, err := hex.DecodeString(string(data[1:11]))
		if err == nil {
			data = HexToBin(data)
		}
	}
	fmt.Printf("  %db ", len(data))
	defer fmt.Println()

	connectToTarget()

	fmt.Printf("V%02X ", getBootVersion())
	fmt.Printf("#%04X ", getChipType())

	sendCmd(RDUNP_CMD)
	wantAck(20)
	fmt.Print("R ")

	connectToTarget()

	sendCmd(WRUNP_CMD)
	wantAck(0)
	fmt.Print("W ")

	connectToTarget()

	if extended {
		// assumes L0xx (0x417), which has 128-byte pages
		pages := (len(data) + 127) / 128
		massErase(pages)
		fmt.Printf("E%d* ", pages)
	} else {
		massErase(0)
		fmt.Print("E ")
	}

	writeFlash(data)
	fmt.Print("done.")
}

func readWithTimeout(t time.Duration) []byte {
	select {
	case data := <-serialRecv:
		return data
	case <-time.After(t):
		//fmt.Println("timeout")
		return nil
	}
}

func sendByte(b uint8) {
	if *verbose {
		fmt.Printf(">%02X", b)
	}
	if !*raw && b == Iac {
		serialSend <- []byte{b, b}
	} else {
		serialSend <- []byte{b}
	}
	checkSum ^= b
}

func send2bytes(v int) {
	sendByte(uint8(v >> 8))
	sendByte(uint8(v))
}

func send4bytes(v int) {
	send2bytes(v >> 16)
	send2bytes(v)
}

func getReply() uint8 {
	b := byte(0)
	if len(pending) == 0 {
		timeout := 250 * time.Millisecond
		if tty == nil {
			timeout = 3 * time.Second // more patience for telnet, wifi, etc
		}
		pending = readWithTimeout(timeout)
	}
	if len(pending) > 0 {
		b = pending[0]
		if *verbose {
			fmt.Printf("<%02X#%d", b, len(pending))
		}
		pending = pending[1:]
	}
	return b
}

func wantAck(retries int) {
	r := getReply()
	for retries > 0 && r == 0 {
		r = getReply()
		retries -= 1
	}
	if r != ACK {
		fmt.Printf("\nFailed: %02X\n", r)
	}
	checkSum = 0
}

func sendCmd(cmd uint8) {
	readWithTimeout(50 * time.Millisecond)
	pending = nil

	sendByte(cmd)
	sendByte(^cmd)
	pending = nil
	wantAck(0)
}

func connectToTarget() {
	for {
		boardReset(true) // reset with BOOT0 high to enter boot loader
		time.Sleep(100 * time.Millisecond)
		sendByte(0x7F)
		r := getReply()
		if r == ACK || r == NAK {
			if r == ACK {
				fmt.Print("+")
			}
			break
		}
		fmt.Print(".") // connecting...
	}
}

func getBootVersion() uint8 {
	sendCmd(GET_CMD)
	n := getReply()
	rev := getReply()
	extended = false
	for i := 0; i < int(n); i++ {
		if getReply() == EXTERA_CMD {
			extended = true
		}
	}
	wantAck(0)
	return rev
}

func getChipType() uint16 {
	sendCmd(GETID_CMD)
	getReply() // should be 1
	chipType := uint16(getReply()) << 8
	chipType |= uint16(getReply())
	wantAck(0)
	return chipType
}

func massErase(pages int) {
	if extended {
		sendCmd(EXTERA_CMD)
		// for some reason, a "full" mass erase is rejected with a NAK
		//send2bytes(0xFFFF)
		// ... so erase a list of segments instead, 1 more than needed
		// this will only erase the pages to be programmed!
		send2bytes(pages - 1)
		for i := 0; i < pages; i++ {
			send2bytes(i)
		}
		sendByte(checkSum)
	} else {
		sendCmd(ERASE_CMD)
		sendByte(0xFF)
		sendByte(0x00)
	}
	wantAck(10)
}

func writeFlash(data []byte) {
	origVerbose := *verbose
	defer func() { *verbose = origVerbose }()

	fmt.Print("writing: ")
	eraseCount := 0
	for offset := 0; offset < len(data); offset += 256 {
		for i := 0; i < eraseCount; i++ {
			fmt.Print("\b")
		}
		msg := fmt.Sprintf("%d/%d ", offset/256+1, (len(data)+255)/256)
		fmt.Print(msg)
		eraseCount = len(msg)

		sendCmd(WRITE_CMD)
		send4bytes(0x08000000 + offset)
		sendByte(checkSum)
		wantAck(0)
		sendByte(256 - 1)
		for i := 0; i < 256; i++ {
			if offset+i < len(data) {
				sendByte(data[offset+i])
			} else {
				sendByte(0xFF)
			}
		}
		sendByte(checkSum)
		wantAck(0)
		*verbose = false // reduce debug output after the first page write
	}
}

// HexToBin converts an Intel-hex format file to binary.
func HexToBin(data []byte) []byte {
	var bin []byte
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasSuffix(line, "\r") {
			line = line[:len(line)-1]
		}
		if len(line) == 0 {
			continue
		}
		if line[0] != ':' || len(line) < 11 {
			fmt.Println("Not ihex format:", line)
			return data
		}
		bytes, err := hex.DecodeString(line[1:])
		check(err)
		if bytes[3] != 0x00 {
			continue
		}
		offset := (int(bytes[1]) << 8) + int(bytes[2])
		length := bytes[0]
		for offset > len(bin) {
			bin = append(bin, 0xFF)
		}
		bin = append(bin, bytes[4:4+length]...)
	}
	return bin
}
