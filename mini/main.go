package main

import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"go.bug.st/serial.v1"
)

var (
	device      serial.Port
	console     *readline.Instance
	conOut      io.Writer
	serialRecv  = make(chan []byte)
	commandRecv = make(chan string)
)

func main() {
	log.SetFlags(0) // omit timestamps

	var err error
	console, err = readline.New("? ")
	check(err)
	conOut = console.Stdout()

	port := selectPort()

	console.SetPrompt("> ")

	device, err = serial.Open(port, &serial.Mode{BaudRate: 115200})
	check(err)

	go serialInput()
	go consoleInput()

	for {
		select {
		case data := <-serialRecv:
			fmt.Fprint(conOut, string(data))
		case line := <-commandRecv:
			device.Write([]byte(line + "\r"))
			reply := getReply()
			//fmt.Fprintf(conOut, "%q\n", reply)
			if strings.HasPrefix(reply, line+" ") {
				reply = reply[len(line)+1:]
			}
			if strings.HasSuffix(reply, " ok.\n") {
				reply = reply[:len(reply)-5] + "\n"
			}
			if reply != "\n" {
				fmt.Fprint(conOut, reply)
			}
		}
	}
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func selectPort() string {
	allPorts, err := serial.GetPortsList()
	check(err)

	var ports []string
	for _, p := range allPorts {
		if !strings.HasPrefix(p, "/dev/tty.") {
			ports = append(ports, p)
			fmt.Printf("%3d: %s\n", len(ports), p)
		}
	}

	console.Refresh()
	reply, err := console.Readline()
	check(err)

	sel, err := strconv.Atoi(reply)
	check(err)

	return ports[sel-1]
}

func serialInput() {
	for {
		data := make([]byte, 250)
		n, err := device.Read(data)
		if n > 0 {
			serialRecv <- data[:n]
		}
		check(err)
	}
}

func consoleInput() {
	for {
		line, err := console.Readline()
		check(err)
		commandRecv <- line
	}
}

func getReply() (reply string) {
	for {
		select {
		case data := <-serialRecv:
			reply += string(data)
			if strings.HasSuffix(reply, " ok.\n") ||
				strings.HasSuffix(reply, " not found.\n") ||
				strings.HasSuffix(reply, " is compile-only.\n") ||
				strings.HasSuffix(reply, " Stack underflow\n") {
				return
			}
		case <-time.After(50 * time.Millisecond):
			reply += "[timeout]\n"
			return
		}
	}
	return
}
