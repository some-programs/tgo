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
	"slices"
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

	if listRequested(argv) && !flags.Results.Any(StatusPass) {
		// go test -list prints the names as output of the passing package.
		flags.Results = append(slices.Clone(flags.Results), StatusPass)
	}

	args := append([]string{"test", "-json"}, goTestArgs(argv)...)
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
		return err
	}

	t0 := time.Now()
	if err := processEvents(stdout, flags, renderer, t0); err != nil {
		log.Println("process error:", err)
	}
	// Drain what processEvents left unread, so go test can't block writing
	// to a full pipe and keep cmd.Wait from returning.
	_, _ = io.Copy(io.Discard, stdout)

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

// goTestArgs returns argv without any -v flags, since -v makes go test -json
// stop marking error output with OutputType "error". Verbose output is
// handled by tgo itself. Arguments after -args are passed through unchanged.
func goTestArgs(argv []string) []string {
	var args []string
	for i, arg := range argv {
		if isArgsFlag(arg) {
			return append(args, argv[i:]...)
		}
		if isV, _ := parseVerboseFlag(arg); isV {
			continue
		}
		args = append(args, arg)
	}
	return args
}

// listRequested reports whether the go test arguments include -list.
// Arguments after -args are for the test binary and are ignored.
func listRequested(argv []string) bool {
	for _, arg := range argv {
		if isArgsFlag(arg) {
			break
		}
		if name, _, _, ok := parseFlag(arg); ok && (name == "list" || name == "test.list") {
			return true
		}
	}
	return false
}

// printState records how a key's result was handled by processEvents.
type printState struct {
	status Status // status of the last handled result
	n      int    // number of the key's events handled so far
}

func processEvents(r io.Reader, flags Flags, renderer *Renderer, t0 time.Time) error {
	if renderer == nil {
		renderer = defaultRenderer
	}
	w := renderer.out()

	tests := make(TestStorage)
	// printed holds the keys whose results have been handled, whether or
	// not they were shown.
	printed := make(map[Key]printState)
	// runningBench holds the most recently started benchmark per package.
	runningBench := make(map[string]string)
	scanner := bufio.NewScanner(r)

	// printFinished prints the result of key after a terminal event.
	//
	// A test runs once per -count, so each run is printed with its own
	// events. A package reports once, but a build failure has two terminal
	// events (build-fail and fail), so a package is printed again only if its
	// status changes.
	printFinished := func(key Key) {
		events := tests[key]
		p := printed[key]
		if key.Test != "" {
			events = events[p.n:]
		}
		status := events.Status()
		if status == StatusNone || (key.Test == "" && status == p.status) {
			return
		}
		printed[key] = printState{status: status, n: len(tests[key])}
		if flags.Results.Any(status) {
			renderer.PrintDetail(events, flags)
		}
	}

	renderer.PrintHeader()

	for scanner.Scan() {
		var e Event
		log.Println("LINE:", scanner.Text())
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			// Not an event, such as output of a -exec program: show it as is.
			log.Println("unmarshal error:", err)
			if line := scanner.Text(); strings.TrimSpace(line) != "" {
				fmt.Fprintln(w, line)
			}
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
				printFinished(k)
			}
		}
		printFinished(key)
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

		if covered := tests.WithCoverage(); len(covered) > 0 {
			renderer.PrintCoverage(covered)
		}

		renderer.PrintFooter(tests, t0)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(w, "error reading go test output:", err)
		return err
	}

	return nil
}
