package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
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

var (
	gobin     = flag.String("go", findGo(), "Go binary")
	nobuild   = flag.Bool("nobuild", false, "skip go build")
	noarchive = flag.Bool("noarchive", false, "skip archiving")
)

var targets = []struct{ os, arch string }{
	{"linux", "amd64"},
	{"linux", "386"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"windows", "amd64"},
	{"windows", "386"},
	{"openbsd", "amd64"},
	{"freebsd", "amd64"},
}

const relver = "v1.5.0-rc1"

const ldflags = `-buildid= ` +
	`-X github.com/decred/dcrd/internal/version.BuildMetadata=release ` +
	`-X github.com/decred/dcrd/internal/version.PreRelease=rc1 ` +
	`-X github.com/decred/dcrwallet/version.BuildMetadata=release ` +
	`-X github.com/decred/dcrwallet/version.PreRelease=rc1`

const tags = "safe"

var tools = []struct{ tool, builddir string }{
	{"decred.org/dcrwallet", "./dcrwallet"},
	{"github.com/decred/dcrd", "./dcrd"},
	{"github.com/decred/dcrd/cmd/dcrctl", "./dcrd"},
	{"github.com/decred/dcrd/cmd/promptsecret", "./dcrd"},
	{"github.com/decred/dcrlnd/cmd/dcrlnd", "./dcrlnd"},
}

func main() {
	flag.Parse()
	logvers()
	for i := range targets {
		for j := range tools {
			if *nobuild {
				break
			}
			build(tools[j].tool, targets[i].os, targets[i].arch, tools[j].builddir)
		}
		if *noarchive {
			continue
		}
		archive(targets[i].os, targets[i].arch)
	}
}

func logvers() {
	output, err := exec.Command(*gobin, "version").CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("releasing with %s %s", *gobin, output)
}

func exeName(module, goos string) string {
	exe := path.Base(module) // TODO: fix for /v2+
	if goos == "windows" {
		exe += ".exe"
	}
	return exe
}

func build(tool, goos, arch, builddir string) {
	exe := exeName(tool, goos)
	out := filepath.Join("..", "bin", goos+"-"+arch, exe)
	log.Printf("build: %s", out[3:]) // trim off leading "../"
	gocmd(goos, arch, builddir, "build", "-trimpath", "-tags", tags, "-o", out, "-ldflags", ldflags, tool)
}

func gocmd(goos, arch, builddir string, args ...string) {
	os.Setenv("GOOS", goos)
	os.Setenv("GOARCH", arch)
	os.Setenv("CGO_ENABLED", "0")
	os.Setenv("GOFLAGS", "")
	cmd := exec.Command(*gobin, args...)
	cmd.Dir = builddir
	output, err := cmd.CombinedOutput()
	if len(output) != 0 {
		log.Printf("go '%s'\n%s", strings.Join(args, `' '`), output)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func archive(goos, arch string) {
	if _, err := os.Stat("archive"); os.IsNotExist(err) {
		err := os.Mkdir("archive", 0777)
		if err != nil {
			log.Fatal(err)
		}
	}
	if goos == "windows" {
		archiveZip(goos, arch)
		return
	}
	tarPath := fmt.Sprintf("decred-%s-%s-%s", goos, arch, relver)
	tarFile, err := os.Create(fmt.Sprintf("archive/%s.tar", tarPath))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("archive: %v", tarFile.Name()+".gz")
	tw := tar.NewWriter(tarFile)
	dirInfo, err := os.Stat(filepath.Join("bin", goos+"-"+arch))
	if err != nil {
		log.Fatal(err)
	}
	hdr, err := tar.FileInfoHeader(dirInfo, "")
	if err != nil {
		log.Fatal(err)
	}
	hdr.Name = tarPath
	hdr.Format = tar.FormatPAX
	err = tw.WriteHeader(hdr)
	if err != nil {
		log.Fatal(err)
	}
	for i := range tools {
		exe := exeName(tools[i].tool, goos)
		exePath := filepath.Join("bin", goos+"-"+arch, exe)
		info, err := os.Stat(exePath)
		if err != nil {
			log.Fatal(err)
		}
		exeFi, err := os.Open(exePath)
		if err != nil {
			log.Fatal(err)
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			log.Fatal(err)
		}
		hdr.Name = filepath.Join(tarPath, exe)
		hdr.Format = tar.FormatPAX
		err = tw.WriteHeader(hdr)
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.Copy(tw, exeFi)
		if err != nil {
			log.Fatal(err)
		}
		exeFi.Close()
	}
	err = tw.Close()
	if err != nil {
		log.Fatal(err)
	}
	zf, err := os.Create(tarFile.Name() + ".gz")
	if err != nil {
		log.Fatal(err)
	}
	zw := gzip.NewWriter(zf)
	_, err = tarFile.Seek(0, os.SEEK_SET)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(zw, tarFile)
	if err != nil {
		log.Fatal(err)
	}
	err = zw.Close()
	if err != nil {
		log.Fatal(err)
	}
	err = tarFile.Close()
	if err != nil {
		log.Fatal(err)
	}
	err = os.Remove(tarFile.Name())
	if err != nil {
		log.Fatal(err)
	}
}

func archiveZip(goos, arch string) {
	zipPath := fmt.Sprintf("decred-%s-%s-%s", goos, arch, relver)
	zipFile, err := os.Create(fmt.Sprintf("archive/%s.zip", zipPath))
	defer zipFile.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("archive: %v", zipFile.Name())
	zw := zip.NewWriter(zipFile)
	for i := range tools {
		exe := exeName(tools[i].tool, goos)
		exePath := filepath.Join("bin", goos+"-"+arch, exe)
		exeFi, err := os.Open(exePath)
		if err != nil {
			log.Fatal(err)
		}
		h := &zip.FileHeader{
			Name:   filepath.Join(zipPath, exe),
			Method: zip.Deflate,
		}
		f, err := zw.CreateHeader(h)
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.Copy(f, exeFi)
		if err != nil {
			log.Fatal(err)
		}
		exeFi.Close()
	}
	err = zw.Close()
	if err != nil {
		log.Fatal(err)
	}
}
