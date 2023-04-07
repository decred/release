package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
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
	target    = flag.String("target", "", "only build for os/arch")
	onlydist  = flag.String("dist", "", "only release this distribution (one of: decred dexc dcrinstall dcrinstall-manifests)")
)

type tuple struct{ os, arch string }

var targets = []tuple{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"freebsd", "amd64"},
	{"linux", "386"},
	{"linux", "amd64"},
	{"linux", "arm"},
	{"linux", "arm64"},
	{"linux", "riscv64"},
	{"openbsd", "amd64"},
	{"openbsd", "arm64"},
	{"windows", "386"},
	{"windows", "amd64"},
	{"windows", "arm64"},
}

type dist struct {
	dist      string
	relver    string
	fake      func(*dist)
	tools     []buildtool
	assets    []buildasset
	ldflags   string
	plainbins bool
	manifest  manifest
	sum       *[32]byte
}

type buildtool struct{ tool, builddir string }

type buildasset struct {
	builddir string
	name     string
	goargs   []string
	contents []byte
}

type manifestLine struct {
	name string
	hash [32]byte
}

type manifest []manifestLine

const (
	decredRelver = "v1.7.5"
	dexcRelver   = "v0.5.6"
	ldVersion    = "1.7.5"
	dexcLdVer    = "0.5.6"
	prerelease   = ""
)

var dists = []dist{{
	dist:   "decred",
	relver: decredRelver,
	tools: []buildtool{
		{"decred.org/dcrctl", "./dcrctl"},
		{"decred.org/dcrwallet/v2", "./dcrwallet"},
		{"github.com/decred/dcrd", "./dcrd"},
		{"github.com/decred/dcrd/cmd/gencerts", "./dcrd"},
		{"github.com/decred/dcrd/cmd/promptsecret", "./dcrd"},
		{"github.com/decred/dcrlnd/cmd/dcrlnd", "./dcrlnd"},
		{"github.com/decred/dcrlnd/cmd/dcrlncli", "./dcrlnd"},
		{"github.com/decred/politeia/politeiawww/cmd/politeiaverify", "./politeia"},
		{"github.com/decred/politeia/politeiawww/cmd/politeiavoter", "./politeia"},
	},
	assets: []buildasset{
		{
			builddir: "./dcrwallet",
			name:     "sample-dcrwallet.conf",
			goargs:   []string{"run", "readasset.go", "../sample-dcrwallet.conf"},
		},
		{
			builddir: "./dcrd",
			name:     "sample-dcrd.conf",
			goargs:   []string{"run", "readasset.go", "sample-dcrd.conf"},
		},
		{
			builddir: "./dcrd",
			name:     "sample-dcrctl.conf",
			goargs:   []string{"run", "readasset.go", "sample-dcrctl.conf"},
		},
		{
			builddir: "./dcrlnd",
			name:     "sample-dcrlnd.conf",
			goargs:   []string{"run", "readasset.go", "../sample-dcrlnd.conf"},
		},
		{
			builddir: "./politeia",
			name:     "sample-politeiavoter.conf",
			goargs:   []string{"run", "readasset.go", "sample-politeiavoter.conf"},
		},
	},
	ldflags: fmt.Sprintf(`-buildid= -s -w `+
		`-X github.com/decred/dcrd/internal/version.Version=%[1]s+release `+
		`-X decred.org/dcrwallet/v2/version.BuildMetadata=release `+
		`-X decred.org/dcrwallet/v2/version.PreRelease=%[2]s `+
		`-X github.com/decred/dcrlnd/build.BuildMetadata=release `+
		`-X github.com/decred/dcrlnd/build.PreRelease=%[2]s `+
		`-X github.com/decred/politeia/util/version.BuildMetadata=release `+
		`-X github.com/decred/politeia/util/version.PreRelease=%[2]s `+
		`-X main.BuildMetadata=release `+
		`-X main.PreRelease=%[2]s`, ldVersion, prerelease),
}, {
	dist:   "dexc",
	relver: dexcRelver,
	tools: []buildtool{
		{"decred.org/dcrdex/client/cmd/dexc", "./dcrdex"},
		{"decred.org/dcrdex/client/cmd/dexcctl", "./dcrdex"},
	},
	ldflags: fmt.Sprintf(`-buildid= -s -w `+
		`-X main.Version=%s+release`, dexcLdVer),
}, {
	dist:   "dcrinstall",
	relver: decredRelver,
	tools: []buildtool{
		{"github.com/decred/decred-release/cmd/dcrinstall", "./decred-release"},
	},
	ldflags: `-buildid= -s -w ` +
		`-X main.appBuild=release ` +
		`-X main.dcrinstallManifestVersion=` + decredRelver,
	plainbins: true,
}, {
	dist:   "dcrinstall-manifests",
	relver: decredRelver,
	fake: (&dcrinstallManifest{
		dcrurls: []string{
			ghRelease("decred-binaries", decredRelver, "decred-"+decredRelver+"-manifest.txt"),
			ghRelease("decred-binaries", decredRelver, "dexc-"+dexcRelver+"-manifest.txt"),
			ghRelease("decred-release", decredRelver, "dcrinstall-"+decredRelver+"-manifest.txt"),
		},
	}).fakedist,
}}

