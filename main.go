package main

//go:generate go-bindata data/

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	verbose = flag.Bool("v", false, "more verbose output, for debugging")

	serialRecv  = make(chan []byte, 0)
	serialSend  = make(chan []byte, 0)
	commandSend = make(chan string, 0)
)

func main() {
	flag.Parse()

	if len(os.Args) == 1 {
		fmt.Println("Folie", VERSION, "(type ctrl-d or ctrl-c to quit)")
	}

	go SerialConnect()
	go SerialDispatch()

	ConsoleTask()
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
		//panic(e)
	}
}
