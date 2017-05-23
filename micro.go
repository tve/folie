package folie

// This file implements the high-level control and I/O to an attached microcontroller.

import (
	"fmt"
	"io"
	"os"
	"time"
)

// MicroConn represents a connection to a microcontroller which is a bidirectional line-oriented
// connection plus a number of out-of-band commands to perform actions such as resetting the
// microcontroller, and flashing a raw binary.
type MicroConn interface {
	// Read must return an error if there is a problem with the connection.
	// Write must be atomic and it must return an io.EOF error if the conn is closed.
	io.ReadWriteCloser

	Open() error     // (re-)open the connection
	Reset(bool) bool // reset the microcontroller, the param says whether to enter the bootloader
	Flash([]byte)    // flash the microcontroller with the provided binary image
}

// MicroConnRunner takes a MicroConn and an rx channel. It operates a goroutine that reads
// from the MicroConn into the channel allowing higher levels to select on that channel. It also
// catches errors and reopens the MicroConn transparently.
//
// MicroConnRunner does not take part in the sending of data, instead, the MicroConn's Write method
// should be called directly. It is expected that catching errors on Read is sufficient and "heals"
// errors on the sending side quickly enough.
//
// The initial opening of the connection is done synchronously so an error can be returned
// immediately, afterwards goroutines are spawned to continue the connection(s).
func MicroConnRunner(mc MicroConn, rx chan<- []byte) error {
	if err := mc.Open(); err != nil {
		return err
	}

	// Goroutine that loops over lines and reopens the connection on error.
	go func() {
		for {
			// Read some bytes and forward to channel unless there's a problem.
			buf := getBuffer()
			n, err := mc.Read(buf)
			if n > 0 {
				// Process the bytes we got, we'll get any error again...
				// See docs for io.Reader.
				rx <- buf[:n]
				continue
			}
			if err == nil {
				continue // "nothing happened" according to io.Reader
			}

			fmt.Fprintf(os.Stderr, "\n[disconnected: %s]\nreconnecting...", err)
			mc.Close()

			// Open a fresh connection
			prevErr := err
			for {
				time.Sleep(500 * time.Millisecond)
				if err = mc.Open(); err == nil {
					fmt.Fprintln(os.Stderr, "\n[reconnected]")
					break
				}
				if err != prevErr {
					fmt.Fprintf(os.Stderr, "\nerror: %s\nreconnecting...", err)
				} else {
					fmt.Fprintln(os.Stderr, ".")
				}
			}
		}
	}()

	return nil
}
