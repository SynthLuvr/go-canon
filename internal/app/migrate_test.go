package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SynthLuvr/gocanon/internal/testrunner"
	"github.com/SynthLuvr/gocanon/internal/tools"
)

const oldGoMod = "module nova\n\ngo 1.22\n"

// withRelease stamps a fake released go-canon version so migrate plans
// its self-pin; test binaries otherwise resolve as development builds.
func withRelease(t *testing.T, version string) {
	t.Helper()
	orig := tools.CanonVersion
	tools.CanonVersion = version
	t.Cleanup(func() { tools.CanonVersion = orig })
}

func TestMigrateDryRunPlansEverything(t *testing.T) {
	withRelease(t, "v0.9.9")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(oldGoMod), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run([]string{"migrate", "--dry-run", "--dir", root}, fake, &out, &errOut); code != 0 {
		t.Fatalf("migrate --dry-run exit = %d, output:\n%s%s", code, out.String(), errOut.String())
	}
	got := out.String()
	for _, want := range []string{
		"[dry-run] go mod edit -go=" + tools.GoFloor + " (go-canon's pinned toolchain needs go >= " + tools.GoFloor + ")",
		"[dry-run] go get -tool " + tools.CanonPkg + "@v0.9.9",
		"[dry-run] go get -tool " + tools.GoTools[0].Pkg + "@" + tools.GoTools[0].Version,
		"[dry-run] go get -tool " + tools.GoTools[1].Pkg + "@" + tools.GoTools[1].Version,
		"[dry-run] go get -tool " + tools.GoTools[2].Pkg + "@" + tools.GoTools[2].Version,
		"[dry-run] go get -tool " + tools.TaskTool.Pkg + "@" + tools.TaskTool.Version,
		"[dry-run] write Taskfile.yml",
		"[dry-run] write go-canon.toml",
		"[dry-run] write .go-version",
		"[dry-run] append .go-canon/, cover.out to .gitignore",
		"[dry-run] append * text=auto eol=lf to .gitattributes",
		"[dry-run] go mod tidy",
	} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("dry-run plan missing %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "Taskfile.yml")); !os.IsNotExist(err) {
		t.Error("dry-run wrote Taskfile.yml")
	}
}

func TestMigrateDryRunSkipsExistingPins(t *testing.T) {
	root := t.TempDir()
	fresh := "module nova\n\ngo 1.27\n\ntool (\n\t" +
		strings.Join([]string{
			tools.CanonPkg,
			tools.GoTools[0].Pkg,
			tools.GoTools[1].Pkg,
			tools.GoTools[2].Pkg,
			tools.TaskTool.Pkg,
		}, "\n\t") +
		"\n)\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(fresh), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run([]string{"migrate", "--dry-run", "--dir", root}, fake, &out, &errOut); code != 0 {
		t.Fatalf("migrate exit = %d", code)
	}
	if strings.Contains(out.String(), "go get -tool") {
		t.Errorf("plan re-pins already-pinned tools:\n%s", out.String())
	}
	if strings.Contains(out.String(), "warning: development build") {
		t.Errorf("plan warns about a dev build even though go-canon is pinned:\n%s", out.String())
	}
	if strings.Contains(out.String(), "go mod edit -go") {
		t.Errorf("plan bumps an already-new-enough go directive:\n%s", out.String())
	}
}

