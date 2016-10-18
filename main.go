package main

//go:generate go-bindata data/

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"
	"sync"

	"github.com/chzyer/readline"
	"github.com/tarm/serial"
)

var (
	port = flag.String("p", "", "serial port (usually /dev/tty* or COM*)")
	baud = flag.Int("b", 115200, "serial baud rate")

	tasks sync.WaitGroup
	tty   *serial.Port
	serIn = make(chan []byte, 0)
)

func main() {
	flag.Parse()

	go readSerial()

	go func() {
		for line := range serIn {
			line = bytes.Replace(line, []byte("\n"), []byte("\r\n"), -1)
			os.Stdout.Write(line)
		}
	}()

	tasks.Add(1)
	go consoleTask()

	tasks.Wait()
}

func readSerial() {
	for {
		var err error
		config := serial.Config{Name: *port, Baud: *baud}
		tty, err = serial.OpenPort(&config)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		fmt.Print("[connected]\r\n")
		for {
			line := make([]byte, 250)
			n, err := tty.Read(line)
			if err == io.EOF {
				break
			}
			check(err)
			serIn <- line[:n]
		}
		fmt.Print("\r\n[disconnected] ")

		tty.Close()
	}
}

func consoleTask() {
	defer tasks.Done()

	config := readline.Config{UniqueEditLine: true}
	rl, err := readline.NewEx(&config)
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
