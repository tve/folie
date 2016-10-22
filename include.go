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
	currFile string
	currLine int
)

// SendFile sends out one file, expanding embdded includes as needed.
func SendFile(name string) bool {
	prevFile := currFile
	prevLine := currLine
	currFile = path.Base(name)
	currLine = 0
	fmt.Printf("\\       >>> sending %s\n", currFile)
	defer func() {
		fmt.Printf("\\       <<<<<<<<<<< %s (%d lines)\n", currFile, currLine)
		currFile = prevFile
		currLine = prevLine
	}()

	f, err := os.Open(name)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		currLine++

		s := strings.TrimLeft(line, " ")
		if s == "" || s == "\\" || strings.HasPrefix(s, "\\ ") {
			continue // don't send empty or comment-only lines
		}

		if strings.HasPrefix(line, "include ") {
			for _, fname := range strings.Split(line[8:], " ") {
				if !SendFile(fname) {
					return false
				}
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
