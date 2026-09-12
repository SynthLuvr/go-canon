package app

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SynthLuvlr/go-canon/internal/execx"
	"github.com/SynthLuvlr/go-canon/internal/module"
	"github.com/SynthLuvlr/go-canon/internal/tools"
)

// osvCheck reports whether the OSV database govulncheck queries is
// reachable; it is a variable so tests can stub the network.
var osvCheck = osvReachable

// osvEndpoint is the govulncheck database root.
const osvEndpoint = "https://vuln.go.dev/"

// osvReachable probes the OSV database with a short timeout.
func osvReachable() bool {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(osvEndpoint) // #nosec G107 -- probing a fixed URL is the point
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

// doctorReport accumulates doctor outcomes.
type doctorReport struct {
	fails  int
	stdout io.Writer
}

// ok records a passing check.
func (d *doctorReport) ok(format string, args ...any) {
	fmt.Fprintf(d.stdout, "ok   "+format+"\n", args...)
}

// warn records an advisory.
func (d *doctorReport) warn(format string, args ...any) {
	fmt.Fprintf(d.stdout, "warn "+format+"\n", args...)
}

// fail records a hard failure.
func (d *doctorReport) fail(format string, args ...any) {
	d.fails++
	fmt.Fprintf(d.stdout, "FAIL "+format+"\n", args...)
}

// runDoctor diagnoses the toolchain environment: go, the module's pinned
// tools, pandoc, and (advisory) OSV database reachability.
func runDoctor(args []string, r execx.Runner, stdout io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(stdout, "go-canon: parse flags: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "go-canon %s\n\n", tools.CanonVersion)
	d := &doctorReport{stdout: stdout}

	if root, err := module.Root(); err == nil {
		fmt.Fprintf(stdout, "module %s\n\n", root)
	} else {
		d.fail("no go.mod found — run inside a Go module")
		return 1
	}

	doctorGo(d, r)
	doctorTools(d, r)
	doctorPandoc(d, r)

	if osvCheck() {
		d.ok("OSV database reachable (%s) — govulncheck can run", osvEndpoint)
	} else {
		d.warn("OSV database not reachable — govulncheck will fail until network or cache allows it")
	}
	if d.fails > 0 {
		fmt.Fprintf(stdout, "\n%d check(s) failed\n", d.fails)
		return 1
	}
	return 0
}

// doctorGo checks the go toolchain version and GOTOOLCHAIN mode.
func doctorGo(d *doctorReport, r execx.Runner) {
	out, err := r.Output("go", "version")
	if err != nil {
		d.fail("go not runnable: %v", err)
		return
	}
	version := goVersion(string(out))
	if version == "" {
		d.fail("could not parse go version from %q", strings.TrimSpace(string(out)))
		return
	}
	if atLeast(version, tools.GoVersion) {
		d.ok("go %s (needs >= %s)", version, tools.GoVersion)
	} else {
		d.warn("go %s is older than the validated %s (GOTOOLCHAIN=auto upgrades it from go.mod)", version, tools.GoVersion)
	}
	if toolchain, err := r.Output("go", "env", "GOTOOLCHAIN"); err == nil {
		d.ok("GOTOOLCHAIN=%s", strings.TrimSpace(string(toolchain)))
	} else {
		d.warn("could not read GOTOOLCHAIN: %v", err)
	}
}

// doctorTools checks that each pinned tool resolves and is inside the
// known-good version window for this go-canon release. Task is advisory.
func doctorTools(d *doctorReport, r execx.Runner) {
	for _, t := range tools.GoTools {
		doctorOneTool(d, r, t, true)
	}
	doctorOneTool(d, r, tools.TaskTool, false)
}

// doctorOneTool checks one tool: resolution is hard for go-canon-driven
// tools, advisory for task.
func doctorOneTool(d *doctorReport, r execx.Runner, t tools.Tool, required bool) {
	if _, err := r.Resolve(t.Name); err != nil {
		if required {
			d.fail("%s: %v", t.Name, err)
		} else {
			d.warn("%s not resolvable (optional): %v", t.Name, err)
		}
		return
	}
	pinned, err := r.Output("go", "list", "-m", "-f", "{{.Version}}", t.Module)
	if err != nil {
		if required {
			d.fail("%s pinned version unknown: %v", t.Name, err)
		} else {
			d.warn("%s pinned version unknown: %v", t.Name, err)
		}
		return
	}
	v := strings.TrimSpace(string(pinned))
	if v == t.Version {
		d.ok("%s %s (known-good)", t.Name, v)
	} else {
		d.warn("%s %s is outside the known-good window (validated %s)", t.Name, v, t.Version)
	}
}

// doctorPandoc checks pandoc presence and minimum version.
func doctorPandoc(d *doctorReport, r execx.Runner) {
	if _, err := r.Resolve("pandoc"); err != nil {
		d.fail("pandoc not on PATH — the markdown gate needs pandoc >= %s (https://pandoc.org/installing.html): %v", tools.PandocMin, err)
		return
	}
	out, err := r.Output("pandoc", "--version")
	if err != nil {
		d.fail("pandoc --version failed: %v", err)
		return
	}
	version := firstVersionField(string(out))
	if version == "" {
		d.warn("could not parse pandoc version from %q", strings.TrimSpace(string(out)))
		return
	}
	if atLeast(version, tools.PandocMin) {
		d.ok("pandoc %s (needs >= %s)", version, tools.PandocMin)
	} else {
		d.fail("pandoc %s is older than the required %s", version, tools.PandocMin)
	}
}

// goVersion extracts "1.27.1" from `go version go1.27.1 linux/amd64`.
func goVersion(out string) string {
	fields := strings.Fields(out)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "go1.") {
		return ""
	}
	return strings.TrimPrefix(fields[2], "go")
}

// firstVersionField extracts "3.11" from `pandoc 3.11.1 ...`.
func firstVersionField(out string) string {
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return ""
	}
	return fields[1]
}

// atLeast reports whether the dotted numeric version v is >= min; missing
// fields count as zero.
func atLeast(v, min string) bool {
	vf := strings.Split(v, ".")
	mf := strings.Split(min, ".")
	for i := range max(len(vf), len(mf)) {
		var vn, mn int
		if i < len(vf) {
			vn, _ = strconv.Atoi(vf[i])
		}
		if i < len(mf) {
			mn, _ = strconv.Atoi(mf[i])
		}
		if vn != mn {
			return vn > mn
		}
	}
	return true
}
