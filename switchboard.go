package folie

// This file contains !commands.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
)

// bufferPool holds buffers to use in Read() calls.
var bufferPool = sync.Pool{New: func() interface{} { return make([]byte, 256) }}

func getBuffer() []byte  { return bufferPool.Get().([]byte) }
func putBuffer(b []byte) { bufferPool.Put(b) }

// NetInput can hold a byte buffer or a command.
type NetInput struct {
	What int    // one of RawIn, PacketIn, CommandIn, FlashIn
	Buf  []byte // data
}

const (
	RawIn     = iota // raw bytes input
	ResetIn          // reset uC (has no data)
	PacketIn         // data packet (data is packet)
	CommandIn        // ! command (data is commandline)
	FlashIn          // flash upload (data is flash binary/hex)
)

// Switchboard represents the central point where all input and output methods come together. This
// where data is forwarded from one to another and where the decision is made whether to interpret
// commands or pass data through uninterpreted.
type Switchboard struct {
	MicroInput   <-chan []byte   // receive from microcontroller
	MicroOutput  MicroConn       // send to microcontroller
	ConsoleInput <-chan []byte   // receive from interactive console (has ! commands)
	NetworkInput <-chan NetInput // receive from remote consoles (no ! commands)

	AssetNames []string                     // list of built-in firmwares
	Asset      func(string) ([]byte, error) // callback the get asset

	mu            sync.Mutex  // protect fields below
	consoleOutput []io.Writer // broadcast to multiple consoles

}

func (sw *Switchboard) AddConsoleOutput(wr io.Writer) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.consoleOutput = append(sw.consoleOutput, wr)
}

func (sw *Switchboard) RemoveConsoleOutput(wr io.Writer) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.remove(wr)
}

func (sw *Switchboard) remove(wr io.Writer) {
	for i := 0; i < len(sw.consoleOutput); i++ {
		if sw.consoleOutput[i] == wr {
			sw.consoleOutput = append(sw.consoleOutput[:i], sw.consoleOutput[i+1:]...)
		}
	}
}

func (sw *Switchboard) consoleWrite(buf []byte) {
	sw.mu.Lock()
	for _, wr := range sw.consoleOutput {
		if _, err := wr.Write(buf); err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "\n[Error writing: %s]\n", err)
			}
			sw.remove(wr)
		}
	}
	sw.mu.Unlock()
}

// Run operates teh switchboard and specifically processes input and writes it as approprite to
// outputs. Run is an infinite for loop with a select, so once input arrives that captures the
// thread of control until the corresponding output is done. In some cases, such as when calling
// specialCommand, further input may be awaited and processed, so a command can block the entire
// switchboard until it's done. This is helpful when running commands (like flashing the uC) that
// should not be interrupted by other stuff.
func (sw *Switchboard) Run() {
	for {
		select {
		// Input from the microcontroller.
		case buf := <-sw.MicroInput:
			if Verbose {
				fmt.Printf("recv: %q\n", buf)
			}
			sw.consoleWrite(buf)
			putBuffer(buf)

		// Input from the interactive console, interpret ! commands.
		case buf := <-sw.ConsoleInput:
			if buf[0] == '!' {
				// Convert buf to string.
				var line string
				if buf[len(buf)-1] == '\n' {
					line = string(buf[:len(buf)-1])
				} else {
					line = string(buf)
				}
				// See if it's a special command.
				if sw.specialCommand(line) {
					continue
				}
				// Else, treat as normal.
			}
			if Verbose {
				fmt.Printf("send: %q\n", buf)
			}
			sw.MicroOutput.Write(buf)
			putBuffer(buf)

		// Input from the network, it has several possible commands "baked-in"
		case inp := <-sw.NetworkInput:
			switch inp.What {
			case RawIn:
				if Verbose {
					fmt.Printf("send: %q\n", inp.Buf)
				}
				sw.MicroOutput.Write(inp.Buf)
			case ResetIn:
				sw.MicroOutput.Reset(false)
			case FlashIn:
				up := Uploader{Tx: sw.MicroOutput, Rx: sw.MicroInput}
				up.Upload(inp.Buf)
				sw.MicroOutput.Reset(false)
			case CommandIn:
				// We need to feed line-by-line to the output 'cause we need
				// to read input, match it, and thereby rate-limit.
				for _, line := range bytes.Split(inp.Buf, []byte{'\n'}) {
					sw.MicroOutput.Write(line)
					sw.MicroOutput.Write([]byte{'\n'})
					if !match(string(line), sw.MicroInput) {
						break
					}
				}
			}
			if inp.Buf != nil {
				putBuffer(inp.Buf)
			}
		}
	}
}
