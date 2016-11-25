package main

import (
	"bytes"
	"os"

	"github.com/chzyer/readline"
)

var (
	console *readline.Instance
)

// ConsoleTask listens to the console with readline for editing & history.
func ConsoleTask() {
	if readline.IsTerminal(1) {
		os.Stdout = InsertCRs(os.Stdout)
	}
	if readline.IsTerminal(2) {
		os.Stderr = InsertCRs(os.Stderr)
	}

	var err error
	config := readline.Config{
		UniqueEditLine: true,
		Stdout:         os.Stdout,
	}
	console, err = readline.NewEx(&config)
	check(err)

	go SerialConnect()

	for {
		line, err := console.Readline()
		if err == readline.ErrInterrupt {
			line = "!reset"
		} else if err != nil {
			close(done)
			break
		}
		commandSend <- line
	}
}

// InsertCRs is used to insert lost CRs when readline is active
func InsertCRs(out *os.File) *os.File {
	readFile, writeFile, err := os.Pipe()
	check(err)

	go func() {
		defer readFile.Close()
		var data [250]byte
		for {
			n, err := readFile.Read(data[:])
			if err != nil {
				break
			}
			out.Write(bytes.Replace(data[:n], []byte("\n"), []byte("\r\n"), -1))
		}
	}()

	return writeFile
}
