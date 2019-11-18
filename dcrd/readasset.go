//+build ignore

package main

import (
	"log"
	"os"

	"github.com/decred/dcrd/sampleconfig"
)

var assets = map[string][]byte{
	"sample-dcrd.conf":   []byte(sampleconfig.FileContents),
	//"sample-dcrctl.conf": []byte(sampleconfig.DcrctlSampleConfig),
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
