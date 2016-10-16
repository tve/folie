package main

import (
	"os"
	"bytes"

	"github.com/chzyer/readline"
	"github.com/pkg/term"
)

func main() {
	tty, _ := term.Open("/dev/cu.usbmodemD5D4C5E3",
		term.Speed(115200), term.CBreakMode)

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
