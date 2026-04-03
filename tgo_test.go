package main

import (
	"context"
	"testing"
)

func TestRun_Pass(t *testing.T) {
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := run(context.Background(), flags, []string{"./testdata/pass"})
	if err != nil {
		t.Errorf("expected no error for passing test, got %v", err)
	}
}

func TestRun_Fail(t *testing.T) {
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := run(context.Background(), flags, []string{"./testdata/fail"})
	if err == nil {
		t.Errorf("expected error for failing test, got nil")
	}
}

func TestRun_BuildFail(t *testing.T) {
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone, StatusBuildFail},
		Summary: Statuses{StatusFail, StatusNone, StatusBuildFail},
	}
	err := run(context.Background(), flags, []string{"./testdata/buildfail"})
	if err == nil {
		t.Errorf("expected error for build failing test, got nil")
	}
}

func TestRun_Skip(t *testing.T) {
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := run(context.Background(), flags, []string{"./testdata/skip"})
	if err != nil {
		t.Errorf("expected no error for skipping test, got %v", err)
	}
}

func TestRun_Bench(t *testing.T) {
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusBench},
		Summary: Statuses{StatusBench},
	}
	err := run(context.Background(), flags, []string{"-bench", ".", "./testdata/bench"})
	if err != nil {
		t.Errorf("expected no error for benchmark, got %v", err)
	}
}

func TestRun_Output(t *testing.T) {
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusPass},
		Summary: Statuses{StatusPass},
		V:       V4,
	}
	err := run(context.Background(), flags, []string{"./testdata/output"})
	if err != nil {
		t.Errorf("expected no error for test with output, got %v", err)
	}
}

func TestRun_Crash(t *testing.T) {
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := run(context.Background(), flags, []string{"./testdata/crash"})
	if err == nil {
		t.Errorf("expected error for crashed test, got nil")
	}
}
