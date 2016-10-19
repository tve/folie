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
	// FIXME: added as hack for now, until we can turn parity on and off
	upload = flag.String("u", "", "upload a file and quit")

	tty *serial.Port
)

// SerialConnect opens and re-opens a serial port and feeds the receive channel.
func SerialConnect() {
	///ports, err := serial.GetPortsList()
	///check(err)
	///fmt.Printf("Found ports: %v\n", ports)

	for {
		var err error
		config := &serial.Config{Name: *port, Baud: *baud}
		if *upload != "" {
			config.Parity = serial.ParityEven
		}
		tty, err = serial.OpenPort(config)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// use readline's Stdout to force re-display of current input
		fmt.Fprintln(console.Stdout(), "[connected]")
		if *upload != "" {
			commandSend <- "upload " + *upload
		}
		for {
			data := make([]byte, 250)
			n, err := tty.Read(data)
			if err == io.EOF {
				break
			}
			check(err)
			//fmt.Printf("<%#v", data[:n])
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
			//fmt.Printf(">%#v", data)
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
			fmt.Print(line, " ")
			WrappedUpload(cmd)

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

	name := "blink"
	if len(argv) > 1 {
		name = argv[1]
	}
	data, err := Asset("data/" + name + ".bin")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	Uploader(data)
	// FIXME, see the "-u" flag
	fmt.Print("Press return... ")
	console.Close()
	fmt.Println("Press return... ")
	os.Exit(0)
}
