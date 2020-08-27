//+build ignore

package main

import (
	"log"
	"os"

	"github.com/decred/politeia/politeiawww/cmd/politeiavoter/sampleconfig"
)

var assets = map[string][]byte{
	"sample-politeiavoter.conf": []byte(sampleconfig.FileContents),
}

func main() {
	asset := os.Args[1]
	contents, ok := assets[asset]
	if !ok {
		log.Fatalf("missing asset %q", asset)
	}
	_, err := os.Stdout.Write(contents)
	if err != nil {
		log.Fatal(err)
	}
}
