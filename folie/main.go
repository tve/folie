package main

// Folie main program.

//go:generate go-bindata -prefix files/ files/

import (
	"flag"
	"fmt"
	"github.com/tve/folie"
	"os"
)

/*
var (
	serialRecv  = make(chan []byte)
	serialSend  = make(chan []byte)
	commandSend = make(chan string)
	done        = make(chan error)
)
*/

var VERSION = "3.dev" // overriden in Makefile

func main() {
	fmt.Fprintf(os.Stderr, "[JeeLabs Folie %s]\n", VERSION)

	// Deal with commandline flags
	var (
		verbose = flag.Bool("v", false, "verbose output for debugging")
		listen  = flag.String("l", "0.0.0.0:2022",
			"IP address and port to listen for SSH connections, e.g. 0.0.0.0:2022")
		serverKey      = flag.String("key", "/etc/ssh/ssh_host_dsa_key", "SSH host key for folie to use")
		authorizedKeys = flag.String("auth", ".authorized_keys",
			"SSH authorized client keys, the value \"insecure\" can be used to disable auth, "+
				"which can be useful when listening on localhost")
		port = flag.String("p", "", "serial port (COM*, /dev/cu.*, or /dev/tty*)")
		baud = flag.Int("b", 115200, "serial baud rate")
		//raw = flag.Bool("r", false, "use raw instead of telnet protocol")
	)

	flag.Parse()

	folie.Verbose = *verbose

	// Set-up readline on the interactive terminal.
	rdl, err := folie.NewReadline()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error initializing readline: %s\n", err)
		os.Exit(1)
	}

	// Select serial port or remote serial port.
	if *port == "" {
		*port = folie.SelectPort(rdl)
	}
	if *port == "" {
		// No serial (or remote serial) port chosen, nothing to do.
		fmt.Fprintln(os.Stderr, "No port selected")
		os.Exit(0)
	}
	//fmt.Fprintf(os.Stderr, "Selected port %s\n", *port)

	// Decide whether to listen on SSH locally to accept connections.
	//var sshServer *folie.SSHServer
	if *listen != "" {
		var err error
		if _, err = folie.NewSSHServer(*listen, *serverKey, *authorizedKeys); err != nil {
			fmt.Fprintf(os.Stderr, "SSH server %s", err)
			os.Exit(1)
		}
	}
	fmt.Fprintln(os.Stderr, "SSH ready")

	// Start the goroutines for the local interactive console.
	done := make(chan error)
	consoleTx := make(chan []byte, 1)
	consoleRx := make(chan []byte, 1)
	folie.RunConsole(rdl, consoleTx, consoleRx, done)

	// Open the microcontroller serial port or telnet connection and start goroutines.
	//microTx := make(chan []byte, 1)
	//microRx := make(chan []byte, 1)
	var micro folie.MicroConn
	if _, err := os.Stat(*port); err == nil {
		micro = &folie.SerialConn{Path: *port, Baud: *baud}
	} else {
		micro = &folie.TelnetConn{Addr: *port}
	}
	if err := folie.MicroConnRunner(micro, consoleRx, consoleTx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "[Ready!]")

	//if sshServer != nil {
	//go sshServer.Run(serialRecv, serialSend, commandSend, done)
	//}

	if err, ok := <-done; ok {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		rdl.Close()
		os.Exit(1)
	}
}
