package app

import (
	"fmt"

	"github.com/SynthLuvlr/go-canon/internal/cover"
)

// coverProfile is the coverage artifact written at the module root.
const coverProfile = "cover.out"

// runTest runs the test suite with race detection and atomic coverage
// across all packages, then gates the total statement coverage.
func runTest(e *env) int {
	fmt.Fprintln(e.stdout, "==> go test -race -coverpkg=./... -coverprofile="+coverProfile)
	code, err := e.r.Run(
		"go", "test", "./...", "-race",
		"-covermode=atomic", "-coverprofile="+coverProfile, "-coverpkg=./...",
	)
	if err != nil {
		fmt.Fprintf(e.stderr, "go-canon: %v\n", err)
		return 1
	}
	if code != 0 {
		fmt.Fprintf(e.stdout, "==> go test failed (exit %d)\n", code)
		return code
	}
	out, err := e.r.Output("cover", "-func="+coverProfile)
	if err != nil {
		fmt.Fprintf(e.stderr, "go-canon: %v\n", err)
		return 1
	}
	total, err := cover.ParseTotal(string(out))
	if err != nil {
		fmt.Fprintf(e.stderr, "go-canon: %v\n", err)
		return 1
	}
	threshold := e.cfg.Test.CoverageThreshold
	fmt.Fprintf(e.stdout, "==> coverage %.1f%% (threshold %.1f%%)\n", total, threshold)
	if total+0.05 < threshold {
		fmt.Fprintf(e.stderr, "go-canon: coverage %.1f%% is below the %.1f%% threshold\n", total, threshold)
		return 1
	}
	return 0
}

// runCheck runs lint, then test.
func runCheck(e *env) int {
	if code := runLint(e); code != 0 {
		return code
	}
	return runTest(e)
}
