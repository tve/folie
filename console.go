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
		os.Stdout = insertCRs(os.Stdout)
	}
	if readline.IsTerminal(2) {
		os.Stderr = insertCRs(os.Stderr)
	}

	var err error
	config := readline.Config{
		UniqueEditLine: true,
		Stdout:         os.Stdout,
	}
	console, err = readline.NewEx(&config)
	check(err)
	defer console.Close()

	for {
		line, err := console.Readline()
		if err != nil {
			break
		}
		commandSend <- line
	}
}

// insertCRs is used to insert lost CRs when readline is active
func insertCRs(out *os.File) *os.File {
	readFile, writeFile, err := os.Pipe()
	check(err)

	go func() {
		defer readFile.Close()
		for {
			data := make([]byte, 250)
			n, err := readFile.Read(data)
			if err != nil {
				break
			}
			data = bytes.Replace(data[:n], []byte("\n"), []byte("\r\n"), -1)
			out.Write(data)
		}
	}()

	return writeFile
}
