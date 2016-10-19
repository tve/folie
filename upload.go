package main

import (
	"io"
	"fmt"
)

func Uploader(data []byte, conn io.ReadWriter) {
	defer fmt.Println()
	fmt.Printf("  Uploading %d bytes: ", len(data))
}
