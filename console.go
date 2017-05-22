package folie

// This file defines the Console, which interfaces with an interactive terminal.

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"

	"github.com/chzyer/readline"
)

// NewReadline creates a readline instance on stdin/out.
func NewReadline() (*readline.Instance, error) {
	if readline.IsTerminal(1) {
		os.Stdout = insertCRs(os.Stdout)
	}
	if readline.IsTerminal(2) {
		os.Stderr = insertCRs(os.Stderr)
	}

	config := readline.Config{
		UniqueEditLine:    true, // erase input after submitting
		HistorySearchFold: true, // case insensitive history
		AutoComplete:      FileCompleter{},
	}
	rdl, err := readline.NewEx(&config)
	if err != nil {
		return nil, err
	}
	return rdl, nil
}

// RunConsole runs one goroutine to push lines read on stdin into the rx channel,
// and another to print lines coming from the tx channel onto stdout.
func RunConsole(rdl *readline.Instance, tx <-chan []byte, rx chan<- []byte, done chan error) {
	// Goroutine to listen to stdin and send lines into the rx channel.
	go func() {
		for {
			line, err := rdl.Readline()
			if err == readline.ErrInterrupt {
				line = "!reset"
			} else if err != nil {
				close(done)
				return
			}
			// Convert to []byte and add terminating CR.
			buf := make([]byte, len(line)+1)
			copy(buf, line)
			buf[len(line)] = '\r'
			rx <- buf
		}
	}()

	// Goroutine to print lines coming on tx.
	go func() {
		for line := range tx {
			os.Stdout.Write(line)
		}
	}()
}

// insertCRs is used to insert lost CRs when readline is active.
func insertCRs(out *os.File) *os.File {
	readFile, writeFile, _ := os.Pipe()

	go func() {
		defer readFile.Close()
		var data [250]byte
		for {
			n, err := readFile.Read(data[:])
			if err != nil {
				return
			}
			line := bytes.Replace(data[:n], []byte("\n"), []byte("\r\n"), -1)
			for len(line) > 0 {
				n, err := out.Write(line)
				if err != nil {
					return
				}
				line = line[n:]
			}
		}
	}()

	return writeFile
}

// FileCompleter performs filename completion for readline.
type FileCompleter struct{}

// Do returns a list of candidate completions given the whole line and the current offset into it.
// The length return value is how long they shared the same characters in line.
// Example:
//   [go, git, git-shell, grep]
//   Do("g", 1) => ["o", "it", "it-shell", "rep"], 1
//   Do("gi", 2) => ["t", "t-shell"], 2
//   Do("git", 3) => ["", "-shell"], 3
func (f FileCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	typedSoFar := string(line[:pos])
	spacePos := strings.IndexByte(typedSoFar, ' ')

	if strings.HasPrefix(typedSoFar, "!") && spacePos > 1 {
		slashPos := strings.LastIndexByte(typedSoFar, '/')
		if slashPos < 0 {
			slashPos = spacePos
		}
		prefix := typedSoFar[slashPos+1:]

		dir := "."
		if slashPos > spacePos {
			dir = typedSoFar[spacePos+1 : slashPos+1]
		}

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