func ghRelease(repo, relver, file string) string {
	return "https://github.com/decred/" + repo + "/releases/download/" + relver + "/" + file
}

const defaultTags = "safe,netgo"

var tags = map[string]string{
	"darwin":  "safe",
	"openbsd": "safe",
	"windows": "safe",
}

func main() {
	flag.Parse()
	buildinfo := buildinfo()

	if slash := strings.IndexByte(*target, '/'); slash != -1 {
		os, arch := (*target)[:slash], (*target)[slash+1:]
		targets = append(targets[:0], tuple{os, arch})
	}

	for i := range dists {
		if *onlydist != "" && *onlydist != dists[i].dist {
			continue
		}
		name := dists[i].dist
		relver := dists[i].relver
		log.Printf(`releasing dist "%s-%s" with %s %s`, name, relver, *gobin, buildinfo)
		dists[i].release()
	}
	for i := range dists {
		if dists[i].sum == nil {
			continue
		}
		log.Printf("dist %q manifest hash: SHA256:%x (%s)",
			dists[i].dist, dists[i].sum[:], buildinfo)
	}
}

func (d *dist) release() {
	if d.fake != nil {
		d.fake(d)
		return
	}
	for i, a := range d.assets {
		d.assets[i].contents = readasset(a.builddir, a.goargs)
	}
	for _, target := range targets {
		for _, tool := range d.tools {
			if *nobuild {
				break
			}
			build(tool.tool, tool.builddir, target.os, target.arch, d.ldflags)
		}
		if *noarchive {
			continue
		}
		if d.plainbins {
			d.distbins(target.os, target.arch)
			continue
		}
		d.archive(target.os, target.arch)
	}
	if len(d.manifest) > 0 && *target == "" {
		d.writeManifest()
	}
}

func buildinfo() string {
	output, err := exec.Command(*gobin, "version").CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimRight(string(output), "\r\n")
}

func toolName(module string) string {
	isMajor := func(s string) bool {
		for _, v := range s {
			if v < '0' || v > '9' {
				return false
			}
		}
		return len(s) > 0
	}
	tool := path.Base(module)
	// strip /v2+
	if tool[0] == 'v' && isMajor(tool[1:]) {
		tool = path.Base(path.Dir(module))
	}
	return tool
}

func exeName(module, goos string) string {
	exe := toolName(module)
	if goos == "windows" {
		exe += ".exe"
	}
	return exe
}

func readasset(builddir string, goargs []string) []byte {
	cmd := exec.Command(*gobin, goargs...)
	cmd.Dir = builddir
	output, err := cmd.Output()
	if err != nil {
		log.Printf("failed to readasset: dir=%v goargs=%v", builddir, goargs)
		log.Fatal(err)
	}
	return output
}

