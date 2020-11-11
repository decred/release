This release tool cross-compiles reproducible executables and release archives
(.tar.gz or .zip depending on platform) for Decred releases.

Usage:

```
$ go run .
```

Executables will be written to a `bin` directory and archived or copied into the
`dist` directory.  SHA256 hashes of all files for a distribution are written to
a manifest file, also found in the `dist` directory for each distribution.

The build must be performed from within the Git repository, and it is not
recommended to install the release builder using `go install`.
