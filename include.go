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

// IncludeFile sends out one file, expanding embdded includes as needed.
func IncludeFile(name string) bool {
	prevFile := currFile
	prevLine := currLine
	currFile = path.Base(name)
	currLine = 0
	fmt.Printf("\\       >>> include %s\n", currFile)
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
				if !IncludeFile(fname) {
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
				if expect+"  ok." != last {
					fmt.Printf("%s, line %d: ", currFile, currLine)
					if strings.HasSuffix(last, " not found.") {
						fmt.Println(last[len(expect)+1:])
						return false
					}
					if strings.HasPrefix(last, expect+" Redefine ") &&
						strings.HasSuffix(last, "  ok.") {
						fmt.Println(last[len(expect)+1:])
					} else {
						fmt.Println(last)
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
			fmt.Printf("%s, line %d: %s\n", currFile, currLine, pending)
			return string(pending) == expect+" "
		}
	}
}
