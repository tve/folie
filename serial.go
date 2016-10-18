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

	tty *serial.Port
)

// SerialConnect opens and re-opens a serial port and feeds the receive channel.
func SerialConnect() {
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
			serialRecv <- data[:n]
		}
		fmt.Print("\n[disconnected] ")

		tty.Close()
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
			tty.Write([]byte(cmd + "\r"))
		}
	}
}
