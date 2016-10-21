package main

//go:generate go-bindata data/

import (
	"flag"
	"fmt"
	"os"
)

var (
	verbose = flag.Bool("v", false, "more verbose output, for debugging")

	serialRecv  = make(chan []byte)
	serialSend  = make(chan []byte)
	commandSend = make(chan string)
	done        = make(chan error)
)

func main() {
	flag.Parse()

	if len(os.Args) == 1 {
		fmt.Println("Folie", VERSION, "(type ctrl-d or ctrl-c to quit)")
	}

	go ConsoleTask()
	go SerialConnect()
	go SerialDispatch()

	if err, ok := <-done; ok {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(e error) {
	if e != nil {
		done <- e
	}
}
