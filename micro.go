package folie

// This file implements the high-level control and I/O to an attached microcontroller.

import (
	"bufio"
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

	Open() error  // (re-)open the connection
	Reset(bool)   // reset the microcontroller, the boolean says whether to enter the bootloader
	Flash([]byte) // flash the microcontroller with the provided binary image
}

// MicroConnRunner takes a concrete struct that implements MicroConn and a pair of tx/rx channels
// and shuffles data between the channels and MicroConn. It also takes care of reopening the
// MicroConn underlying connection when an error occurs.
//
// The tx and rx channels are expected to contain LF-terminated lines, but nothing really enforces
// that. No CR or LF stripping or adding is performed.
//
// The initial opening of the connection is done synchronously so an error can be returned
// immediately, afterwards goroutines are spawned to continue the connection(s).
func MicroConnRunner(mc MicroConn, tx <-chan []byte, rx chan<- []byte) error {
	if err := mc.Open(); err != nil {
		return err
	}

	// Loop over connections in a goroutine.
	go func() {
		for {
			// Spawn sender, it will die silently if the connection fails.
			go microConnSender(mc, tx)

			// Run the receiver, it will return an error if the connection fails.
			var err error
			if err = microConnReceiver(mc, rx); err != nil {
				fmt.Fprintf(os.Stderr, "\n[disconnected: %s]\nreconnecting...", err)
			}

			// Close so the microConnSender exits.
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

// microConnSender shuffles lines from tx to the writer.
func microConnSender(w io.Writer, tx <-chan []byte) {
	for line := range tx {
		if _, err := w.Write(line); err != nil {
			if err != io.EOF {
				fmt.Fprintln(os.Stderr, err)
			}
			return
		}
	}
}

// microConnReceiver loops over incoming bytes, splits them up into LF-terminated lines and
// forwards them onto the rx channel.
func microConnReceiver(r io.ReadCloser, rx chan<- []byte) error {
	rd := bufio.NewReader(r)
	for {
		line, err := rd.ReadBytes('\n')
		if err != nil {
			return err
		}
		rx <- line
	}
}
