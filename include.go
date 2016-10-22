package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func includeFile(name string) bool {
	//incLevel <- +1
	//defer func() { incLevel <- -1 }()

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

	throttleDone := make(chan struct{})
	defer close(throttleDone)
	go throttledSend(throttleDone)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		s := strings.TrimLeft(line, " ")
		if s == "" || s == "\\" || strings.HasPrefix(s, "\\ ") {
			continue // don't send empty or comment-only lines
		}

		if !parseAndSend(line) {
			return false
		}
	}

	return true
}

func throttledSend(quitter chan struct{}) {
	for {
		select {
		case <-quitter:
			return
		case data := <-serialRecv:
			fmt.Printf("got: %q\n", data)
		case line := <-commandSend:
			serialSend <- []byte(line + "\r")
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func parseAndSend(line string) bool {
	if strings.HasPrefix(line, "include ") {
		for _, fname := range strings.Split(line[8:], " ") {
			if !includeFile(fname) {
				return false
			}
		}
	} else {
		commandSend <- line
	}
	return true
}
