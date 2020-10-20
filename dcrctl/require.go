// +build require

// This file exists to prevent go mod tidy from removing requires on tools.
// It is excluded from the build as it is not permitted to import main packages.

package main

import (
	_ "decred.org/dcrctl"
)
