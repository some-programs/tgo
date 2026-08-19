package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func normalizeOutput(s string) string {
	// Normalize timestamps in format HH:MM:SS or HH:MM:SS.mmm (with 1 to 3 subsecond digits)
	reSubSecTime := regexp.MustCompile(`\b\d{2}:\d{2}:\d{2}\.\d{1,3}\b`)
	s = reSubSecTime.ReplaceAllString(s, "12:00:00.000")

	reTime := regexp.MustCompile(`\b\d{2}:\d{2}:\d{2}\b`)
	s = reTime.ReplaceAllString(s, "12:00:00")

	// Normalize durations in footer (e.g. "| 95ms  ══════" or "| 1.02s  ══════")
	reFooterDuration := regexp.MustCompile(`\|\s+\d+(?:\.\d+)?[mµn]?s\s+══════`)
	s = reFooterDuration.ReplaceAllString(s, "| 0ms  ══════")

	// Normalize test elapsed time badges like "  (0.01s)" or " (0.10s)"
	reElapsed := regexp.MustCompile(`\s+\(\d+\.\d{2}s\)`)
	s = reElapsed.ReplaceAllString(s, "")

	// Normalize package-level test durations like "ok  \ttestcases/output\t0.001s"
	rePkgDuration := regexp.MustCompile(`(ok\s+\S+\s+)\d+(?:\.\d+)?[mµn]?s`)
	s = rePkgDuration.ReplaceAllString(s, "${1}0.000s")

	// Normalize whitespace-only blank lines
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\r")
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func TestFullProgram(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		args       []string
		flags      Flags
		wantErr    bool
		wantOutput string
	}{
		{
			name: "Pass",
			args: []string{"./pass"},
			flags: Flags{
				Bin:     "go",
				Results: Statuses{StatusPass},
				Summary: Statuses{StatusPass},
			},
			wantErr: false,
			wantOutput: `
*****
=== PASS testcases/pass.TestPass
=== PASS testcases/pass
════════════ PASS ════════════
  PASS testcases/pass.TestPass
  PASS testcases/pass

══════ 12:00:00 | PASS:1 | FAIL:0 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name: "Fail",
			args: []string{"./fail"},
			flags: Flags{
				Bin:     "go",
				Results: Statuses{StatusFail, StatusNone},
				Summary: Statuses{StatusFail, StatusNone},
			},
			wantErr: true,
			wantOutput: `
*****
=== FAIL testcases/fail.TestFail
=== FAIL testcases/fail
════════════ FAIL ════════════
  FAIL testcases/fail.TestFail
  FAIL testcases/fail

══════ 12:00:00 | PASS:0 | FAIL:1 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name: "Skip",
			args: []string{"./skip"},
			flags: Flags{
				Bin:     "go",
				Results: Statuses{StatusSkip},
				Summary: Statuses{StatusSkip},
			},
			wantErr: false,
			wantOutput: `
*****
=== SKIP testcases/skip.TestSkip

    skip_test.go:6: skipping test

════════════ SKIP ════════════
  SKIP testcases/skip.TestSkip

══════ 12:00:00 | PASS:0 | FAIL:0 | BUILD FAIL:0 | NONE:0 | SKIP:1 | 0ms  ══════
`,
		},
		{
			name: "ErrTypes",
			args: []string{"./errtypes"},
			flags: Flags{
				Bin:     "go",
				Results: Statuses{StatusFail, StatusNone},
				Summary: Statuses{StatusFail, StatusNone},
			},
			wantErr: true,
			wantOutput: `
*****
=== FAIL testcases/errtypes.TestErrors

    errtypes_test.go:6: first error line
        second error line

=== FAIL testcases/errtypes
════════════ FAIL ════════════
  FAIL testcases/errtypes.TestErrors
  FAIL testcases/errtypes

══════ 12:00:00 | PASS:0 | FAIL:1 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name: "Coverage",
			args: []string{"-cover", "./cov"},
			flags: Flags{
				Bin:     "go",
				Results: Statuses{StatusPass},
				Summary: Statuses{StatusPass},
			},
			wantErr: false,
			wantOutput: `
*****
=== PASS testcases/cov.TestAdd
=== PASS testcases/cov  {100.0%}
════════════ PASS ════════════
  PASS testcases/cov.TestAdd
  PASS testcases/cov  {100.0%}
════════════ COVR ════════════
100.0% testcases/cov

══════ 12:00:00 | PASS:1 | FAIL:0 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name: "Crash",
			args: []string{"./crash"},
			flags: Flags{
				Bin:     "go",
				Results: Statuses{StatusFail, StatusNone},
				Summary: Statuses{StatusFail, StatusNone},
			},
			wantErr: true,
			wantOutput: `
*****
=== FAIL testcases/crash
════════════ FAIL ════════════
  FAIL testcases/crash
════════════ NONE ════════════
  NONE testcases/crash.TestCrash

══════ 12:00:00 | PASS:0 | FAIL:0 | BUILD FAIL:0 | NONE:1 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name: "BuildFail",
			args: []string{"./buildfail"},
			flags: Flags{
				Bin:     "go",
				Results: Statuses{StatusFail, StatusNone, StatusBuildFail},
				Summary: Statuses{StatusFail, StatusNone, StatusBuildFail},
			},
			wantErr: true,
			wantOutput: `
*****
=== BUILD FAIL testcases/buildfail [testcases/buildfail.test]

# testcases/buildfail [testcases/buildfail.test]
buildfail/buildfail_test.go:6:2: undefined: undefined

=== BUILD FAIL testcases/buildfail
════════════ BUILD FAIL ════════════
BUILD FAIL testcases/buildfail
BUILD FAIL testcases/buildfail [testcases/buildfail.test]

══════ 12:00:00 | PASS:0 | FAIL:0 | BUILD FAIL:2 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name: "Verbose_V4",
			args: []string{"./output"},
			flags: Flags{
				Bin:     "go",
				Results: Statuses{StatusPass},
				Summary: Statuses{StatusPass},
				V:       V4,
			},
			wantErr: false,
			wantOutput: `
*****
=== PASS testcases/output.TestOutput

    run 12:00:00.000
 output 12:00:00.000 === RUN   TestOutput
 output 12:00:00.000 This is some test output.
 output 12:00:00.000     output_test.go:10: This is a test log message.
 output 12:00:00.000 --- PASS: TestOutput
   pass 12:00:00.000

=== PASS testcases/output

  start 12:00:00.000
 output 12:00:00.000 PASS
 output 12:00:00.000 ok  	testcases/output	0.000s
   pass 12:00:00.000

════════════ PASS ════════════
  PASS testcases/output.TestOutput
  PASS testcases/output

══════ 12:00:00 | PASS:1 | FAIL:0 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name: "Mixed_With_V",
			args: []string{"-v", "./mixed"},
			flags: Flags{
				Bin:     "go",
				V:       V2,
				Results: Statuses{StatusBench, StatusPass, StatusNone, StatusFail},
				Summary: Statuses{StatusNone, StatusFail},
			},
			wantErr: true,
			wantOutput: `
*****
=== FAIL testcases/mixed.TestMixed

    mixed_test.go:6: this is a log line
    mixed_test.go:7: first error line
        second error line

=== FAIL testcases/mixed
════════════ FAIL ════════════
  FAIL testcases/mixed.TestMixed
  FAIL testcases/mixed

══════ 12:00:00 | PASS:0 | FAIL:1 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := setupTestDir(t)
			flags := tt.flags
			flags.Dir = dir
			var sb strings.Builder
			renderer := NewRenderer(&sb)

			err := runWithRenderer(context.Background(), flags, renderer, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("runWithRenderer() error = %v, wantErr = %v", err, tt.wantErr)
			}

			got := normalizeOutput(sb.String())
			want := strings.TrimSpace(tt.wantOutput)
			if got != want {
				t.Errorf("Full program output mismatch:\n--- GOT ---\n%s\n--- WANT ---\n%s", got, want)
			}
		})
	}
}

