package main

import (
	"bytes"
	"os"

	"github.com/chzyer/readline"
	"github.com/tarm/serial"
)

func main() {
	devname := "/dev/cu.usbmodemD5D4C5E3"
	config := serial.Config{Name: devname, Baud: 115200}
	tty, _ := serial.OpenPort(&config)

	rl, _ := readline.NewEx(&readline.Config{
		UniqueEditLine: true,
	})
	defer rl.Close()

	go func() {
		for {
			line := make([]byte, 100)
			n, _ := tty.Read(line)
			line = bytes.Replace(line[:n], []byte("\n"), []byte("\r\n"), -1)
			os.Stdout.Write(line)
		}
	}()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		tty.Write([]byte(line + "\r"))
	}
}
