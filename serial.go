package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
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
		conn, err := serial.Open(*port, &serial.Mode{
			BaudRate: *baud,
		})
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		tty = conn

		// use readline's Stdout to force re-display of current input
		fmt.Fprintf(console.Stdout(), "[connected to %s]\n", *port)
		for {
			data := make([]byte, 250)
			n, err := tty.Read(data)
			if err == io.EOF || err == syscall.ENXIO {
				break
			}
			check(err)
			serialRecv <- data[:n]
		}
		fmt.Print("\n[disconnected] ")

		tty.Close()
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

				if tty == nil { // avoid write-while-closed panics
					fmt.Printf("[CAN'T WRITE! %s]\n", *port)
				} else if _, err := tty.Write(out); err != nil {
					fmt.Printf("[WRITE ERROR! %s]\n", *port)
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

		case "!h", "!help":
			fmt.Println(line)
			showHelp()

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
Special commands interpreted by folie, can also be abbreviated as "!h", etc:
  !help           this message
  !send <file>    send text file to the serial port, expand "include" lines
  !upload         show the list of built-in firmware images
  !upload <n>     upload built-in image <n> using STM32 boot protocol
  !upload <file>  upload specified firmware image (bin or hex format)
  !upload <url>   fetch firmware image from given URL, then upload it
To quit, hit ctrl-d or ctrl-c. For command history, use up-/down-arrow.
`

func showHelp() {
	fmt.Print(helpMsg[1:])
}

func wrappedOpen(argv []string) {
	allPorts, err := serial.GetPortsList()
	check(err)

	var ports []string
	for _, p := range allPorts {
		if !strings.HasPrefix(p, "/dev/tty.") {
			ports = append(ports, p)
		}
	}
	//sort.Strings(ports)

	if len(ports) == 0 {
		done <- fmt.Errorf("No serial ports found.")
		return
	}

	sel := 1
	if len(ports) > 1 {
		fmt.Println("Select the serial port:")
		for i, p := range ports {
			fmt.Printf("%3d: %s\n", i+1, p)
		}
		console.SetPrompt("? ")
		console.Refresh()
		reply := <-commandSend
		console.SetPrompt("")
		fmt.Println(reply)

		sel, _ = strconv.Atoi(reply)
	}

	// quit on index errors, since we have no other useful choice!
	defer func() {
		if e := recover(); e != nil {
			done <- nil // forces quit without producing an error message
			return
		}
		fmt.Println("Enter '!help' for additional help, or ctrc-d to quit.")
	}()

	openBlock <- ports[sel-1]
}

func wrappedSend(argv []string) {
	if len(argv) == 1 {
		fmt.Printf("Usage: %s <filename>\n", argv[0])
		return
	}
	if !IncludeFile(argv[1], 0) {
		fmt.Println("Send failed.")
	}
}

var crcTable = []uint16{
	0x0000, 0xCC01, 0xD801, 0x1400, 0xF001, 0x3C00, 0x2800, 0xE401,
	0xA001, 0x6C00, 0x7800, 0xB401, 0x5000, 0x9C01, 0x8801, 0x4400,
}

func crc16(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for b := range data {
		crc = (crc >> 4) ^ crcTable[crc&0x0F] ^ crcTable[b&0x0F]
		crc = (crc >> 4) ^ crcTable[crc&0x0F] ^ crcTable[(b>>4)&0x0F]
	}
	return crc
}

func wrappedUpload(argv []string) {
	names := AssetNames()
	sort.Strings(names)

	if len(argv) == 1 {
		fmt.Println("These firmware images are built-in:")
		for i, name := range names {
			data, _ := Asset(name)
			fmt.Printf("%3d: %-16s %5db  crc:%04X\n",
				i+1, name, len(data), crc16(data))
		}
		fmt.Println("Use '!u <n>' to upload a specific one.")
		return
	}

	// try built-in images first, indicated by entering a valid number
	var data []byte
	if n, err := strconv.Atoi(argv[1]); err == nil && 0 < n && n <= len(names) {
		data, _ = Asset(names[n-1])
	} else if _, err := url.Parse(argv[1]); err == nil {
		fmt.Print("Fetching... ")
		res, err := http.Get(argv[1])
		if err == nil {
			data, err = ioutil.ReadAll(res.Body)
			res.Body.Close()
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("got it, crc:%04X\n", crc16(data))
	} else { // else try opening the arg as file
		f, err := os.Open(argv[1])
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
