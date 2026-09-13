// Package testrunner provides a scriptable execx.Runner fake for tests.
package testrunner

import (
	"fmt"
	"strings"

	"github.com/SynthLuvlr/gocanon/internal/execx"
)

// Fake records every command and answers from test-provided closures.
// Nil closures default to success.
type Fake struct {
	// RunFn answers Run calls; nil means exit 0.
	RunFn func(name string, args []string) (int, error)
	// OutputFn answers Output calls; nil returns the joined command.
	OutputFn func(name string, args []string) ([]byte, error)
	// ResolveFn answers Resolve calls; nil returns a fake absolute path.
	ResolveFn func(name string) (string, error)
	// Commands records every Run/Output/Resolve invocation, as
	// "name arg1 arg2" strings.
	Commands []string
}

// Compile-time check that Fake satisfies Runner.
var _ execx.Runner = (*Fake)(nil)

// Run implements execx.Runner.
func (f *Fake) Run(name string, args ...string) (int, error) {
	f.Commands = append(f.Commands, join(name, args))
	if f.RunFn == nil {
		return 0, nil
	}
	return f.RunFn(name, args)
}

// Output implements execx.Runner.
func (f *Fake) Output(name string, args ...string) ([]byte, error) {
	f.Commands = append(f.Commands, join(name, args))
	if f.OutputFn == nil {
		return []byte(join(name, args) + "\n"), nil
	}
	return f.OutputFn(name, args)
}

// Resolve implements execx.Runner.
func (f *Fake) Resolve(name string) (string, error) {
	if f.ResolveFn == nil {
		return "/fake/bin/" + name, nil
	}
	return f.ResolveFn(name)
}

// Has reports whether any recorded command starts with prefix.
func (f *Fake) Has(prefix string) bool {
	for _, cmd := range f.Commands {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

// join formats a command for the Commands log.
func join(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return fmt.Sprintf("%s %s", name, strings.Join(args, " "))
}
