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

const dldflags = `-X github.com/decred/dcrd/internal/version.BuildMetadata=release ` +
	`-X github.com/decred/dcrd/internal/version.PreRelease=`
const wldflags = `-X github.com/decred/dcrwallet/version.BuildMetadata=release ` +
	`-X github.com/decred/dcrwallet/version.PreRelease=`

var modules = []struct{ mod, ldflags string }{
	// dcrd release-v1.4.0 is broken due to replaces in main module
	//{"github.com/decred/dcrd", dldflags},
	{"github.com/decred/dcrd/cmd/dcrctl", dldflags},
	{"github.com/decred/dcrd/cmd/promptsecret", dldflags},
	{"github.com/decred/dcrwallet", wldflags},
}

func main() {
	flag.Parse()
	logvers()
	os.Setenv("GOBIN", "./bin")
	for i := range modules {
		for j := range targets {
			build(modules[i].mod, modules[i].ldflags, targets[j].os, targets[j].arch)
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

func build(module, ldflags, goos, arch string) {
	exe := path.Base(module)
	if goos == "windows" {
		exe += ".exe"
	}
	// TODO: add -trimpath with Go 1.13
	gocmd(goos, arch, "build", "-o", filepath.Join("./bin", goos+"-"+arch, exe), "-ldflags", ldflags, module)
}

func gocmd(goos, arch string, args ...string) {
	os.Setenv("GOOS", goos)
	os.Setenv("GOARCH", arch)
	log.Printf("> go %v", strings.Join(args, " "))
	output, err := exec.Command("go", args...).CombinedOutput()
	if len(output) != 0 {
		log.Printf("> %s", output)
	}
	if err != nil {
		log.Fatal(err)
	}
}
