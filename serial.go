package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tarm/serial"
)

var (
	port = flag.String("p", "", "serial port (usually /dev/tty* or COM*)")
	baud = flag.Int("b", 115200, "serial baud rate")
)

// SerialConnect open and re-opens a serial port and feeds the two channels.
func SerialConnect() {
	var tty *serial.Port

	go func() {
		for data := range commandOut {
			// FIXME need a way to recover from write-while-closed panics
			tty.Write([]byte(data + "\r"))
		}
	}()

	for {
		var err error
		config := serial.Config{Name: *port, Baud: *baud}
		tty, err = serial.OpenPort(&config)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// by using readline's Stdout, we can force re-display of current input
		fmt.Fprintln(console.Stdout(), "[connected]")
		for {
			data := make([]byte, 250)
			n, err := tty.Read(data)
			if err == io.EOF {
				break
			}
			check(err)
			serialIn <- data[:n]
		}
		fmt.Print("\n[disconnected] ")

		tty.Close()
	}
}

// SerialDispatch handles incoming serial data and decides what to do with it.
func SerialDispatch() {
	for data := range serialIn {
		os.Stdout.Write(data)
	}
}
