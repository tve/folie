package folie

// This file contains the TelnetConn, which connects to a remote telnet server to access a
// microcontroller via a remote serial port.

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	Iac  = 255
	Will = 251
	Sb   = 250
	Se   = 240

	ComPortOpt = 44
	SetParity  = 3
	SetControl = 5

	PAR_NONE = 1
	PAR_EVEN = 3

	FLOW_OFF = 1
	DTR_ON   = 8
	DTR_OFF  = 9
	RTS_ON   = 11
	RTS_OFF  = 12
)

// TelnetConn implements microConn for a microcontroller attached via a telnet connection to a
// server that understands telnet escapes for adjusting the baudrate and toggling DTS/RTS. Examples
// are ser2net and esp-link.
type TelnetConn struct {
	Addr string // telnet server address, passed to net.Dial

	conn    io.ReadWriteCloser // connection to remote telnet server
	tnState int                // tracks telnet protocol state when reading
	mu      sync.Mutex         // used to make Write atomic
}

var _ MicroConn = &TelnetConn{} // ensure the interface is implemented

// Open connects to the telnet server and sends the initial escape sequence to configure the
// remote serial device.
func (tc *TelnetConn) Open() error {
	conn, err := net.Dial("tcp", tc.Addr)
	if err != nil {
		return fmt.Errorf("%s: %s", tc.Addr, err)
	}

	if _, err := conn.Write([]byte{Iac, Will, ComPortOpt}); err != nil {
		conn.Close()
		return err
	}
	conn.Write(telnetEscape(SetParity, PAR_NONE))
	conn.Write(telnetEscape(SetControl, FLOW_OFF))
	conn.Write(telnetEscape(SetControl, RTS_ON))
	conn.Write(telnetEscape(SetControl, DTR_OFF))

	tc.conn = conn
	tc.tnState = 0
	return nil
}

// Close the connection.
func (tc *TelnetConn) Close() error { return tc.conn.Close() }

// Read bytes from the connection.
func (tc *TelnetConn) Read(buf []byte) (int, error) {
	n, err := tc.conn.Read(buf)
	if err == nil && n > 0 {
		n = tc.clean(buf[:n])
	}
	return n, err
}

// write atomically sends bytes across the connection.
// It does NOT escape the Telnet escape character.
func (tc *TelnetConn) write(buf []byte) (int, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	n, err := tc.conn.Write(buf)
	if err == io.EOF { // FIXME: what do we get when Writing to a closed connection?
		err = io.EOF
	}
	return n, err
}

// Write atomically sends bytes across the connection.
// It escapes the Telnet escape character.
func (tc *TelnetConn) Write(buf []byte) (int, error) {
	buf = bytes.Replace(buf, []byte{Iac}, []byte{Iac, Iac}, -1)
	return tc.write(buf)
}

// Reset the remote microcontroller.
// Return true if the reset can be issued, false if there is an error.
func (tc *TelnetConn) Reset(enterBoot bool) bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if _, err := tc.conn.Write(telnetEscape(SetControl, DTR_ON)); err != nil {
		return false
	}
	if enterBoot {
		tc.conn.Write(telnetEscape(SetControl, RTS_OFF))
		tc.conn.Write(telnetEscape(SetParity, PAR_EVEN))
	} else {
		tc.conn.Write(telnetEscape(SetControl, RTS_ON))
		tc.conn.Write(telnetEscape(SetParity, PAR_NONE))
	}
	time.Sleep(100 * time.Millisecond)
	tc.conn.Write(telnetEscape(SetControl, DTR_OFF))
	return true
}

//

// telnetEscape returns an encoded/escaped command sequence.
func telnetEscape(typ, val uint8) []byte {
	if Verbose {
		fmt.Printf("{esc:%02X:%02X}", typ, val)
	}
	return []byte{Iac, Sb, ComPortOpt, typ, val, Iac, Se}
}

// clean removes incoming telnet escape commands from the input buffer. It returns the
// length of the modified buffer. It modified tc.tnState as a side-effect.
func (tc *TelnetConn) clean(buf []byte) int {
	j := 0
	for _, b := range buf {
		buf[j] = b
		switch tc.tnState {
		case 0: // normal, copying
			if b == Iac {
				tc.tnState = 1
			} else {
				j++
			}
		case 1: // seen Iac
			if b == Sb {
				tc.tnState = 2
			} else if b >= Will {
				tc.tnState = 3
			} else {
				j++
				tc.tnState = 0
			}
		case 2: // inside command
			if b == Se {
				tc.tnState = 0
			}
		case 3: // Will/Wont/Do/Dont seen
			tc.tnState = 0
		}
	}
	return j
}
