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

var callCount int

// IncludeFile sends out one file, expanding embdded includes as needed.
func IncludeFile(name string, level int) bool {
	f, err := os.Open(name)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer f.Close()

	currFile := path.Base(name)
	currLine := 0
	if level == 0 {
		callCount = 0
	}
	callCount++
	prefix := strings.Repeat(">", callCount)

	lastMsg := ""
	defer func() {
		statusMsg(lastMsg, "")
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		currLine++
		lastMsg = statusMsg(lastMsg, "%s %s %d: ", prefix, currFile, currLine)

		line := scanner.Text()
		s := strings.TrimLeft(line, " ")
		if s == "" || s == "\\" || strings.HasPrefix(s, "\\ ") {
			continue // don't send empty or comment-only lines
		}

		if strings.HasPrefix(line, "include ") {
			for _, fname := range strings.Split(line[8:], " ") {
				statusMsg(lastMsg, "")
				if !IncludeFile(fname, level+1) {
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

// statusMsg prints a formatted string and returns it. It takes the previous
// string to be able to clear it before outputting the new message.
func statusMsg(prev string, desc string, args ...interface{}) string {
	msg := fmt.Sprintf(desc, args...)
	n := len(msg)
	// FIXME this optimisation is incorrect, it sometimes eats up first 3 chars
	if false && n > 3 && n == len(prev) && msg[:n-3] == prev[:n-3] {
		fmt.Print("\b\b\b", msg[n-3:]) // optimise if only end changes
	} else {
		if len(msg) < len(prev) {
			fmt.Print("\r", strings.Repeat(" ", len(prev)))
		}
		fmt.Print("\r", msg)
	}
	return msg
}

func match(expect string) bool {
	timeout := time.After(3 * time.Second)

	var pending []byte
	for {
		select {

		case data := <-serialRecv:
			pending = append(pending, data...)

		case <-time.After(10 * time.Millisecond):
			if !bytes.Contains(pending, []byte{'\n'}) {
				continue
			}

			lines := bytes.Split(pending, []byte{'\n'})
			n := len(lines)
			for i := 0; i < n-2; i++ {
				fmt.Printf("%s\n", lines[i])
			}

			last := string(lines[n-2])
			if len(lines[n-1]) == 0 && strings.HasPrefix(last, expect+" ") {
				if last != expect+"  ok." {
					tail := last[len(expect)+1:]
					fmt.Printf("%s\n", tail)
					if strings.HasSuffix(last, " not found.") ||
						strings.HasSuffix(last, " Stack underflow") {
						return false // no point in keeping going
					}
				}
				return true
			}
			fmt.Printf("%s\n", last)
			pending = lines[n-1]

		case <-timeout:
			if len(pending) == 0 {
				return true
			}
			fmt.Printf("%s (timeout)\n",
				pending)
			return string(pending) == expect+" "
		}
	}
}
