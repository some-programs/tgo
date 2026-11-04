//go:build unix

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRun_Interrupt(t *testing.T) {
	t.Parallel()
	dir := setupTestDir(t)
	pidFile := filepath.Join(dir, "interrupt", "pid")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// Interrupt once the test is running.
		for ctx.Err() == nil {
			if _, err := os.Stat(pidFile); err == nil {
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	flags := Flags{
		Bin:     "go",
		Dir:     dir,
		Results: Statuses{StatusNone},
		Summary: Statuses{StatusNone},
	}
	var sb strings.Builder
	start := time.Now()
	err := runWithRenderer(ctx, flags, NewRenderer(&sb), []string{"-count=1", "./interrupt"})
	if !errors.Is(err, exitInterrupted) {
		t.Errorf("runWithRenderer() error = %v, want %v", err, exitInterrupted)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Errorf("interrupted run took %v", elapsed)
	}

	out := normalizeOutput(sb.String())
	if !strings.Contains(out, "=== NONE testcases/interrupt.TestInterrupt") ||
		!strings.Contains(out, "interrupt_test.go:19: waiting") {
		t.Errorf("interrupted test not reported:\n%s", out)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("reading pid file: %v", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("parsing pid %q: %v", data, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("test binary (pid %d) still running after tgo returned", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
