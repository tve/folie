package main

import (
	//"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial.v1"
)

var (
	port = flag.String("p", "", "serial port (COM*, /dev/cu.*, or /dev/tty*)")
	baud = flag.Int("b", 115200, "serial baud rate")
	raw  = flag.Bool("r", false, "use raw instead of telnet protocol")

	tty     serial.Port        // only used for serial connections
	dev     io.ReadWriteCloser // used for both serial and tcp connections
	tnState int                // tracks telnet protocol state when reading
)

func selectPort() string {
	allPorts, err := serial.GetPortsList()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}

	var ports []string
	for _, p := range allPorts {
		if !strings.HasPrefix(p, "/dev/tty.") {
			ports = append(ports, p)
		}
	}
	//sort.Strings(ports)

	if len(ports) == 0 {
		fmt.Fprintln(os.Stderr, "No serial ports found.")
		return ""
	}

	fmt.Fprintln(console.Stdout(), "Select the serial port:")
	for i, p := range ports {
		fmt.Fprintf(console.Stdout(), "%3d: %s\n", i+1, p)
	}
	console.SetPrompt("? ")
	console.Refresh()
	reply, _ := console.Readline()
	console.SetPrompt("")
	fmt.Println(reply)

	sel, _ := strconv.Atoi(reply)

	// quit on index errors, since we have no other useful choice
	defer func() {
		if e := recover(); e != nil {
			return
		}
		fmt.Println("Enter '!help' for additional help, or ctrl-d to quit.")
	}()

	return ports[sel-1]
}

func boardReset(enterBoot bool) {
	if !*raw {
		telnetReset(enterBoot)
	} else if tty != nil {
		tty.SetDTR(true)
		tty.SetRTS(!enterBoot)
		time.Sleep(time.Millisecond)
		tty.SetDTR(false)
	}
	time.Sleep(time.Millisecond)
}

func blockUntilOpen() {
	var lastErr string
	for {
		var err error
		if _, err = os.Lstat(*port); os.IsNotExist(err) &&
			strings.Count(*port, ":") == 1 && !strings.HasSuffix(*port, ":") {
			// if nonexistent, it's an ip addr + port, open it as network port
			dev, err = net.Dial("tcp", *port)
		} else {
			tty, err = serial.Open(*port, &serial.Mode{
				BaudRate: *baud,
			})
			dev = tty
			if runtime.GOOS == "linux" {
				switchToByPathDev("/dev/serial/by-path/")
			}
		}
		if err == nil {
			break
		}
		if err.Error() != lastErr {
			fmt.Println(err)
			lastErr = err.Error()
		}
		time.Sleep(500 * time.Millisecond)
	}
	serialRecv = make(chan []byte)

	go SerialDispatch()

	// use readline's Stdout to force re-display of current input
	fmt.Fprintf(console.Stdout(), "[connected to %s]\n", path.Base(*port))

	if !*raw {
		telnetInit()
	} else {
		tty.SetRTS(true)
		tty.SetDTR(false)
	}
}

// switchToByPathDev allows re-opening a device even if its name changes
func switchToByPathDev(prefix string) {
	if dir, err := os.Open(prefix); err == nil {
		names, _ := dir.Readdirnames(-1)
		// look for an entry matching the current serial device name
		for _, name := range names {
			alias := prefix + name
			link, e := os.Readlink(alias)
			if e == nil && path.Base(*port) == path.Base(link) {
				// switch to the session-indepenent name (only switches once)
				*port = alias
				break
			}
		}
	}
}

// SerialConnect opens and re-opens a serial port and feeds the receive channel.
func SerialConnect() {
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
			if n > 0 {
				serialRecv <- data[:n]
			}
		}
		fmt.Print("\n[disconnected] ")

		dev.Close()
		dev = nil
		tty = nil
		close(serialRecv)
	}
}

// SerialDispatch handles all incoming and outgoing serial data.
func SerialDispatch() {
	go func() {
		for data := range serialSend {
			// send spaces iso tabs, because mecrisp echoes tabs differently
			//data = bytes.Replace(data, []byte{'\t'}, []byte{' '}, -1)
			if dev == nil { // avoid write-while-closed panics
				fmt.Printf("[CAN'T WRITE! %s]\n", *port)
				return
			} else if _, err := dev.Write(data); err != nil {
				fmt.Printf("[WRITE ERROR! %s]\n", *port)
				dev.Close()
				return
			}
		}
	}()

	for {
		select {

		case data, ok := <-serialRecv:
			if *verbose {
				fmt.Printf("recv: %q %v\n", data, ok)
			}
			if !ok {
				return
			}
			os.Stdout.Write(data)

		case line := <-commandSend:
			if strings.HasPrefix(line, "!") {
				if SpecialCommand(line) {
					continue
				}
				line = line[1:]
			}
			data := []byte(line + "\r")
			if *verbose {
				fmt.Printf("send: %q\n", data)
			}
			serialSend <- data
		}
	}
}

// SpecialCommand recognises and handles certain commands in a different way.
func SpecialCommand(line string) bool {
	cmd := strings.SplitN(line, " ", 2)
	if len(cmd) > 0 {
		switch cmd[0] {

		case "!":
			fmt.Println("[enter '!h' for help]")

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
  !reset          reset the board, same as ctrl-c
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
