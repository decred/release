// +build assets

package main

import (
	"fmt"

	"decred.org/dcrdex-assets/assets"
)

func main() {
	fmt.Println(assets.Path("dexc/site"))
}
