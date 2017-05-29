package main

// Folie main program.

//go:generate go-bindata -prefix files/ files/

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tve/folie"
)

var VERSION = "3.dev" // overriden in Makefile

func main() {
	fmt.Fprintf(os.Stderr, "[JeeLabs Folie %s]\n", VERSION)

	// Deal with commandline flags
	var (
		verbose = flag.Bool("v", false, "verbose output for debugging")
		listen  = flag.String("l", "",
			"IP address and port to listen for SSH connections, e.g. 0.0.0.0:2022")
		serverKey = flag.String("key", "/etc/ssh/ssh_host_dsa_key",
			"SSH host key for folie to use")
		authorizedKeys = flag.String("auth", ".authorized_keys",
			"SSH authorized client keys, the value \"insecure\" can be used to disable auth, "+
				"which can be useful when listening on localhost")
		port = flag.String("p", "", "serial port (COM*, /dev/cu.*, /dev/tty*, or hostname:port)")
		baud = flag.Int("b", 115200, "serial baud rate")
		raw  = flag.Bool("r", false, "use raw instead of telnet protocol")
		ssh  = flag.String("ssh", "", "ssh address:port to connect to")
	)

	flag.Parse()

	folie.Verbose = *verbose

	// Set-up readline on the interactive terminal.
	rdl, err := folie.NewReadline()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error initializing readline: %s\n", err)
		osExit(1)
	}

	// Select serial port, remote serial port, or ssh server.
	var sshClient *folie.SSHClient
	if *ssh != "" {
		if *port != "" {
			fmt.Fprintln(os.Stderr, "-p and -ssh cannot be combined\n")
			osExit(1)
		}
		if *listen != "" {
			fmt.Fprintln(os.Stderr, "-listen and -ssh cannot be combined\n")
			osExit(1)
		}
		sshClient, err = folie.NewSSHClient(*ssh)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			osExit(1)
		}

	} else {
		if *port == "" {
			*port = folie.SelectPort(rdl)
		}
		if *port == "" {
			// No serial (or remote serial) port chosen, nothing to do.
			fmt.Fprintln(os.Stderr, "No port selected")
			osExit(0)
		}
	}
	//fmt.Fprintf(os.Stderr, "Selected port %s\n", *port)

	// Decide whether to listen on SSH locally to accept connections.
	var sshServer *folie.SSHServer
	if *listen != "" {
		var err error
		if sshServer, err = folie.NewSSHServer(*listen, *serverKey, *authorizedKeys); err != nil {
			fmt.Fprintf(os.Stderr, "SSH server %s", err)
			osExit(2)
		}
	}

	// Start the goroutines for the local interactive console.
	done := make(chan error)
	consoleInput := make(chan []byte, 1)
	folie.RunConsole(rdl, consoleInput, done)

	// Open the microcontroller serial port or telnet connection and start goroutines.
	var micro folie.MicroConn
	if sshClient != nil {
		micro = sshClient
	} else if _, err := os.Stat(*port); err == nil {
		if *raw {
			// Raw serial port controlled using DTR/RTS/...
			micro = &folie.SerialConn{Path: *port, Baud: *baud}
		} else {
			// Serial port with Serplu controlled via telnet escapes.
			micro = &folie.TelnetConn{Path: *port}
		}
	} else {
		// Remote serial port across the network.
		micro = &folie.TelnetConn{Addr: *port}
	}
	microInput := make(chan []byte, 1)
	if err := folie.MicroConnRunner(micro, microInput); err != nil {
		fmt.Fprintln(os.Stderr, err)
		osExit(3)
	}

	// Start the switchboard in the middle.
	networkInput := make(chan folie.NetInput, 1)
	sw := folie.Switchboard{MicroInput: microInput, MicroOutput: micro,
		ConsoleInput: consoleInput, NetworkInput: networkInput,
		AssetNames: AssetNames(), Asset: Asset}
	sw.AddConsoleOutput(os.Stdout)
	go sw.Run()

	if sshServer != nil {
		go sshServer.Run(networkInput, func(w io.Writer) { sw.AddConsoleOutput(w) })
	}

	fmt.Fprintln(os.Stderr, "[Ready!]")
	if err, ok := <-done; ok {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		rdl.Close()
		osExit(1)
	}
}

// osExit calls os.Exit after a small sleep to let stdout/stderr output drain. This is necessary
// because of the loop-back pipe for the InsertCR stuff... Ouch.
func osExit(code int) {
	time.Sleep(100 * time.Millisecond)
	os.Exit(code)
}
