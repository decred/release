//go:build ignore
// +build ignore

package main

import (
	"log"
	"os"

	"github.com/decred/dcrd/sampleconfig"
)

var assets = map[string]func() string{
	"sample-dcrd.conf":   sampleconfig.FileContents,
	"sample-dcrctl.conf": func() string { return sampleconfig.DcrctlSampleConfig },
}

func main() {
	asset := os.Args[1]
	contents, ok := assets[asset]
	if !ok {
		log.Fatalf("missing asset %q", asset)
	}
	_, err := os.Stdout.Write([]byte(contents()))
	if err != nil {
		log.Fatal(err)
	}
}
