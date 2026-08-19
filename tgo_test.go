package main

import (
	"context"
	"io"
	"testing"
)

func TestRun_Pass(t *testing.T) {
	setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./pass"})
	if err != nil {
		t.Errorf("expected no error for passing test, got %v", err)
	}
}

func TestRun_Fail(t *testing.T) {
	setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./fail"})
	if err == nil {
		t.Errorf("expected error for failing test, got nil")
	}
}

func TestRun_BuildFail(t *testing.T) {
	setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone, StatusBuildFail},
		Summary: Statuses{StatusFail, StatusNone, StatusBuildFail},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./buildfail"})
	if err == nil {
		t.Errorf("expected error for build failing test, got nil")
	}
}

func TestRun_Skip(t *testing.T) {
	setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./skip"})
	if err != nil {
		t.Errorf("expected no error for skipping test, got %v", err)
	}
}

func TestRun_Bench(t *testing.T) {
	setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusBench},
		Summary: Statuses{StatusBench},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"-bench", ".", "./bench"})
	if err != nil {
		t.Errorf("expected no error for benchmark, got %v", err)
	}
}

func TestRun_Output(t *testing.T) {
	setupTestDir(t)
	flags := Flags{
		Bin:     "go",
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
	setupTestDir(t)
	flags := Flags{
		Bin:     "go",
		Results: Statuses{StatusFail, StatusNone},
		Summary: Statuses{StatusFail, StatusNone},
	}
	err := runWithRenderer(context.Background(), flags, NewRenderer(io.Discard), []string{"./crash"})
	if err == nil {
		t.Errorf("expected error for crashed test, got nil")
	}
}
