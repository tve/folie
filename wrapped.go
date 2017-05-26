package folie

// This file contains !commands.

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

// The functions in this file are only called in the context of interactive console input and they
// all print directly to stdout. This is OK in this context, but may have to be changed in the
// future if we want to support a remote interactive console. Not clear there is a need for that.

// specialCommand recognises and handles certain commands in a different way.
func (sw *Switchboard) specialCommand(line string) bool {
	cmd := strings.SplitN(line, " ", 2)
	switch cmd[0] {
	case "!":
		fmt.Println("[enter '!h' for help]")

	case "!c", "!cd":
		fmt.Println(line)
		wrappedCd(cmd)

	case "!h", "!help":
		fmt.Println(line)
		showHelp()

	case "!l", "!ls":
		fmt.Println(line)
		wrappedLs(cmd)

	case "!r", "!reset":
		fmt.Println(line)
		sw.wrappedReset()

	case "!s", "!send":
		fmt.Println(line)
		sw.wrappedSend(cmd)

	case "!u", "!upload":
		fmt.Println(line)
		sw.wrappedUpload(cmd)

	default:
		return false
	}
	return true
}

const helpMsg = `
Special commands, these can also be abbreviated as "!r", etc:
  !reset          reset the board, same as ctrl-c
  !send <file>    send text file to the serial port, expand "include" lines
  !upload         show the list of built-in firmware images
  !upload <n>     upload built-in image <n> using STM32 boot protocol
  !upload <file>  upload specified firmware image (bin or hex format)
  !upload <url>   fetch firmware image from given URL, then upload it
Utility commands:
  !cd <dir>       change directory (or list current one if not specified)
  !ls <dir>       list contents of the specified (or current) directory
  !help           this message
To quit, hit ctrl-d. For command history, use up-/down-arrow.
`

func showHelp() {
	fmt.Print(helpMsg[1:])
}

func wrappedCd(argv []string) {
	if len(argv) > 1 {
		if err := os.Chdir(argv[1]); err != nil {
			fmt.Println(err)
			return
		}
	}
	if dir, err := os.Getwd(); err == nil {
		fmt.Println(dir)
	} else {
		fmt.Println(err)
	}
}

func wrappedLs(argv []string) {
	dir := "."
	if len(argv) > 1 {
		dir = argv[1]
	}
	var names []string
	files, _ := ioutil.ReadDir(dir)
	for _, f := range files {
		n := f.Name()
		if !strings.HasPrefix(n, ".") {
			if f.IsDir() {
				n = n + "/"
			}
			names = append(names, n)
		}
	}
	fmt.Println(strings.Join(names, " "))
}

func (sw *Switchboard) wrappedReset() {
	if ok := sw.MicroOutput.Reset(false); !ok {
		// Couldn't perform the reset, probably error on serial/telnet.
		fmt.Println("[use CTRL-D to exit]")
	}
}

func (sw *Switchboard) wrappedSend(argv []string) {
	if len(argv) == 1 {
		fmt.Printf("Usage: %s <filename>\n", argv[0])
		return
	}
	if !includeFile(sw.MicroOutput, sw.MicroInput, argv[1], 0) { // FIXME: need tx channel
		fmt.Println("Send failed.")
	}
}

var crcTable = []uint16{
	0x0000, 0xCC01, 0xD801, 0x1400, 0xF001, 0x3C00, 0x2800, 0xE401,
	0xA001, 0x6C00, 0x7800, 0xB401, 0x5000, 0x9C01, 0x8801, 0x4400,
}

func crc16(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc = (crc >> 4) ^ crcTable[crc&0x0F] ^ crcTable[b&0x0F]
		crc = (crc >> 4) ^ crcTable[crc&0x0F] ^ crcTable[(b>>4)&0x0F]
	}
	return crc
}

func (sw *Switchboard) wrappedUpload(argv []string) {
	names := sw.AssetNames
	sort.Strings(names)

	if len(argv) == 1 {
		fmt.Println("These firmware images are built-in:")
		for i, name := range names {
			data, _ := sw.Asset(name)
			fmt.Printf("%3d: %-16s %5db  crc:%04X\n",
				i+1, name, len(data), crc16(data))
		}
		fmt.Println("Use '!u <n>' to upload a specific one.")
		return
	}

	// try built-in images first, indicated by entering a valid number
	var data []byte
	if n, err := strconv.Atoi(argv[1]); err == nil && 0 < n && n <= len(names) {
		data, _ = sw.Asset(names[n-1])
	} else if u, err := url.Parse(argv[1]); err == nil && u.Scheme != "" {
		fmt.Print("Fetching... ")
		res, err := http.Get(argv[1])
		if err == nil {
			data, err = ioutil.ReadAll(res.Body)
			res.Body.Close()
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			fmt.Printf("%s: %s\n", res.Status, string(data))
		}
		fmt.Printf("got it, crc:%04X\n", crc16(data))
	} else { // else try opening the arg as file
		f, err := os.Open(argv[1])
		if err == nil {
			data, err = ioutil.ReadAll(f)
			f.Close()
		}
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	defer sw.MicroOutput.Reset(false) // reset with BOOT0 low to restart normally

	u := &Uploader{Tx: sw.MicroOutput, Rx: sw.MicroInput}
	u.Upload(data)
}
