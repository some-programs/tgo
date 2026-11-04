package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ExitError represents a process exit status.
type ExitError int

func (e ExitError) Error() string {
	return strconv.FormatInt(int64(e), 10)
}

func run(ctx context.Context, flags Flags, argv []string) error {
	return runWithRenderer(ctx, flags, defaultRenderer, argv)
}

// exitInterrupted is the exit status used when tgo is interrupted, as for a
// process killed by SIGINT.
const exitInterrupted = ExitError(130)

func runWithRenderer(parent context.Context, flags Flags, renderer *Renderer, argv []string) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	coverEnabled := coverageRequested(argv)

	args := []string{"test", "-json"}
	for _, v := range argv {
		if v == "-v" {
			continue
		}
		args = append(args, v)
	}
	log.Println("args", args)

	cmd := exec.CommandContext(ctx, flags.Bin, args...)
	if flags.Dir != "" {
		cmd.Dir = flags.Dir
	}
	cmd.Stderr = os.Stderr
	interruptDone := setupInterrupt(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()

	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		return err
	}

	t0 := time.Now()
	if err := processEvents(stdout, flags, renderer, coverEnabled, t0); err != nil {
		log.Println("process error:", err)
	}

	cmdErr := cmd.Wait()
	interruptDone()
	if parent.Err() != nil {
		return exitInterrupted
	}
	var ee *exec.ExitError
	if cmdErr != nil && errors.As(cmdErr, &ee) {
		if ee.Exited() {
			return ExitError(ee.ExitCode())
		}
		return cmdErr
	}
	return nil
}

// coverageRequested reports whether the go test arguments enable coverage:
// -cover, or any of -covermode, -coverpkg and -coverprofile, which imply it.
// Arguments after -args are for the test binary and are ignored.
func coverageRequested(argv []string) bool {
	enabled := false
	for _, arg := range argv {
		if arg == "-args" || arg == "--args" {
			break
		}
		flag, ok := strings.CutPrefix(arg, "-")
		if !ok {
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(flag, "-"), "=")
		switch name {
		case "cover":
			enabled = !hasValue || value != "false"
		case "covermode", "coverpkg", "coverprofile":
			enabled = true
		}
	}
	return enabled
}

func processEvents(r io.Reader, flags Flags, renderer *Renderer, coverEnabled bool, t0 time.Time) error {
	if renderer == nil {
		renderer = defaultRenderer
	}
	w := renderer.out()

	tests := make(TestStorage)
	// printed holds the status each key was last printed with. A key is
	// printed again if a later run (go test -count=N) changes its status.
	printed := make(map[Key]Status)
	// runningBench holds the most recently started benchmark per package.
	runningBench := make(map[string]string)
	scanner := bufio.NewScanner(r)

	printIfChanged := func(key Key) {
		status := tests[key].Status()
		if status != StatusNone && printed[key] != status && flags.Results.Any(status) {
			renderer.PrintDetail(tests[key], flags)
			printed[key] = status
		}
	}

	renderer.PrintHeader()

	for scanner.Scan() {
		var e Event
		log.Println("LINE:", scanner.Text())
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			log.Println("scanner error", err)
			continue
		}
		tests.Append(e)
		key := e.Key()
		if e.Action == ActionRun && strings.HasPrefix(e.Test, "Benchmark") {
			runningBench[key.Package] = e.Test
		}
		if e.Status() == StatusNone {
			continue
		}
		if e.Test == "" {
			for _, k := range tests.FinishBenchmarks(e, runningBench[key.Package]) {
				printIfChanged(k)
			}
		}
		printIfChanged(key)
	}

	if len(tests) > 0 {
		// Print any non-finished tests if requested
		if flags.Results.Any(StatusNone) {
			noneTests := tests.
				Filter(func(k Key, _ Events) bool {
					_, ok := printed[k]
					return !ok
				}).
				FindByStatus(StatusNone)
			for _, key := range noneTests.OrderedKeys() {
				renderer.PrintDetail(tests[key], flags)
				printed[key] = StatusNone
			}
		}

		// Print configured summaries
		for _, status := range flags.Summary {
			filtered := tests.FindByStatus(status)
			if status == StatusSkip && flags.V <= V3 {
				filtered = filtered.FilterNotests()
			}
			if len(filtered) > 0 {
				renderer.PrintSummary(filtered, status)
			}
		}

		if coverEnabled {
			filtered := tests.WithCoverage()
			if len(filtered) > 0 {
				renderer.PrintCoverage(filtered)
			}
		}

		renderer.PrintFooter(tests, t0)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(w, "error reading standard input:", err)
		return err
	}

	return nil
}
