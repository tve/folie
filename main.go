package main

//go:generate go-bindata data/

import (
	"flag"
	"log"
)

var (
	serialIn   = make(chan []byte, 0)
	commandOut = make(chan string, 0)
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
