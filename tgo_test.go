package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/color"
)

// TestMain disables color so the expected output is the same whether or
// not the test binary writes to a terminal, as it does under a bare go test.
func TestMain(m *testing.M) {
	color.NoColor = true
	os.Exit(m.Run())
}

func TestRun_Pass(t *testing.T) {
	t.Parallel()
	dir := setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./pass"})
	if err != nil {
		t.Errorf("expected no error for passing test, got %v", err)
	}
}

func TestRun_Fail(t *testing.T) {
	t.Parallel()
	dir := setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./fail"})
	if err == nil {
		t.Errorf("expected error for failing test, got nil")
	}
}

func TestRun_BuildFail(t *testing.T) {
	t.Parallel()
	dir := setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		Results: Statuses{StatusFail, StatusNone, StatusBuildFail},
		Summary: Statuses{StatusFail, StatusNone, StatusBuildFail},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./buildfail"})
	if err == nil {
		t.Errorf("expected error for build failing test, got nil")
	}
}

func TestRun_Skip(t *testing.T) {
	t.Parallel()
	dir := setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./skip"})
	if err != nil {
		t.Errorf("expected no error for skipping test, got %v", err)
	}
}

func TestRun_Bench(t *testing.T) {
	t.Parallel()
	dir := setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		Results: Statuses{StatusBench},
		Summary: Statuses{StatusBench},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"-bench", ".", "./bench"})
	if err != nil {
		t.Errorf("expected no error for benchmark, got %v", err)
	}
}

func TestRun_Output(t *testing.T) {
	t.Parallel()
	dir := setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		Results: Statuses{StatusPass},
		Summary: Statuses{StatusPass},
		V:       V4,
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./output"})
	if err != nil {
		t.Errorf("expected no error for test with output, got %v", err)
	}
}

func TestRun_Crash(t *testing.T) {
	t.Parallel()
	dir := setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./crash"})
	if err == nil {
		t.Errorf("expected error for crashed test, got nil")
	}
}

func TestRun_Mixed_With_V(t *testing.T) {
	t.Parallel()
	dir := setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		V:       V2,
		Results: Statuses{StatusBench, StatusPass, StatusNone, StatusFail},
		Summary: Statuses{StatusNone, StatusFail},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"-v", "./mixed"})
	if err == nil {
		t.Errorf("expected error for mixed failing test, got nil")
	}
}

func TestFindModules(t *testing.T) {
	t.Parallel()
	dir := setupMultiModuleTestDir(t)

	modules, err := findModules(dir)
	if err != nil {
		t.Fatalf("findModules failed: %v", err)
	}

	expected := []string{
		filepath.Join(dir, "mod_a"),
		filepath.Join(dir, "mod_b"),
		filepath.Join(dir, "mod_c"),
	}

	if len(modules) != len(expected) {
		t.Fatalf("expected %d modules, got %d: %v", len(expected), len(modules), modules)
	}
	for i, exp := range expected {
		if modules[i] != exp {
			t.Errorf("modules[%d] = %q, want %q", i, modules[i], exp)
		}
	}
}

func TestCleanRelDir(t *testing.T) {
	t.Parallel()
	if got := cleanRelDir("."); got != "." {
		t.Errorf("cleanRelDir(.) = %q, want .", got)
	}
	if got := cleanRelDir("mod_a"); got != "./mod_a" {
		t.Errorf("cleanRelDir(mod_a) = %q, want ./mod_a", got)
	}
	if got := cleanRelDir("./mod_a"); got != "./mod_a" {
		t.Errorf("cleanRelDir(./mod_a) = %q, want ./mod_a", got)
	}
}

func TestRun_Recursive_WithFailures(t *testing.T) {
	t.Parallel()
	dir := setupMultiModuleTestDir(t)

	var sb strings.Builder
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		R:       ".",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}

	err := runWithRenderer(context.Background(), flags, NewRenderer(&sb), nil)
	if err == nil {
		t.Errorf("expected error for multi-module run with failure, got nil")
	}

	out := sb.String()
	if !strings.Contains(out, "[./mod_b]") {
		t.Errorf("expected output to contain failure from mod_b [./mod_b], got: %s", out)
	}
	if !strings.Contains(out, "FAILURES (1 of 3 modules failed)") {
		t.Errorf("expected output to contain 1 of 3 modules failed header, got: %s", out)
	}
	if !strings.Contains(out, "modules:2/3") {
		t.Errorf("expected output to contain modules:2/3 summary, got: %s", out)
	}
}

func TestRun_Recursive_FailFast(t *testing.T) {
	t.Parallel()
	dir := setupMultiModuleTestDir(t)

	var sb strings.Builder
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		R:       ".",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}

	// mod_a passes, mod_b fails, mod_c should be skipped due to -failfast
	err := runWithRenderer(context.Background(), flags, NewRenderer(&sb), []string{"-failfast"})
	if err == nil {
		t.Errorf("expected error for multi-module run with -failfast, got nil")
	}

	out := sb.String()
	if strings.Contains(out, "---> [./mod_c]") {
		t.Errorf("expected mod_c to be skipped with -failfast, but header was found in output: %s", out)
	}
	if !strings.Contains(out, "1-skipped") {
		t.Errorf("expected output to contain 1-skipped indication in footer, got: %s", out)
	}
}

func TestRun_Recursive_NoModules(t *testing.T) {
	t.Parallel()
	emptyDir := t.TempDir()

	flags := Flags{
		Bin: "go",
		Dir: emptyDir,
		R:   ".",
	}

	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), nil)
	if err == nil {
		t.Errorf("expected error when no go.mod found, got nil")
	}
}

func TestRun_Recursive_Coverage(t *testing.T) {
	t.Parallel()
	dir := setupMultiModuleTestDir(t)

	var sb strings.Builder
	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		R:       ".",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}

	_ = runWithRenderer(context.Background(), flags, NewRenderer(&sb), []string{"-cover"})

	out := sb.String()
	if !strings.Contains(out, "COVR") {
		t.Errorf("expected output to contain COVR header, got: %s", out)
	}
	if !strings.Contains(out, "100.0% [./mod_a] moda") {
		t.Errorf("expected output to contain coverage for mod_a '100.0%% [./mod_a] moda', got: %s", out)
	}
}
