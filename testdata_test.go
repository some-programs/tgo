package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

var testDataFS = fstest.MapFS{
	"go.mod": {
		Data: []byte("module testcases\n\ngo 1.27\n"),
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
	"cov/cov.go": {
		Data: []byte(`package cov

func Add(a, b int) int {
	return a + b
}
`),
	},
	"nocov/nocov.go": {
		Data: []byte(`package nocov

func Sub(a, b int) int {
	return a - b
}
`),
	},
	"cov/cov_test.go": {
		Data: []byte(`package cov

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fail()
	}
}
`),
	},
	"errtypes/errtypes_test.go": {
		Data: []byte(`package errtypes

import "testing"

func TestErrors(t *testing.T) {
	t.Errorf("first error line\nsecond error line")
}
`),
	},
	"flaky/flaky_test.go": {
		Data: []byte(`package flaky

import "testing"

var runs int

// TestFlaky passes on its first run and fails on its second (-count=2).
func TestFlaky(t *testing.T) {
	runs++
	if runs == 2 {
		t.Fatal("second run fails")
	}
}
`),
	},
	"failcount/failcount_test.go": {
		Data: []byte(`package failcount

import "testing"

var runs int

// TestFailEachRun fails on every run (-count=N) with a different message.
func TestFailEachRun(t *testing.T) {
	runs++
	t.Errorf("failure in run %d", runs)
}
`),
	},
	"benchfail/benchfail_test.go": {
		Data: []byte(`package benchfail

import "testing"

func BenchmarkOK(b *testing.B) {
	for b.Loop() {
	}
}

func BenchmarkZFail(b *testing.B) {
	b.Fatal("boom")
}
`),
	},
	"benchcrash/benchcrash_test.go": {
		Data: []byte(`package benchcrash

import "testing"

func BenchmarkOK(b *testing.B) {
	for b.Loop() {
	}
}

func BenchmarkZCrash(b *testing.B) {
	panic("crash")
}
`),
	},
	"xtestfail/xtestfail.go": {
		Data: []byte(`package xtestfail
`),
	},
	"xtestfail/xtestfail_test.go": {
		Data: []byte(`package xtestfail_test

import "testing"

func TestX(t *testing.T) {
	undefined()
}
`),
	},
	"interrupt/interrupt_test.go": {
		Data: []byte(`package interrupt

import (
	"os"
	"strconv"
	"testing"
	"time"
)

// TestInterrupt writes its pid to a file so the caller knows it is running,
// then waits to be interrupted.
func TestInterrupt(t *testing.T) {
	t.Log("waiting")
	if err := os.WriteFile("pid.tmp", []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename("pid.tmp", "pid"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Minute)
}
`),
	},
	"attr/attr_test.go": {
		Data: []byte(`package attr

import "testing"

func TestAttr(t *testing.T) {
	t.Attr("issue", "123")
	t.Log("hello")
}
`),
	},
	"mixed/mixed_test.go": {
		Data: []byte(`package mixed

import "testing"

func TestMixed(t *testing.T) {
	t.Log("this is a log line")
	t.Errorf("first error line\nsecond error line")
}
`),
	},
}

// setupTestDir creates a temporary directory populated with testDataFS test files.
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
	return dir
}
