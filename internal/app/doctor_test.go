package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/SynthLuvr/go-canon/internal/testrunner"
	"github.com/SynthLuvr/go-canon/internal/tools"
)

// doctorFake answers the doctor probes with a healthy environment.
func doctorFake() *testrunner.Fake {
	versions := map[string]string{
		tools.GoTools[0].Module: tools.GoTools[0].Version,
		tools.GoTools[1].Module: tools.GoTools[1].Version,
		tools.GoTools[2].Module: tools.GoTools[2].Version,
		tools.TaskTool.Module:   tools.TaskTool.Version,
	}
	return &testrunner.Fake{
		OutputFn: func(name string, args []string) ([]byte, error) {
			switch {
			case name == "go" && args[0] == "version":
				return []byte("go version go1.27.1 linux/amd64\n"), nil
			case name == "go" && args[0] == "env":
				return []byte("auto\n"), nil
			case name == "go" && args[0] == "list":
				return []byte(versions[args[len(args)-1]] + "\n"), nil
			case name == "pandoc":
				return []byte("pandoc 3.11.1\n"), nil
			}
			return nil, nil
		},
	}
}

func withStubOSV(t *testing.T, reachable bool) {
	t.Helper()
	orig := osvCheck
	osvCheck = func() bool { return reachable }
	t.Cleanup(func() { osvCheck = orig })
}

func TestDoctorHealthy(t *testing.T) {
	writeModule(t, map[string]string{"go.mod": scratchGoMod})
	withStubOSV(t, true)
	fake := doctorFake()
	var out, errOut strings.Builder
	if code := Run([]string{"doctor"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("doctor exit = %d, output:\n%s%s", code, out.String(), errOut.String())
	}
	for _, want := range []string{
		"ok   go 1.27.1",
		"ok   golangci-lint " + tools.GoTools[0].Version,
		"ok   govulncheck",
		"ok   modernize",
		"ok   task " + tools.TaskTool.Version,
		"ok   pandoc 3.11.1",
		"OSV database reachable",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("doctor output missing %q:\n%s", want, out.String())
		}
	}
}

func TestDoctorWarnsOutsideKnownGoodWindow(t *testing.T) {
	writeModule(t, map[string]string{"go.mod": scratchGoMod})
	withStubOSV(t, true)
	fake := doctorFake()
	orig := fake.OutputFn
	fake.OutputFn = func(name string, args []string) ([]byte, error) {
		if name == "go" && args[0] == "list" && strings.Contains(args[len(args)-1], "golangci-lint") {
			return []byte("v9.9.9\n"), nil
		}
		if name == "go" && args[0] == "version" {
			return []byte("go version go1.26.0 linux/amd64\n"), nil
		}
		return orig(name, args)
	}
	var out, errOut strings.Builder
	if code := Run([]string{"doctor"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("doctor exit = %d, want 0 (drift is advisory)", code)
	}
	for _, want := range []string{
		"warn golangci-lint v9.9.9 is outside the known-good window",
		"ok   go 1.26.0 (floor >= " + tools.GoFloor,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("doctor output missing %q:\n%s", want, out.String())
		}
	}
}

func TestDoctorWarnsBelowGoFloor(t *testing.T) {
	writeModule(t, map[string]string{"go.mod": scratchGoMod})
	withStubOSV(t, true)
	fake := doctorFake()
	orig := fake.OutputFn
	fake.OutputFn = func(name string, args []string) ([]byte, error) {
		if name == "go" && args[0] == "version" {
			return []byte("go version go1.23.0 linux/amd64\n"), nil
		}
		return orig(name, args)
	}
	var out, errOut strings.Builder
	if code := Run([]string{"doctor"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("doctor exit = %d, want 0 (below the floor is advisory)", code)
	}
	if !strings.Contains(out.String(), "warn go 1.23.0 is older than the required "+tools.GoFloor) {
		t.Errorf("doctor output missing the below-floor warning:\n%s", out.String())
	}
}

func TestDoctorFailsOnMissingTool(t *testing.T) {
	writeModule(t, map[string]string{"go.mod": scratchGoMod})
	withStubOSV(t, true)
	fake := doctorFake()
	fake.ResolveFn = func(name string) (string, error) {
		if name == "modernize" {
			return "", errors.New("not in go.mod tool block")
		}
		return "/fake/bin/" + name, nil
	}
	var out, errOut strings.Builder
	if code := Run([]string{"doctor"}, fake, &out, &errOut); code != 1 {
		t.Fatalf("doctor exit = %d, want 1 (required tool missing)", code)
	}
	if !strings.Contains(out.String(), "FAIL modernize") {
		t.Errorf("doctor output missing modernize failure:\n%s", out.String())
	}
}

func TestDoctorFailsOnOldPandoc(t *testing.T) {
	writeModule(t, map[string]string{"go.mod": scratchGoMod})
	withStubOSV(t, true)
	fake := doctorFake()
	orig := fake.OutputFn
	fake.OutputFn = func(name string, args []string) ([]byte, error) {
		if name == "pandoc" {
			return []byte("pandoc 2.19.2\n"), nil
		}
		return orig(name, args)
	}
	var out, errOut strings.Builder
	if code := Run([]string{"doctor"}, fake, &out, &errOut); code != 1 {
		t.Fatalf("doctor exit = %d, want 1 (pandoc too old)", code)
	}
	if !strings.Contains(out.String(), "FAIL pandoc 2.19.2") {
		t.Errorf("doctor output missing pandoc failure:\n%s", out.String())
	}
}

func TestDoctorWarnsWhenOSVUnreachable(t *testing.T) {
	writeModule(t, map[string]string{"go.mod": scratchGoMod})
	withStubOSV(t, false)
	fake := doctorFake()
	var out, errOut strings.Builder
	if code := Run([]string{"doctor"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("doctor exit = %d, want 0 (OSV is advisory)", code)
	}
	if !strings.Contains(out.String(), "warn OSV database not reachable") {
		t.Errorf("doctor output missing OSV advisory:\n%s", out.String())
	}
}

func TestDoctorOutsideModule(t *testing.T) {
	withStubOSV(t, true)
	t.Chdir(t.TempDir())
	fake := doctorFake()
	var out, errOut strings.Builder
	if code := Run([]string{"doctor"}, fake, &out, &errOut); code != 1 {
		t.Fatalf("doctor exit = %d, want 1 outside a module", code)
	}
	if !strings.Contains(out.String(), "no go.mod") {
		t.Errorf("doctor output missing go.mod failure:\n%s", out.String())
	}
}

func TestAtLeast(t *testing.T) {
	tests := []struct {
		v, min string
		want   bool
	}{
		{v: "1.27.1", min: "1.27", want: true},
		{v: "1.27", min: "1.27", want: true},
		{v: "1.26.0", min: "1.27", want: false},
		{v: "3.10.2", min: "3.10", want: true},
		{v: "3.9.9", min: "3.10", want: false},
		{v: "2", min: "1.9.9", want: true},
	}
	for _, tt := range tests {
		if got := atLeast(tt.v, tt.min); got != tt.want {
			t.Errorf("atLeast(%q, %q) = %v, want %v", tt.v, tt.min, got, tt.want)
		}
	}
}

func TestGoVersionParse(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "go version go1.27.1 linux/amd64\n", want: "1.27.1"},
		{in: "go version devel go1.28-abc123 linux/amd64\n", want: ""},
		{in: "garbage\n", want: ""},
	}
	for _, tt := range tests {
		if got := goVersion(tt.in); got != tt.want {
			t.Errorf("goVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
