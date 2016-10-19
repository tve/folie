package main

import (
	"fmt"
)

func Uploader(data []byte) {
	defer fmt.Println()
	fmt.Printf("  Uploading %d bytes: ", len(data))
}
