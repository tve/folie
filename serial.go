package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"go.bug.st/serial.v1"
)

var (
	port = flag.String("p", "", "serial port (COM*, /dev/cu.*, or /dev/tty*)")
	baud = flag.Int("b", 115200, "serial baud rate")
	raw  = flag.Bool("r", false, "use raw instead of telnet protocol")

	tty       serial.Port        // only used for serial connections
	dev       io.ReadWriteCloser // used for both serial and tcp connections
	tnState   int                // tracks telnet protocol state when reading
	openBlock = make(chan string)
)

func blockUntilOpen() {
	for {
		var err error
		if _, err = os.Stat(*port); os.IsNotExist(err) &&
			strings.Count(*port, ":") == 1 && !strings.HasSuffix(*port, ":") {
			// if nonexistent, it's an ip addr + port, open it as network port
			dev, err = net.Dial("tcp", *port)
		} else {
			tty, err = serial.Open(*port, &serial.Mode{
				BaudRate: *baud,
			})
			dev = tty
		}
		if err == nil {
			break
		}
		fmt.Println(err)
		time.Sleep(500 * time.Millisecond)
	}

	// use readline's Stdout to force re-display of current input
	fmt.Fprintf(console.Stdout(), "[connected to %s]\n", *port)

	if !*raw {
		telnetInit()
	}
}

// SerialConnect opens and re-opens a serial port and feeds the receive channel.
func SerialConnect() {
	if *port == "" {
		commandSend <- "<open>"
		*port = <-openBlock
	}

	for {
		tnState = 0 // clear telnet state before anything comes in

		blockUntilOpen()

		for {
			data := make([]byte, 250)
			n, err := dev.Read(data)
			if err != nil {
				break
			}
			if !*raw {
				n = telnetClean(data, n)
			}
			serialRecv <- data[:n]
		}
		fmt.Print("\n[disconnected] ")

		dev.Close()
		dev = nil
		tty = nil
	}
}

// SerialDispatch handles all incoming and outgoing serial data.
func SerialDispatch() {
	// outbound serial will be slowed down just a tad for Mecrisp/USB
	go func() {
		for data := range serialSend {
			for len(data) > 0 {
				out := data
				if len(out) > 60 {
					out = out[:60] // send as separate chunks of under 64 bytes
				}
				data = data[len(out):]

				if dev == nil { // avoid write-while-closed panics
					fmt.Printf("[CAN'T WRITE! %s]\n", *port)
					blockUntilOpen()
				} else if _, err := dev.Write(out); err != nil {
					fmt.Printf("[WRITE ERROR! %s]\n", *port)
					dev.Close()
					blockUntilOpen()
				} else if len(data) > 0 {
					// when chunked, add a brief delay to force separate sends
					time.Sleep(2 * time.Millisecond)
				}
			}
		}
	}()

	for {
		select {

		case data := <-serialRecv:
			if *verbose {
				fmt.Printf("recv: %q\n", data)
			}
			os.Stdout.Write(data)

		case line := <-commandSend:
			if !SpecialCommand(line) {
				data := []byte(line + "\r")
				if *verbose {
					fmt.Printf("send: %q\n", data)
				}
				serialSend <- data
			}
		}
	}
}

// SpecialCommand recognises and handles certain commands in a different way.
func SpecialCommand(line string) bool {
	cmd := strings.SplitN(line, " ", 2)
	if len(cmd) > 0 {
		switch cmd[0] {

		case "<open>":
			// TODO can't be typed in to re-open, only usable on startup
			wrappedOpen(cmd)

		case "!c", "!cd":
			fmt.Println(line)
			wrappedCd(cmd)

		case "!h", "!help":
			fmt.Println(line)
			showHelp()

		case "!l", "!ls":
			fmt.Println(line)
			wrappedLs(cmd)

		case "!r", "!reset":
			fmt.Println(line)
			wrappedReset()

		case "!s", "!send":
			fmt.Println(line)
			wrappedSend(cmd)

		case "!u", "!upload":
			fmt.Println(line)
			wrappedUpload(cmd)

		default:
			return false
		}
	}
	return true
}

const helpMsg = `
Special commands, these can also be abbreviated as "!r", etc:
  !reset          reset the board, same as ctcl-c (ignored in raw mode)
  !send <file>    send text file to the serial port, expand "include" lines
  !upload         show the list of built-in firmware images
  !upload <n>     upload built-in image <n> using STM32 boot protocol
  !upload <file>  upload specified firmware image (bin or hex format)
  !upload <url>   fetch firmware image from given URL, then upload it
Utility commands:
  !cd <dir>       change directory (or list current one if not specified)
  !ls <dir>       list contents of the specified (or current) directory
  !help           this message
To quit, hit ctrl-d. For command history, use up-/down-arrow.
`

func showHelp() {
	fmt.Print(helpMsg[1:])
}
