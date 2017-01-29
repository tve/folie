package main

//go:generate go-bindata -prefix files/ files/

import (
	"flag"
	"fmt"
	"os"
)

var (
	verbose = flag.Bool("v", false, "verbose output for debugging")

	serialRecv  = make(chan []byte)
	serialSend  = make(chan []byte)
	commandSend = make(chan string)
	done        = make(chan error)
)

func main() {
	flag.Parse()

	if len(os.Args) == 1 {
		fmt.Println("Folie", VERSION)
	}

	ConsoleSetup()

	if *port == "" {
		*port = selectPort()
	}

	if *port != "" {
		go ConsoleTask()
		go SerialConnect()

		if err, ok := <-done; ok {
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			console.Close()
			os.Exit(1)
		}
	}
}

func check(e error) {
	if e != nil {
		done <- e
	}
}