func TestMigrateApplies(t *testing.T) {
	withRelease(t, "v0.9.9")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module nova\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("nova\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run([]string{"migrate", "--dir", root}, fake, &out, &errOut); code != 0 {
		t.Fatalf("migrate exit = %d, stderr:\n%s", code, errOut.String())
	}
	for _, name := range []string{"Taskfile.yml", "go-canon.toml", ".go-version", ".gitattributes"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s not written: %v", name, err)
		}
	}
	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore")) // #nosec G304 -- file written by the test
	if err != nil {
		t.Fatal(err)
	}
	text := string(gitignore)
	for _, want := range []string{"nova\n", ".go-canon/\n", "cover.out\n"} {
		if !strings.Contains(text, want) {
			t.Errorf(".gitignore missing %q:\n%s", want, text)
		}
	}
	taskfile, err := os.ReadFile(filepath.Join(root, "Taskfile.yml")) // #nosec G304 -- file written by the test
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(taskfile), "go tool go-canon lint") {
		t.Errorf("Taskfile.yml missing the go-canon lint task:\n%s", taskfile)
	}
	var getPins, edits []string
	for _, cmd := range fake.Commands {
		switch {
		case strings.HasPrefix(cmd, "go get -tool "):
			getPins = append(getPins, cmd)
		case strings.HasPrefix(cmd, "go mod "):
			edits = append(edits, cmd)
		}
	}
	if len(getPins) != 5 {
		t.Errorf("go get -tool ran %d times, want 5: %v", len(getPins), getPins)
	}
	if len(edits) != 1 || edits[0] != "go mod tidy" {
		t.Errorf("go module edits = %v, want only the final tidy", edits)
	}
}

// TestMigrateDevBuildSkipsSelfPin covers development builds (no
// resolved release): migrate pins the other tools, warns, and skips
// the go-canon self-pin instead of inventing a version.
func TestMigrateDevBuildSkipsSelfPin(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module nova\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &testrunner.Fake{}
	var dryOut, errOut strings.Builder
	if code := Run([]string{"migrate", "--dry-run", "--dir", root}, fake, &dryOut, &errOut); code != 0 {
		t.Fatalf("migrate --dry-run exit = %d, output:\n%s%s", code, dryOut.String(), errOut.String())
	}
	if !strings.Contains(dryOut.String(), "warning: development build") {
		t.Errorf("dry-run missing the development-build warning:\n%s", dryOut.String())
	}
	if strings.Contains(dryOut.String(), "[dry-run] go get -tool "+tools.CanonPkg+"@") {
		t.Errorf("dry-run pins a go-canon version from a dev build:\n%s", dryOut.String())
	}
	var out strings.Builder
	if code := Run([]string{"migrate", "--dir", root}, fake, &out, &errOut); code != 0 {
		t.Fatalf("migrate exit = %d, stderr:\n%s", code, errOut.String())
	}
	var getPins int
	for _, cmd := range fake.Commands {
		if strings.HasPrefix(cmd, "go get -tool ") {
			getPins++
		}
	}
	if getPins != 4 {
		t.Errorf("go get -tool ran %d times, want 4 without the self-pin", getPins)
	}
}

func TestMigrateIdempotentOnFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module nova\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run([]string{"migrate", "--dir", root}, fake, &out, &errOut); code != 0 {
		t.Fatalf("first migrate exit = %d", code)
	}
	first, err := os.ReadFile(filepath.Join(root, ".gitignore")) // #nosec G304 -- file written by the test
	if err != nil {
		t.Fatal(err)
	}
	var out2, errOut2 strings.Builder
	if code := Run([]string{"migrate", "--dir", root}, fake, &out2, &errOut2); code != 0 {
		t.Fatalf("second migrate exit = %d", code)
	}
	second, err := os.ReadFile(filepath.Join(root, ".gitignore")) // #nosec G304 -- file written by the test
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("second migrate rewrote .gitignore:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestMigrateNeedsGoMod(t *testing.T) {
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run([]string{"migrate", "--dry-run", "--dir", t.TempDir()}, fake, &out, &errOut); code != 1 {
		t.Fatalf("migrate exit = %d, want 1 without go.mod", code)
	}
	if !strings.Contains(errOut.String(), "no go.mod") {
		t.Errorf("stderr = %q, want no-go.mod message", errOut.String())
	}
}

func TestMigrateBadFlag(t *testing.T) {
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run([]string{"migrate", "--nope"}, fake, &out, &errOut); code != 2 {
		t.Errorf("migrate --nope exit = %d, want 2", code)
	}
}
