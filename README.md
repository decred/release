This release tool cross-compiles reproducible executables and release archives
(.tar.gz or .zip depending on platform) for Decred releases.

Usage:

```
$ go build
$ ./release
```

Executables will be written to a `bin` directory and archived per-platform in
the `archive` directory.  SHA256 hashes of each archive are written to a
manifest file, also found in the `archive` directory.

The release tool must be run from within the Git repository, and it is recommend
to only build it using `go build`, not `go install`.
