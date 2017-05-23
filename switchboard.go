package folie

// This file contains !commands.

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// bufferPool holds buffer to use in Read() calls.
var bufferPool = sync.Pool{New: func() interface{} { return make([]byte, 256) }}

func getBuffer() []byte  { return bufferPool.Get().([]byte) }
func putBuffer(b []byte) { bufferPool.Put(b) }

type Switchboard struct {
	MicroInput   <-chan []byte // receive from microcontroller
	MicroOutput  MicroConn     // send to microcontroller
	ConsoleInput <-chan []byte // receive from interactive console (has ! commands)
	NetworkInput <-chan []byte // receive from remote consoles (no ! commands)

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
	for i := range sw.consoleOutput {
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

func (sw *Switchboard) Run() {
	for {
		select {
		case buf := <-sw.MicroInput:
			if Verbose {
				fmt.Printf("recv: %q\n", buf)
			}
			sw.consoleWrite(buf)
			putBuffer(buf)

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

		case buf := <-sw.NetworkInput:
			if Verbose {
				fmt.Printf("send: %q\n", buf)
			}
			sw.MicroOutput.Write(buf)
			putBuffer(buf)
		}
	}
}
