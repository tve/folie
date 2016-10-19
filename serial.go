package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tarm/serial"
)

var (
	port = flag.String("p", "", "serial port (COM*, /dev/cu.*, or /dev/tty*)")
	baud = flag.Int("b", 115200, "serial baud rate")

	// FIXME - temporary hack, since I can't adjust serial parity on the fly
	upload = flag.String("u", "", "upload the specified firmware, then quit")

	tty        *serial.Port
	uploadMode = false
	uploadDone = make(chan bool, 0)
)

// SerialConnect opens and re-opens a serial port and feeds the receive channel.
func SerialConnect() {
	for {
		var err error
		config := serial.Config{
			Name: *port,
			Baud: *baud,
			ReadTimeout: time.Second,
		}
		if uploadMode {
			config.Parity = serial.ParityEven
		}
		tty, err = serial.OpenPort(&config)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if uploadMode {
			// upload needs a serial port, reopened with even parity
			// close and reopen without parity once it is done
			<-uploadDone
			uploadMode = false
		} else {
			// use readline's Stdout to force re-display of current input
			fmt.Fprintln(console.Stdout(), "[connected]")
			for {
				data := make([]byte, 250)
				fmt.Println("read")
				n, err := tty.Read(data)
				fmt.Println(err)
				if err == io.EOF {
					break
				}
				check(err)
				serialRecv <- data[:n]
			}
			fmt.Print("\n[disconnected] ")
		}

		if tty != nil {
			tty.Close()
		}
	}
}

// SerialDispatch handles all incoming and outgoing serial data.
func SerialDispatch() {
	for {
		select {

		case data := <-serialRecv:
			os.Stdout.Write(data)

		case cmd := <-commandSend:
			// FIXME need a way to recover from write-while-closed panics
			if !SpecialCommand(cmd) {
				tty.Write([]byte(cmd + "\r"))
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
			uploadMode = true
			t := tty
			tty = nil
			t.Close()
			Uploader(MustAsset("data/mecrisp.bin"), tty)

		default:
			return true
		}
	}
	return false
}
