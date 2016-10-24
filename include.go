package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
)

var (
	currFile  string
	currLine  int
	currCall int
)

// Status prints a formatted string and also returns it. It takes the previous
// string to be able to backspace over it before outputting the new message.
func Status(prev string, desc string, args ...interface{}) string {
	msg := fmt.Sprintf(desc, args...)
	// clear previous message
	for i := 0; i < len(prev); i++ {
		fmt.Print("\b \b")
	}
	fmt.Print(msg)
	return msg
}

// IncludeFile sends out one file, expanding embdded includes as needed.
func IncludeFile(name string, level int) bool {
	f, err := os.Open(name)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer f.Close()

	prevFile := currFile
	prevLine := currLine
	currFile = path.Base(name)
	currLine = 0
	if level == 0 {
		currCall = 0
	}
	currCall++

	lastMsg := ""
	defer func() {
		Status(lastMsg, "")
		currFile = prevFile
		currLine = prevLine
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		currLine++
		lastMsg = Status(lastMsg, "%s %s %d: ",
			strings.Repeat(">", currCall), currFile, currLine)

		line := scanner.Text()
		s := strings.TrimLeft(line, " ")
		if s == "" || s == "\\" || strings.HasPrefix(s, "\\ ") {
			continue // don't send empty or comment-only lines
		}

		if strings.HasPrefix(line, "include ") {
			for _, fname := range strings.Split(line[8:], " ") {
				Status(lastMsg, "")
				if !IncludeFile(fname, level+1) {
					return false
				}
				//Status(">", "")
			}
		} else {
			serialSend <- []byte(line + "\r")
			if !match(line) {
				return false
			}
		}
	}

	return true
}

func match(expect string) bool {
	timeout := time.After(3 * time.Second)

	var pending []byte
	for {
		select {

		case data := <-serialRecv:
			pending = append(pending, data...)

		case <-time.After(10 * time.Millisecond):
			if !bytes.ContainsRune(pending, '\n') {
				continue
			}

			lines := bytes.Split(pending, []byte{'\n'})
			n := len(lines)
			for i := 0; i < n-2; i++ {
				fmt.Printf("%s, line %d: %s\n", currFile, currLine, lines[i])
			}

			last := string(lines[n-2])
			if len(lines[n-1]) == 0 && strings.HasPrefix(last, expect+" ") {
				if last != expect+"  ok." {
					tail := last[len(expect)+1:]
					fmt.Printf("%s, line %d: %s\n", currFile, currLine, tail)
					if strings.HasSuffix(last, " not found.") ||
						strings.HasSuffix(last, " Stack underflow") {
						return false // no point in keeping going
					}
				}
				return true
			}
			fmt.Printf("%s, line %d: %s\n", currFile, currLine, last)
			pending = lines[n-1]

		case <-timeout:
			if len(pending) == 0 {
				return true
			}
			fmt.Printf("%s, line %d: %s (timeout)\n",
				currFile, currLine, pending)
			return string(pending) == expect+" "
		}
	}
}