func build(tool, builddir, goos, arch, ldflags string) {
	tags, ok := tags[goos]
	if !ok {
		tags = defaultTags
	}
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
	if arch == "arm" {
		os.Setenv("GOARM", "7")
	}
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

func (d *dist) distbins(goos, arch string) {
	distdir := filepath.Join("dist", d.dist+"-"+d.relver)
	if _, err := os.Stat(distdir); os.IsNotExist(err) {
		err := os.MkdirAll(distdir, 0777)
		if err != nil {
			log.Fatal(err)
		}
	}
	hash := sha256.New()
	for i := range d.tools {
		tool := toolName(d.tools[i].tool)
		srcexe := tool
		if goos == "windows" {
			srcexe += ".exe"
		}
		srcpath := filepath.Join("bin", goos+"-"+arch, srcexe)
		src, err := os.Open(srcpath)
		if err != nil {
			log.Fatal(err)
		}
		defer src.Close()
		dstpath := fmt.Sprintf("dist/%s-%s/%s-%s-%s-%[2]s",
			d.dist, d.relver, tool, goos, arch)
		if goos == "windows" {
			dstpath += ".exe"
		}
		log.Printf("dist: %v", dstpath)
		dst, err := os.OpenFile(dstpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			log.Fatal(err)
		}
		hash.Reset()
		w := io.MultiWriter(dst, hash)
		_, err = io.Copy(w, src)
		if err != nil {
			log.Fatal(err)
		}
		err = dst.Close()
		if err != nil {
			log.Fatal(err)
		}

		var sum [32]byte
		copy(sum[:], hash.Sum(nil))
		d.manifest = append(d.manifest, manifestLine{filepath.Base(dst.Name()), sum})
	}
}

func (d *dist) archive(goos, arch string) {
	distdir := filepath.Join("dist", d.dist+"-"+d.relver)
	if _, err := os.Stat(distdir); os.IsNotExist(err) {
		err := os.MkdirAll(distdir, 0777)
		if err != nil {
			log.Fatal(err)
		}
	}
	if goos == "windows" {
		d.archiveZip(goos, arch)
		return
	}
	tarPath := fmt.Sprintf("%s-%s-%s-%s", d.dist, goos, arch, d.relver)
	tarFile, err := os.Create(filepath.Join(distdir, tarPath+".tar"))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("archive: %v", tarFile.Name()+".gz")
	tw := tar.NewWriter(tarFile)
	addFile := func(name string, r io.Reader, mode, size int64) {
		var ty byte = tar.TypeReg
		const modedir = int64(os.ModeDir)
		if mode&modedir == modedir {
			ty = tar.TypeDir
			mode &^= modedir
			size = 0
		}
		hdr := &tar.Header{
			Name:     strings.ReplaceAll(tarPath+"/"+name, `\`, `/`),
			Typeflag: ty,
			Mode:     mode,
			Size:     size,
			Format:   tar.FormatPAX,
		}
		err = tw.WriteHeader(hdr)
		if err != nil {
			log.Fatal(err)
		}
		if r != nil {
			_, err = io.Copy(tw, r)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	addFile("", nil, int64(os.ModeDir|0755), 0) // add tarPath directory
	for i := range d.tools {
		exe := exeName(d.tools[i].tool, goos)
		exePath := filepath.Join("bin", goos+"-"+arch, exe)
		info, err := os.Stat(exePath)
		if err != nil {
			log.Fatal(err)
		}
		file, err := os.Open(exePath)
		if err != nil {
			log.Fatal(err)
		}
		addFile(exe, file, 0755, info.Size())
		file.Close()
	}
	for _, a := range d.assets {
		addFile(a.name, bytes.NewBuffer(a.contents), 0644, int64(len(a.contents)))
	}
	err = tw.Close()
	if err != nil {
		log.Fatal(err)
	}
	zf, err := os.Create(tarFile.Name() + ".gz")
	if err != nil {
		log.Fatal(err)
	}
	hash := sha256.New()
	defer func() {
		name := filepath.Base(tarFile.Name()) + ".gz"
		var sum [32]byte
		copy(sum[:], hash.Sum(nil))
		d.manifest = append(d.manifest, manifestLine{name, sum})
	}()
	w := io.MultiWriter(zf, hash)
	zw := gzip.NewWriter(w)
	_, err = tarFile.Seek(0, io.SeekStart)
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

func (d *dist) archiveZip(goos, arch string) {
	zipPath := fmt.Sprintf("%s-%s-%s-%s", d.dist, goos, arch, d.relver)
	zipFile, err := os.Create(fmt.Sprintf("dist/%s-%s/%s.zip", d.dist, d.relver, zipPath))
	if err != nil {
		log.Fatal(err)
	}
	defer zipFile.Close()
	hash := sha256.New()
	w := io.MultiWriter(zipFile, hash)
	defer func() {
		name := filepath.Base(zipFile.Name())
		var sum [32]byte
		copy(sum[:], hash.Sum(nil))
		d.manifest = append(d.manifest, manifestLine{name, sum})
	}()
	log.Printf("dist: %v", zipFile.Name())
	zw := zip.NewWriter(w)
	addFile := func(name string, r io.Reader) {
		h := &zip.FileHeader{
			Name:   strings.ReplaceAll(zipPath+"/"+name, `\`, `/`),
			Method: zip.Deflate,
		}
		f, err := zw.CreateHeader(h)
		if err != nil {
			log.Fatal(err)
		}
		if r != nil {
			_, err = io.Copy(f, r)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	addFile("", nil) // create zipPath directory
	for i := range d.tools {
		exe := exeName(d.tools[i].tool, goos)
		exePath := filepath.Join("bin", goos+"-"+arch, exe)
		exeFi, err := os.Open(exePath)
		if err != nil {
			log.Fatal(err)
		}
		addFile(exe, exeFi)
		exeFi.Close()
	}
	for _, a := range d.assets {
		addFile(a.name, bytes.NewBuffer(a.contents))
	}
	err = zw.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func (d *dist) writeManifest() {
	fi, err := os.Create(fmt.Sprintf("dist/%s-%s/%[1]s-%[2]s-manifest.txt", d.dist, d.relver))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("manifest: %v", fi.Name())
	buf := new(bytes.Buffer)
	for _, mline := range d.manifest {
		_, err = fmt.Fprintf(buf, "%x  %s\n", mline.hash, mline.name)
		if err != nil {
			log.Fatal(err)
		}
	}
	sum := sha256.Sum256(buf.Bytes())
	d.sum = &sum
	_, err = io.Copy(fi, buf)
	if err != nil {
		log.Fatal(err)
	}
	err = fi.Close()
	if err != nil {
		log.Fatal(err)
	}
}

type dcrinstallManifest struct {
	*dist
	dcrurls []string
}

func (d *dcrinstallManifest) fakedist(dist *dist) {
	if *target != "" {
		return
	}
	d.dist = dist
	outpath := fmt.Sprintf("dist/dcrinstall-%s-manifests.txt", d.relver)
	out, err := os.Create(outpath)
	if err != nil {
		log.Fatal(err)
	}
	fakeout, err := os.Create("fake-latest")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("dist: %v", outpath)
	hash := sha256.New()
	outhash := sha256.New()
	w := io.MultiWriter(out, outhash)
	for _, u := range d.dcrurls {
		uu, err := url.Parse(u)
		if err != nil {
			log.Fatal(err)
		}
		name := path.Base(uu.Path)
		sep := strings.IndexByte(name, '-')
		dist := name[:sep]
		relver := strings.TrimSuffix(name[sep+1:], "-manifest.txt")
		localpath := fmt.Sprintf("dist/%s-%s/%[1]s-%[2]s-manifest.txt", dist, relver)
		fi, err := os.Open(localpath)
		if os.IsNotExist(err) {
			log.Fatalf("dependency %s not satisified", localpath)
		}
		if err != nil {
			log.Fatal(err)
		}
		hash.Reset()
		io.Copy(hash, fi)
		fi.Close()
		sum := hash.Sum(nil)
		_, err = fmt.Fprintf(w, "%x  %s\n", sum, u)
		if err != nil {
			log.Fatal(err)
		}
		_, err = fmt.Fprintf(fakeout, "%x  %s\n", sum, "file://"+localpath)
		if err != nil {
			log.Fatal(err)
		}
	}
	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
	err = fakeout.Close()
	if err != nil {
		log.Fatal(err)
	}
	d.sum = new([32]byte)
	copy(d.sum[:], outhash.Sum(nil))
}
