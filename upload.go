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
	fmt.Printf("  Uploading %d bytes: ", len(data))
	connectToTarget()
	fmt.Println("\nyeay!")
	connectToTarget()
	fmt.Println("\nyeay again!")
	fmt.Printf("boot:%02x ", getBootVersion())
}

func readWithTimeout(t time.Duration) []byte {
	select {
	case data := <-serialRecv:
		return data
	case <-time.After(t):
		fmt.Println("timeout")
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

func sendCmd(cmd uint8) {
	sendByte(cmd)
	sendByte(^cmd)
	pending = nil
	wantAck()
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

func wantAck() {
	//for getReply() != ACK {
	//	fmt.Print("-")
	//}
	r := getReply()
	fmt.Println("reply:", r)
	if r != ACK {
		fmt.Printf("\nFailed: %02x\n", r)
		//panic(nil)
	}
	checkSum = 0
}

func connectToTarget() {
	for {
		fmt.Print(".") // auto-baud greeting
		sendByte(0x7F)
		r := getReply()
		if r == ACK || r == NAK {
			if r == ACK {
				fmt.Print("+")
			}
			break
		}
		time.Sleep(time.Second)
	}
	// got a valid reply, flush
	//readWithTimeout(100 * time.Millisecond)
	//pending = nil
}

func getBootVersion() uint8 {
	sendCmd(GET_CMD)
	n := getReply()
	fmt.Println("bootreply", n)
	rev := getReply()
	extended = false
	for i := 0; i < int(n); i++ {
		if getReply() == EXTERA_CMD {
			extended = true
		}
	}
	wantAck()
	return rev
}
