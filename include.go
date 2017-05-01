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

	currDir := path.Dir(name)
	currFile := path.Base(name)
	currLine := 0
	if level == 0 {
		callCount = 0
	}
	callCount++
	prefix := fmt.Sprintf("%d>", callCount)

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
			for _, fname := range strings.Fields(line)[1:] {
				statusMsg(lastMsg, "")
				if !IncludeFile(path.Join(currDir, fname), level+1) {
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
	timer := time.NewTimer(3 * time.Second)

	var pending []byte
	for {
		select {

		case data := <-serialRecv:
			pending = append(pending, data...)
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(time.Second)

		case <-commandSend:
			return false // abort include

		case <-time.After(10 * time.Millisecond):
			if !bytes.Contains(pending, []byte{'\n'}) {
				continue
			}

			lines := bytes.Split(pending, []byte{'\n'})
			n := len(lines)
			for i := 0; i < n-2; i++ {
				fmt.Printf("%s\n", lines[i])
			}
			lines = lines[n-2:]

			last := string(lines[0])
			if len(lines[1]) == 0 {
				hasExpected := strings.HasPrefix(last, expect+" ")
				if hasExpected || strings.HasSuffix(last, " ok.") {
					if last != expect+"  ok." {
						msg := last
						// only show output if source does not start with "("
						// ... in that case, show just the comment up to ")"
						if hasExpected {
							msg = last[len(expect)+1:]
							if last[0] == '(' {
								if n := strings.Index(expect, ")"); n > 0 {
									msg = last[:n+1] + last[len(expect):]
								}
							}
						}
						if msg == "" {
							return true // don't show empty [if]-skipped lines
						}
						fmt.Printf("%s\n", msg)
						if hasFatalError(last) {
							return false // no point in keeping going
						}
					}
					return true
				}
			}
			fmt.Printf("%s\n", last)
			pending = lines[1]

		case <-timer.C:
			if len(pending) == 0 {
				return true
			}
			fmt.Printf("%s (timeout)\n", pending)
			return string(pending) == expect+" "
		}
	}
}

func hasFatalError(s string) bool {
	for _, match := range []string{
		" not found.",
		" is compile-only.",
		" Stack not balanced.",
		" Stack underflow",
		" Stack overflow",
		" Flash full",
		" Ram full",
		" Structures don't match",
		" Jump too far",
	} {
		if strings.HasSuffix(s, match) {
			return true
		}
	}
	return false
}
