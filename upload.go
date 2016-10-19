package main

import (
	"fmt"
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
	checkSum byte
	pending  []byte
	extended bool
)

func Uploader(data []byte) {
	defer fmt.Println()
	fmt.Printf("%d bytes: ", len(data))
	connectToTarget()
	fmt.Printf(" v%02x ", getBootVersion())
	fmt.Printf("#%04x ", getChipType())
	massErase()
	fmt.Print("* ")
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
		fmt.Printf(">%02x", b)
	}
	serialSend <- []byte{b}
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
		pending = readWithTimeout(time.Second)
	}
	if len(pending) > 0 {
		b = pending[0]
		if *verbose {
			fmt.Printf("<%02x", b)
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
		fmt.Printf("\nFailed: %02x\n", r)
	}
	checkSum = 0
}

func sendCmd(cmd uint8) {
	sendByte(cmd)
	sendByte(^cmd)
	pending = nil
	wantAck(0)
}

func connectToTarget() {
	for {
		sendByte(0x7F)
		r := getReply()
		if r == ACK || r == NAK {
			if r == ACK {
				fmt.Print("+")
			}
			break
		}
		fmt.Print(".") // connecting...
		time.Sleep(time.Second)
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
			fmt.Print("e")
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

func massErase() {
	sendCmd(ERASE_CMD)
	sendByte(0xFF)
	sendByte(checkSum)
	wantAck(4)
}

func writeFlash(data []byte) {
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
		*verbose = false // turn verbose off after one write to reduce output
	}
}
