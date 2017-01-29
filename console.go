package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"

	"github.com/chzyer/readline"
)

var (
	console *readline.Instance
)

func ConsoleSetup() {
	if readline.IsTerminal(1) {
		os.Stdout = InsertCRs(os.Stdout)
	}
	if readline.IsTerminal(2) {
		os.Stderr = InsertCRs(os.Stderr)
	}

	var err error
	config := readline.Config{
		UniqueEditLine:    true,
		HistorySearchFold: true,
		AutoComplete:      FileCompleter{},
	}
	console, err = readline.NewEx(&config)
	check(err)
}

// ConsoleTask listens to the console with readline for editing & history.
func ConsoleTask() {
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

// Readline will pass the whole line and current offset to it
// Completer need to pass all the candidates, and how long they shared the same characters in line
// Example:
//   [go, git, git-shell, grep]
//   Do("g", 1) => ["o", "it", "it-shell", "rep"], 1
//   Do("gi", 2) => ["t", "t-shell"], 2
//   Do("git", 3) => ["", "-shell"], 3

type FileCompleter struct{}

func (f FileCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	typedSoFar := string(line[:pos])
	spacePos := strings.IndexByte(typedSoFar, ' ')

	if strings.HasPrefix(typedSoFar, "!") && spacePos > 1 {
		slashPos := strings.LastIndexByte(typedSoFar, '/')
		dir := "."
		if slashPos < 0 {
			slashPos = spacePos
		}
		if slashPos > spacePos {
			dir = typedSoFar[spacePos+1 : slashPos+1]
		}
		prefix := typedSoFar[slashPos+1:]

		files, _ := ioutil.ReadDir(dir)
		for _, f := range files {
			name := f.Name()
			if strings.HasPrefix(name, prefix) {
				suffix := name[len(prefix):]
				if f.IsDir() {
					suffix += "/"
				}
				newLine = append(newLine, []rune(suffix))
			}
		}
	}

	length = pos
	return
}
