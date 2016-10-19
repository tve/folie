package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.bug.st/serial.v1"
)

var (
	port = flag.String("p", "", "serial port (COM*, /dev/cu.*, or /dev/tty*)")
	baud = flag.Int("b", 115200, "serial baud rate")

	tty serial.Port
)

// SerialConnect opens and re-opens a serial port and feeds the receive channel.
func SerialConnect() {
	///ports, err := serial.GetPortsList()
	///check(err)
	///fmt.Printf("Found ports: %v\n", ports)

	for {
		var err error
		tty, err = serial.Open(*port, &serial.Mode{
			BaudRate: *baud,
		})
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// use readline's Stdout to force re-display of current input
		fmt.Fprintln(console.Stdout(), "[connected]")
		var data [250]byte
		for {
			n, err := tty.Read(data[:])
			if err == io.EOF {
				break
			}
			check(err)
			fmt.Printf("<%#v", data[:n])
			serialRecv <- data[:n]
		}
		fmt.Print("\n[disconnected] ")

		tty.Close()
	}
}

// SerialDispatch handles all incoming and outgoing serial data.
func SerialDispatch() {
	go func() {
		for data := range serialSend {
			// FIXME need a way to recover from write-while-closed panics
			fmt.Printf(">%#v", data)
			tty.Write(data)
		}
	}()

	for {
		select {

		case data := <-serialRecv:
			os.Stdout.Write(data)

		case line := <-commandSend:
			if !SpecialCommand(line) {
				serialSend <- []byte(line + "\r")
			}
		}
	}
}

// SpecialCommand recognises and handles certain commands in a different way.
func SpecialCommand(line string) bool {
	cmd := strings.Split(line, " ")
	if len(cmd) > 0 {
		switch cmd[0] {

		case "upload":
			WrappedUpload(cmd[1:])

		default:
			return false
		}
	}
	return true
}

func WrappedUpload(argv []string) {
	// temporarily switch to even parity during upload
	//tty.SetMode(&serial.Mode{BaudRate: *baud, Parity: serial.EvenParity})
	//defer tty.SetMode(&serial.Mode{BaudRate: *baud, Parity: serial.NoParity})

	Uploader(MustAsset("data/mecrisp.bin"))
}
