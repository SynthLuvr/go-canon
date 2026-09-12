// Package main implements the go-canon command: the shared Go toolchain
// gates and presets, driven by each repo's pinned `tool` directives.
package main

import (
	"os"

	"github.com/SynthLuvlr/go-canon/internal/app"
	"github.com/SynthLuvlr/go-canon/internal/execx"
)

func main() {
	os.Exit(app.Run(os.Args[1:], execx.New(os.Stdout, os.Stderr), os.Stdout, os.Stderr))
}