func TestFullProgram_CLI(t *testing.T) {
	t.Parallel()
	rootDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	tempBuildDir := t.TempDir()
	binPath := filepath.Join(tempBuildDir, "tgo_bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = rootDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build tgo: %v\nOutput: %s", err, string(out))
	}

	tests := []struct {
		name       string
		args       []string
		env        []string
		wantErr    bool
		wantOutput string
	}{
		{
			name:    "Pass",
			args:    []string{"./pass"},
			env:     []string{"TGO_RESULTS=pass", "TGO_SUMMARY=pass"},
			wantErr: false,
			wantOutput: `
*****
=== PASS testcases/pass.TestPass
=== PASS testcases/pass
════════════ PASS ════════════
  PASS testcases/pass.TestPass
  PASS testcases/pass

══════ 12:00:00 | PASS:1 | FAIL:0 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name:    "Fail",
			args:    []string{"./fail"},
			wantErr: true,
			wantOutput: `
*****
=== FAIL testcases/fail.TestFail
=== FAIL testcases/fail
════════════ FAIL ════════════
  FAIL testcases/fail.TestFail
  FAIL testcases/fail

══════ 12:00:00 | PASS:0 | FAIL:1 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name:    "Skip",
			args:    []string{"./skip"},
			env:     []string{"TGO_RESULTS=skip", "TGO_SUMMARY=skip"},
			wantErr: false,
			wantOutput: `
*****
=== SKIP testcases/skip.TestSkip

    skip_test.go:6: skipping test

════════════ SKIP ════════════
  SKIP testcases/skip.TestSkip

══════ 12:00:00 | PASS:0 | FAIL:0 | BUILD FAIL:0 | NONE:0 | SKIP:1 | 0ms  ══════
`,
		},
		{
			name:    "Coverage",
			args:    []string{"-cover", "./cov"},
			env:     []string{"TGO_RESULTS=pass", "TGO_SUMMARY=pass"},
			wantErr: false,
			wantOutput: `
*****
=== PASS testcases/cov.TestAdd
=== PASS testcases/cov  {100.0%}
════════════ PASS ════════════
  PASS testcases/cov.TestAdd
  PASS testcases/cov  {100.0%}
════════════ COVR ════════════
100.0% testcases/cov

══════ 12:00:00 | PASS:1 | FAIL:0 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
		{
			name:    "Mixed_With_V",
			args:    []string{"-v", "./mixed"},
			wantErr: true,
			wantOutput: `
*****
=== FAIL testcases/mixed.TestMixed

    mixed_test.go:6: this is a log line
    mixed_test.go:7: first error line
        second error line

=== FAIL testcases/mixed
════════════ FAIL ════════════
  FAIL testcases/mixed.TestMixed
  FAIL testcases/mixed

══════ 12:00:00 | PASS:0 | FAIL:1 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := setupTestDir(t)

			cmd := exec.Command(binPath, tt.args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "NO_COLOR=1")
			cmd.Env = append(cmd.Env, tt.env...)

			out, err := cmd.CombinedOutput()
			if (err != nil) != tt.wantErr {
				t.Fatalf("CLI error = %v, wantErr = %v, Output: %s", err, tt.wantErr, string(out))
			}

			got := normalizeOutput(string(out))
			want := strings.TrimSpace(tt.wantOutput)
			if got != want {
				t.Errorf("CLI full program output mismatch:\n--- GOT ---\n%s\n--- WANT ---\n%s", got, want)
			}
		})
	}
}
