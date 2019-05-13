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
	for i := range targets {
		for j := range modules {
			build(modules[j].mod, modules[j].ldflags, targets[i].os, targets[i].arch)
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
	out := filepath.Join("bin", goos+"-"+arch, exe)
	log.Printf("build: %s", out)
	// TODO: add -trimpath with Go 1.13
	gocmd(goos, arch, "build", "-o", out, "-ldflags", ldflags, module)
}

func gocmd(goos, arch string, args ...string) {
	os.Setenv("GOOS", goos)
	os.Setenv("GOARCH", arch)
	output, err := exec.Command(*gobin, args...).CombinedOutput()
	if len(output) != 0 {
		log.Printf("go '%s'\n%s", strings.Join(args, `' '`), output)
	}
	if err != nil {
		log.Fatal(err)
	}
}
