//+build ignore

package main

import (
	"io"
	"log"
	"os"

	"github.com/decred/dcrlnd/assets"
)

func main() {
	asset := os.Args[1]
	path := assets.Path(asset)
	fi, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer fi.Close()
	_, err = io.Copy(os.Stdout, fi)
	if err != nil {
		log.Fatal(err)
	}
}
