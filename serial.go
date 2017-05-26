package folie

// This file contains the SerialConn, which interfaces with a serial port.

import (
	//"bytes"

	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
	"go.bug.st/serial.v1"
)

const (
	byIDPrefix = "/dev/serial/by-id/" // where to find serial devices by ID on linux
)

// SerialConn implements microConn for a microcontroller attached to a local serial port.
type SerialConn struct {
	Path string // pathname of device
	Baud int    // desired baud rate, if 0 defaults to 115200

	intPath string      // path after switching to by-id pathname
	tty     serial.Port // the actual serial port
	mu      sync.Mutex  // make writes atomic
}

var _ MicroConn = &SerialConn{} // ensure the interface is implemented

// Open connects to the serial device and initializes the baud rate as well as RTS & DTR.
func (sc *SerialConn) Open() error {
	if sc.intPath == "" {
		sc.intPath = switchDev(sc.Path)
	}
	if sc.Baud == 0 {
		sc.Baud = 115200
	}

	tty, err := serial.Open(sc.intPath, &serial.Mode{BaudRate: sc.Baud})
	if err != nil {
		return fmt.Errorf("%s: %s", sc.intPath, err)
	}
	tty.SetRTS(true)
	tty.SetDTR(false)
	sc.tty = tty
	return nil
}

// Close closes the connection.
func (sc *SerialConn) Close() error { return sc.tty.Close() }

// Read bytes from the connection.
func (sc *SerialConn) Read(buf []byte) (int, error) { return sc.tty.Read(buf) }

// Write atomically sends bytes across the connection.
func (sc *SerialConn) Write(buf []byte) (int, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	n, err := sc.tty.Write(buf)
	if err == io.EOF { // FIXME: what do we get when Writing to a closed connection?
		err = io.EOF
	}
	return n, err
}

// Reset resets the attached microcontroller. If enterBoot is true the microcontroller enters the
// built-in bootloader allowing flash memory to be reprogrammed.
// Return true if the reset can be issued, false if there is an error.
func (sc *SerialConn) Reset(enterBoot bool) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if err := sc.tty.SetDTR(true); err != nil {
		return false
	}
	sc.tty.SetRTS(!enterBoot)
	if enterBoot {
		sc.tty.SetMode(&serial.Mode{BaudRate: sc.Baud, Parity: serial.EvenParity})
	} else {
		sc.tty.SetMode(&serial.Mode{BaudRate: sc.Baud})
	}
	time.Sleep(time.Millisecond)
	sc.tty.SetDTR(false)
	time.Sleep(time.Millisecond)
	return true
}

// SelectPort enumerates available ports, prompts for a choice, and returns the chosen port name.
// It returns an empty string if nothing useful was chosen.
func SelectPort(console *readline.Instance) string {
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
	for {
		console.SetPrompt("? ")
		console.Refresh()
		reply, err := console.Readline()
		if err != nil {
			return ""
		}
		console.SetPrompt("")
		fmt.Fprintln(console.Stdout(), reply)

		if sel, _ := strconv.Atoi(reply); sel > 0 && sel <= len(ports) {
			return ports[sel-1]
		}
		fmt.Fprintln(console.Stdout(), "Enter number of desired port or ctrl-d to quit.")
	}
}

// switchDev switches a /dev/ttyXXX path to /dev/serial/by-id/YYY in order to allow reopening
// the device when it's reset or unplugged and replugged. It returns the mapped port name or the
// provided name if no mapping could be found.
func switchDev(devicePath string) string {
	if dir, err := os.Open(byIDPrefix); err == nil {
		names, _ := dir.Readdirnames(-1)
		// look for an entry matching the current serial device name
		for _, name := range names {
			alias := byIDPrefix + name
			link, e := os.Readlink(alias)
			if e == nil && path.Base(devicePath) == path.Base(link) {
				// return the session-independent name
				return alias
			}
		}
	}
	return devicePath
}
