package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildVersionOutputAddsVPrefixForSemver(t *testing.T) {
	got := buildVersionOutput("1.2.3")
	want := fmt.Sprintf("v1.2.3 (%s, %s/%s)", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if got != want {
		t.Fatalf("buildVersionOutput() = %q, want %q", got, want)
	}
}

func TestBuildVersionOutputKeepsExistingVPrefix(t *testing.T) {
	got := buildVersionOutput("v1.2.3")
	want := fmt.Sprintf("v1.2.3 (%s, %s/%s)", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if got != want {
		t.Fatalf("buildVersionOutput() = %q, want %q", got, want)
	}
}

func TestBuildVersionOutputKeepsNonSemver(t *testing.T) {
	got := buildVersionOutput("dev")
	want := fmt.Sprintf("dev (%s, %s/%s)", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if got != want {
		t.Fatalf("buildVersionOutput() = %q, want %q", got, want)
	}
}

func TestRunReturnsUsageExitCodeForInvalidNonInteractiveArguments(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	var stdout, stderr bytes.Buffer
	code := run([]string{"--non-interactive", "--mode", "edit", "--submode", "copy", "draft.md"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("unexpected exit code: %d (%s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--approval") {
		t.Fatalf("expected helpful configuration error, got %q", stderr.String())
	}
}

func TestRunRequiresCommandLineFileForNonInteractiveMode(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	t.Setenv("GOAUTHORLLM_FILE", filepath.Join(dir, "draft.md"))

	var stdout, stderr bytes.Buffer
	code := run([]string{"--non-interactive", "--mode", "generate", "--submode", "continue"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "command") {
		t.Fatalf("expected command-line file error, code=%d stderr=%q", code, stderr.String())
	}
}
