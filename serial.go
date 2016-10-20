package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial.v1"
)

var (
	port = flag.String("p", "", "serial port (COM*, /dev/cu.*, or /dev/tty*)")
	baud = flag.Int("b", 115200, "serial baud rate")

	tty       serial.Port
	openBlock = make(chan string)
)

// SerialConnect opens and re-opens a serial port and feeds the receive channel.
func SerialConnect() {
	if *port == "" {
		commandSend <- "<open>"
		*port = <-openBlock
	}

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
	go func() {
		for data := range serialSend {
			// FIXME need a way to recover from write-while-closed panics
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

		case "<open>":
			// TODO can't be typed in to re-open, only usable on startup
			WrappedOpen(cmd)

		case "upload":
			fmt.Print(line, " ")
			WrappedUpload(cmd)

		default:
			return false
		}
	}
	return true
}

func WrappedOpen(argv []string) {
	allPorts, err := serial.GetPortsList()
	check(err)

	var ports []string
	for _, p := range allPorts {
		if !strings.HasPrefix(p, "/dev/tty.") {
			ports = append(ports, p)
		}
	}

	fmt.Println("Select a serial port:")
	for i, p := range ports {
		fmt.Println("  ", i+1, "=", p)
	}
	reply := <-commandSend
	fmt.Println(reply)

	sel, err := strconv.Atoi(reply)
	check(err)
	openBlock <- ports[sel-1]
}

func WrappedUpload(argv []string) {
	name := "blink"
	if len(argv) > 1 {
		name = argv[1]
	}

	// try built-in images first
	data, err := Asset("data/" + name + ".bin")
	if err != nil {
		// else try opening the arg as file
		f, err := os.Open(name)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer f.Close()

		data, err = ioutil.ReadAll(f)
		check(err)
	}

	// temporarily switch to even parity during upload
	tty.SetMode(&serial.Mode{BaudRate: *baud, Parity: serial.EvenParity})
	defer tty.SetMode(&serial.Mode{BaudRate: *baud})

	Uploader(data)
}
