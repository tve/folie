package main

//go:generate go-bindata -prefix files/ files/

import (
	"flag"
	"fmt"
	"os"
)

var (
	verbose     = flag.Bool("v", false, "verbose output for debugging")
	serialRecv  = make(chan []byte)
	serialSend  = make(chan []byte)
	commandSend = make(chan string)
	done        = make(chan error)
)

func main() {
	listen := flag.String("l", "0.0.0.0:2022",
		"IP address and port to listen for SSH connections, e.g. 0.0.0.0:2022")
	serverKey := flag.String("key", "/etc/ssh/ssh_host_dsa_key", "SSH host key for folie to use")
	authorizedKeys := flag.String("auth", ".authorized_keys",
		"SSH authorized client keys, the value \"insecure\" can be used to disable auth, "+
			"which can be useful when listening on localhost")

	flag.Parse()

	if len(os.Args) == 1 {
		fmt.Println("Folie", VERSION)
	}

	var sshServer *SSHServer
	if *listen != "" {
		var err error
		if sshServer, err = NewSSHServer(*listen, *serverKey, *authorizedKeys); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	ConsoleSetup()

	if *port == "" {
		*port = selectPort()
	}

	if *port != "" {
		go ConsoleTask()
		go SerialConnect()
		if sshServer != nil {
			go sshServer.Run(serialSend, serialRecv, commandSend, done)
		}

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
