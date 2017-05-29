package folie

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
)

// This file contains code to flash a microcontroller with a hex/binary image.

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

var Verbose bool

type Uploader struct {
	Tx     MicroConn
	Rx     <-chan []byte
	Stdout io.Writer

	checkSum byte   // upload protocol checksum
	pending  []byte // data received while waiting for line echo
	extended bool   // flash erase uses extended mode
}

// Uploader implements the STM32 usart boot protocol to upload new firmware.
func (u *Uploader) Upload(data []byte) {
	// convert to binary if first few bytes look like they are in "ihex" format
	if len(data) > 11 && data[0] == ':' {
		_, err := hex.DecodeString(string(data[1:11]))
		if err == nil {
			data = u.hexToBin(data)
		}
	}
	fmt.Fprintf(u.Stdout, "  %db ", len(data))
	defer fmt.Fprintln(u.Stdout)

	u.connectToTarget()

	fmt.Fprintf(u.Stdout, "V%02X ", u.getBootVersion())
	fmt.Fprintf(u.Stdout, "#%04X ", u.getChipType())

	u.sendCmd(RDUNP_CMD)
	u.wantAck(20)
	fmt.Fprint(u.Stdout, "R ")

	u.connectToTarget()

	u.sendCmd(WRUNP_CMD)
	u.wantAck(0)
	fmt.Fprint(u.Stdout, "W ")

	u.connectToTarget()

	if u.extended {
		// assumes L0xx (0x417), which has 128-byte pages
		pages := (len(data) + 127) / 128
		u.massErase(pages)
		fmt.Fprintf(u.Stdout, "E%d* ", pages)
	} else {
		u.massErase(0)
		fmt.Fprint(u.Stdout, "E ")
	}

	u.writeFlash(data)
	fmt.Fprint(u.Stdout, "done.")
}

func (u *Uploader) readWithTimeout(t time.Duration) []byte {
	select {
	case data := <-u.Rx:
		return data
	case <-time.After(t):
		//fmt.Fprintln(u.Stdout, "timeout")
		return nil
	}
}

func (u *Uploader) sendByte(b uint8) {
	if Verbose {
		fmt.Fprintf(u.Stdout, ">%02X", b)
	}
	u.Tx.Write([]byte{b})
	u.checkSum ^= b
}

func (u *Uploader) send2bytes(v int) {
	u.sendByte(uint8(v >> 8))
	u.sendByte(uint8(v))
}

func (u *Uploader) send4bytes(v int) {
	u.send2bytes(v >> 16)
	u.send2bytes(v)
}

func (u *Uploader) getReply() uint8 {
	b := byte(0)
	if len(u.pending) == 0 {
		timeout := time.Second
		/* what do we gain by reducing the timeout for local serial?
		timeout := 250 * time.Millisecond
		if tty == nil {
			timeout = 3 * time.Second // more patience for telnet, wifi, etc
		}*/
		u.pending = u.readWithTimeout(timeout)
	}
	if len(u.pending) > 0 {
		b = u.pending[0]
		if Verbose {
			fmt.Fprintf(u.Stdout, "<%02X#%d", b, len(u.pending))
		}
		u.pending = u.pending[1:]
	}
	return b
}

func (u *Uploader) wantAck(retries int) {
	r := u.getReply()
	for retries > 0 && r == 0 {
		r = u.getReply()
		retries -= 1
	}
	if r != ACK {
		fmt.Fprintf(u.Stdout, "\nFailed: %02X\n", r)
	}
	u.checkSum = 0
}

func (u *Uploader) sendCmd(cmd uint8) {
	u.readWithTimeout(50 * time.Millisecond)
	u.pending = nil

	u.sendByte(cmd)
	u.sendByte(^cmd)
	u.pending = nil
	u.wantAck(0)
}

func (u *Uploader) connectToTarget() {
	for {
		u.Tx.Reset(true) // reset with BOOT0 high to enter boot loader
		time.Sleep(100 * time.Millisecond)
		u.sendByte(0x7F)
		r := u.getReply()
		if r == ACK || r == NAK {
			if r == ACK {
				fmt.Fprint(u.Stdout, "+")
			}
			break
		}
		fmt.Fprint(u.Stdout, ".") // connecting...
	}
}

func (u *Uploader) getBootVersion() uint8 {
	u.sendCmd(GET_CMD)
	n := u.getReply()
	rev := u.getReply()
	u.extended = false
	for i := 0; i < int(n); i++ {
		if u.getReply() == EXTERA_CMD {
			u.extended = true
		}
	}
	u.wantAck(0)
	return rev
}

func (u *Uploader) getChipType() uint16 {
	u.sendCmd(GETID_CMD)
	u.getReply() // should be 1
	chipType := uint16(u.getReply()) << 8
	chipType |= uint16(u.getReply())
	u.wantAck(0)
	return chipType
}

func (u *Uploader) massErase(pages int) {
	if u.extended {
		u.sendCmd(EXTERA_CMD)
		// for some reason, a "full" mass erase is rejected with a NAK
		//u.send2bytes(0xFFFF)
		// ... so erase a list of segments instead, 1 more than needed
		// this will only erase the pages to be programmed!
		u.send2bytes(pages - 1)
		for i := 0; i < pages; i++ {
			u.send2bytes(i)
		}
		u.sendByte(u.checkSum)
	} else {
		u.sendCmd(ERASE_CMD)
		u.sendByte(0xFF)
		u.sendByte(0x00)
	}
	u.wantAck(10)
}

func (u *Uploader) writeFlash(data []byte) {
	origVerbose := Verbose
	defer func() { Verbose = origVerbose }()

	fmt.Fprint(u.Stdout, "writing: ")
	eraseCount := 0
	for offset := 0; offset < len(data); offset += 256 {
		for i := 0; i < eraseCount; i++ {
			fmt.Fprint(u.Stdout, "\b")
		}
		msg := fmt.Sprintf("%d/%d ", offset/256+1, (len(data)+255)/256)
		fmt.Fprint(u.Stdout, msg)
		eraseCount = len(msg)

		u.sendCmd(WRITE_CMD)
		u.send4bytes(0x08000000 + offset)
		u.sendByte(u.checkSum)
		u.wantAck(0)
		u.sendByte(256 - 1)
		for i := 0; i < 256; i++ {
			if offset+i < len(data) {
				u.sendByte(data[offset+i])
			} else {
				u.sendByte(0xFF)
			}
		}
		u.sendByte(u.checkSum)
		u.wantAck(0)
		Verbose = false // reduce debug output after the first page write
	}
}

// hexToBin converts an Intel-hex format file to binary.
func (u *Uploader) hexToBin(data []byte) []byte {
	var bin []byte
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasSuffix(line, "\r") {
			line = line[:len(line)-1]
		}
		if len(line) == 0 {
			continue
		}
		if line[0] != ':' || len(line) < 11 {
			fmt.Fprintln(u.Stdout, "Not ihex format:", line)
			return data
		}
		bytes, err := hex.DecodeString(line[1:])
		if err != nil {
			fmt.Fprintln(u.Stdout, "Not ihex format:", line)
			return data
		}
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
