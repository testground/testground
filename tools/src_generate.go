// +build ignore

package main

import (
	"fmt"
	"net/http"

	"github.com/shurcooL/vfsgen"
)

// TODO not finished yet. The goal is to bundle the source for the sdk with
// binary builds of the testground, so it can be accessed by builders to insert
// go.mod replace directives.
func main() {
	err := vfsgen.Generate(http.Dir("./sdk"), vfsgen.Options{Filename: "sdk_src_generated.go"})
	if err != nil {
		panic(fmt.Errorf("unable to generate assets: %w", err))
	}
}
