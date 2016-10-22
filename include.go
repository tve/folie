package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"
)

// IncludeFile sends out one file, expanding embdded includes as needed.
func IncludeFile(name string) bool {
	lineNum := 0
	fmt.Printf("\\       >>> include %s\n", name)
	defer func() {
		fmt.Printf("\\       <<<<<<<<<<< %s (%d lines)\n", name, lineNum)
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
		lineNum++

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
			match(line + " ")
		}
	}

	return true
}

func match(needle string) {
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
				fmt.Printf("unexpected: %q\n", lines[i])
			}
			last := lines[n-2]
			if len(lines[n-1]) == 0 && bytes.HasPrefix(last, []byte(needle)) {
				if needle + " ok." != string(last) {
					fmt.Printf("extra: %q\n", last)
				}
				return
			}
			fmt.Printf("unexpected end: %q\n", last)
			pending = lines[n-1]

		case <-timeout:
			fmt.Println("timed out")
			return
		}
	}
}
