package main

//go:generate go-bindata data/

import (
	"bytes"
	"flag"
	"log"
	"os"
	"sync"

	"github.com/chzyer/readline"
	"github.com/tarm/serial"
)

var (
	port = flag.String("p", "", "serial port (usually /dev/tty* or COM*)")
	baud = flag.Int("b", 115200, "serial baud rate")

	tasks sync.WaitGroup
	tty   *serial.Port
)

func main() {
	var err error
	flag.Parse()

	config := serial.Config{Name: *port, Baud: *baud}
	tty, err = serial.OpenPort(&config)
	check(err)

	go func() {
		for {
			line := make([]byte, 100)
			n, _ := tty.Read(line)
			line = bytes.Replace(line[:n], []byte("\n"), []byte("\r\n"), -1)
			os.Stdout.Write(line)
		}
	}()

	tasks.Add(1)
	go consoleTask()

	tasks.Wait()
}

func consoleTask() {
	defer tasks.Done()

	rl, err := readline.NewEx(&readline.Config{ UniqueEditLine: true })
	check(err)
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		tty.Write([]byte(line + "\r"))
	}
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
