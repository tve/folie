package main

//go:generate go-bindata data/

import (
	"flag"
	"log"
)

var (
	serialRecv  = make(chan []byte, 0)
	commandSend = make(chan string, 0)
)

func main() {
	flag.Parse()

	go SerialConnect()
	go SerialDispatch()

	ConsoleTask()
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
