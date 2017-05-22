package folie

// This file contains !commands.

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

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

func wrappedReset() {
	/*
		if dev == nil {
			fmt.Println("[use CTRL-D to exit]")
		} else {
			boardReset(false)
		}
	*/
}

func wrappedSend(argv []string) {
	if len(argv) == 1 {
		fmt.Printf("Usage: %s <filename>\n", argv[0])
		return
	}
	if !IncludeFile(nil, argv[1], 0) { // FIXME: need tx channel
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

func wrappedUpload(argv []string) {
	/*
		names := AssetNames()
		sort.Strings(names)

		if len(argv) == 1 {
			fmt.Println("These firmware images are built-in:")
			for i, name := range names {
				data, _ := Asset(name)
				fmt.Printf("%3d: %-16s %5db  crc:%04X\n",
					i+1, name, len(data), crc16(data))
			}
			fmt.Println("Use '!u <n>' to upload a specific one.")
			return
		}

		// try built-in images first, indicated by entering a valid number
		var data []byte
		if n, err := strconv.Atoi(argv[1]); err == nil && 0 < n && n <= len(names) {
			data, _ = Asset(names[n-1])
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

		if tty != nil && *raw {
			// temporarily switch to even parity during upload
			tty.SetMode(&serial.Mode{BaudRate: *baud, Parity: serial.EvenParity})
			defer tty.SetMode(&serial.Mode{BaudRate: *baud})
		}

		defer boardReset(false) // reset with BOOT0 low to restart normally

		Uploader(data)
	*/
}
