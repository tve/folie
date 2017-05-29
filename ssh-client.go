package folie

// This file contains the SSH server.

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHClient represents an instance of an SSH client that connect to a remote folie as a network
// input. Locally it takes the place of a serial port or telnet connection. Therefore SSHClient
// implements MicroConn.
type SSHClient struct {
	addr   string
	client *ssh.Client
	config *ssh.ClientConfig
	//addTxWriter func(io.Writer)
	session            *ssh.Session
	txReader, rxReader io.Reader
	txWriter, rxWriter io.Writer
}

func (sc *SSHClient) Reset(bootloader bool) bool { return false }
func (sc *SSHClient) Close() error               { return nil }

// NewSSHClient prepares the crypto info to connect to a remote folie process via SSH. It returns an
// SSHClient on which Open can be called multiple times to open an re-open the connection.
func NewSSHClient(addr string) (*SSHClient, error) {
	home := os.Getenv("HOME")
	knownHosts := filepath.Join(home, ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(knownHosts)
	if err != nil {
		return nil, err
	}

	// Assume unencrypted private key...
	keyFile := filepath.Join(home, ".ssh", "tve-2016")
	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot open SSH key file %s: %s", keyFile, err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("cannot parse SSH key file %s: %s", keyFile, err)
	}

	config := &ssh.ClientConfig{
		HostKeyCallback: hostKeyCallback,
		ClientVersion:   "JeeLabs folie",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}
	return &SSHClient{addr: addr, config: config}, nil
}

// Open dials the SSH connection to the remote folie server. It also requests and interactive shell
// session.
func (sc *SSHClient) Open() error {
	// Dial the connection.
	client, err := ssh.Dial("tcp", sc.addr, sc.config)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %s", sc.addr, err)
	}
	sc.client = client

	// Create pipes for output and for input.
	sc.txReader, sc.txWriter = io.Pipe() // local console -> remote input
	sc.rxReader, sc.rxWriter = io.Pipe() // remote output -> local stdout

	// Prepare an interactive "shell" session.
	sc.session, err = sc.client.NewSession()
	if err != nil {
		return err
	}

	sc.session.Stdin = sc.txReader
	sc.session.Stdout = sc.rxWriter
	sc.session.Stderr = nil // folie doesn't use stderr

	// Start the session.
	if err = sc.session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell session: %s", err)
	}

	return nil
}

func (sc *SSHClient) Read(buf []byte) (int, error)  { return sc.rxReader.Read(buf) }
func (sc *SSHClient) Write(buf []byte) (int, error) { return sc.txWriter.Write(buf) }

func (sc *SSHClient) Flash(pgm []byte) {
	sess, err := sc.client.NewSession()
	if err != nil {
		fmt.Fprintf(sc.rxWriter, "Error flashing: %s\n", err)
		return
	}

	sess.Stdin = bytes.NewReader(pgm)
	sess.Stdout = sc.rxWriter

	// Run the flashing session, this blocks until it's done.
	if err = sess.Run("flash"); err != nil {
		fmt.Fprintf(sc.rxWriter, "Error flashing: %s\n", err)
		return
	}
}
