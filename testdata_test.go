package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

var testDataFS = fstest.MapFS{
	"go.mod": {
		Data: []byte("module testcases\n\ngo 1.24\n"),
	},
	"bench/bench_test.go": {
		Data: []byte(`package bench

import "testing"

func BenchmarkSimple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Do nothing
	}
}
`),
	},
	"buildfail/buildfail_test.go": {
		Data: []byte(`package buildfail

import "testing"

func TestBuildFail(t *testing.T) {
	undefined()
}
`),
	},
	"crash/crash_test.go": {
		Data: []byte(`package crash

import (
	"os"
	"testing"
)

func TestCrash(t *testing.T) {
	os.Exit(1)
}
`),
	},
	"fail/fail_test.go": {
		Data: []byte(`package fail

import "testing"

func TestFail(t *testing.T) {
	t.Fail()
}
`),
	},
	"output/output_test.go": {
		Data: []byte(`package output

import (
	"fmt"
	"testing"
)

func TestOutput(t *testing.T) {
	fmt.Println("This is some test output.")
	t.Log("This is a test log message.")
}
`),
	},
	"pass/pass_test.go": {
		Data: []byte(`package pass

import "testing"

func TestPass(t *testing.T) {
	// A simple test that passes
}
`),
	},
	"skip/skip_test.go": {
		Data: []byte(`package skip

import "testing"

func TestSkip(t *testing.T) {
	t.Skip("skipping test")
}
`),
	},
}

// setupTestDir creates a temporary directory populated with testDataFS test files
// and changes the working directory to it for the duration of the test.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, file := range testDataFS {
		targetPath := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
		if err := os.WriteFile(targetPath, file.Data, 0644); err != nil {
			t.Fatalf("failed to write file %s: %v", name, err)
		}
	}
	t.Chdir(dir)
	return dir
}
