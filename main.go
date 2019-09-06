package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

func findGo() string {
	path, _ := exec.LookPath("go")
	return path
}

var gobin = flag.String("go", findGo(), "Go binary")

var targets = []struct{ os, arch string }{
	{"linux", "amd64"},
	{"linux", "arm"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"windows", "amd64"},
	{"openbsd", "amd64"},
	{"freebsd", "amd64"},
}

const ldflags = `-buildid= ` +
	`-X github.com/decred/dcrd/internal/version.BuildMetadata=release ` +
	`-X github.com/decred/dcrd/internal/version.PreRelease= ` +
	`-X github.com/decred/dcrwallet/version.BuildMetadata=release ` +
	`-X github.com/decred/dcrwallet/version.PreRelease=`

const tags = "safe"

var tools = []string{
	// dcrd release-v1.4.0 is broken due to replaces in main module
	//"github.com/decred/dcrd",
	"github.com/decred/dcrd/cmd/dcrctl",
	"github.com/decred/dcrd/cmd/promptsecret",
	"github.com/decred/dcrwallet",
}

func main() {
	flag.Parse()
	logvers()
	for i := range targets {
		for j := range tools {
			build(tools[j], targets[i].os, targets[i].arch)
		}
	}
}

func logvers() {
	output, err := exec.Command(*gobin, "version").CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("releasing with %s %s", *gobin, output)
}

func build(tool, goos, arch string) {
	exe := path.Base(tool) // TODO: fix for /v2+
	if goos == "windows" {
		exe += ".exe"
	}
	out := filepath.Join("bin", goos+"-"+arch, exe)
	log.Printf("build: %s", out)
	gocmd(goos, arch, "build", "-trimpath", "-tags", tags, "-o", out, "-ldflags", ldflags, tool)
}

func gocmd(goos, arch string, args ...string) {
	os.Setenv("GOOS", goos)
	os.Setenv("GOARCH", arch)
	os.Setenv("CGO_ENABLED", "0")
	output, err := exec.Command(*gobin, args...).CombinedOutput()
	if len(output) != 0 {
		log.Printf("go '%s'\n%s", strings.Join(args, `' '`), output)
	}
	if err != nil {
		log.Fatal(err)
	}
}
