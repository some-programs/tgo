package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
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

// ModuleRunResult records the test outcome for a single Go module.
type ModuleRunResult struct {
	Dir     string
	Storage TestStorage
	Err     error
}

// Failed reports whether this module run had any test or command failure.
func (m ModuleRunResult) Failed() bool {
	if m.Err != nil {
		return true
	}
	stats := m.Storage.Stats()
	return stats.Fail > 0 || stats.BuildFail > 0
}

func runWithRenderer(parent context.Context, flags Flags, renderer *Renderer, argv []string) error {
	if flags.R != "" {
		return runRecursive(parent, flags, renderer, argv)
	}
	_, err := runSingle(parent, flags, renderer, argv, true, true)
	return err
}

func runSingle(parent context.Context, flags Flags, renderer *Renderer, argv []string, printHeader, printSummaryFooter bool) (TestStorage, error) {
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
	setupInterrupt(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdout.Close()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	t0 := time.Now()
	storage, procErr := processEventsInternal(stdout, flags, renderer, t0, printHeader, printSummaryFooter)
	if procErr != nil {
		log.Println("process error:", procErr)
	}
	// Drain what processEvents left unread, so go test can't block writing
	// to a full pipe and keep cmd.Wait from returning.
	_, _ = io.Copy(io.Discard, stdout)

	cmdErr := cmd.Wait()
	if parent.Err() != nil {
		return storage, exitInterrupted
	}
	var ee *exec.ExitError
	if cmdErr != nil && errors.As(cmdErr, &ee) {
		if ee.Exited() {
			return storage, ExitError(ee.ExitCode())
		}
		return storage, cmdErr
	}
	if procErr != nil {
		return storage, procErr
	}
	return storage, nil
}

func runRecursive(parent context.Context, flags Flags, renderer *Renderer, argv []string) error {
	if renderer == nil {
		renderer = defaultRenderer
	}

	searchRoot := flags.R
	if flags.Dir != "" && !filepath.IsAbs(flags.R) {
		searchRoot = filepath.Join(flags.Dir, flags.R)
	}

	modules, err := findModules(searchRoot)
	if err != nil {
		return err
	}
	if len(modules) == 0 {
		return fmt.Errorf("no go.mod found under %s", flags.R)
	}

	moduleArgv := argv
	if !hasPackageTarget(argv) {
		moduleArgv = append(slices.Clone(argv), "./...")
	}

	failFast := failFastRequested(argv)
	t0 := time.Now()

	renderer.PrintHeader()

	var results []ModuleRunResult
	var firstFailure error
	skippedCount := 0

	for i, modDir := range modules {
		if parent.Err() != nil {
			return exitInterrupted
		}

		relDir := modDir
		if r, err := filepath.Rel(searchRoot, modDir); err == nil {
			relDir = r
		}
		relDir = cleanRelDir(filepath.Clean(relDir))

		renderer.PrintModuleHeader(relDir)

		modFlags := flags
		modFlags.Dir = modDir

		storage, runErr := runSingle(parent, modFlags, renderer, moduleArgv, false, false)
		res := ModuleRunResult{
			Dir:     relDir,
			Storage: storage,
			Err:     runErr,
		}
		results = append(results, res)

		if res.Failed() {
			if firstFailure == nil {
				if runErr != nil {
					firstFailure = runErr
				} else {
					firstFailure = ExitError(1)
				}
			}
			if failFast {
				skippedCount = len(modules) - (i + 1)
				break
			}
		}
	}

	renderer.PrintRecursiveSummary(results, skippedCount, t0)

	if parent.Err() != nil {
		return exitInterrupted
	}
	return firstFailure
}

// findModules searches root recursively for directories containing go.mod,
// skipping .git, vendor, testdata, and hidden directories.
func findModules(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var modules []string
	cleanRoot := filepath.Clean(root)

	err = filepath.WalkDir(cleanRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if path != cleanRoot {
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Name() == "go.mod" {
			modules = append(modules, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(modules)
	return modules, nil
}

func cleanRelDir(dir string) string {
	if dir == "." {
		return "."
	}
	if !filepath.IsAbs(dir) && !strings.HasPrefix(dir, "."+string(filepath.Separator)) && !strings.HasPrefix(dir, ".."+string(filepath.Separator)) {
		return "." + string(filepath.Separator) + dir
	}
	return dir
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

// maxLineSize is the longest line of go test output that is processed.
const maxLineSize = 1 << 30

// printState records how a key's result was handled by processEvents.
type printState struct {
	status Status // status of the last handled result
	n      int    // number of the key's events handled so far
}

func processEvents(r io.Reader, flags Flags, renderer *Renderer, t0 time.Time) error {
	_, err := processEventsInternal(r, flags, renderer, t0, true, true)
	return err
}

func processEventsInternal(r io.Reader, flags Flags, renderer *Renderer, t0 time.Time, printHeader, printSummaryFooter bool) (TestStorage, error) {
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
	// go test -json puts each output line in one event, so a test that logs
	// a large value produces a line far beyond the default 64 KB limit.
	scanner.Buffer(nil, maxLineSize)

	// unhandled returns the events of key that printFinished has not handled.
	//
	// A test runs once per -count, so each run is judged by its own events.
	// A package reports once, but a build failure has two terminal events
	// (build-fail and fail), so a package is judged by all its events.
	unhandled := func(key Key) Events {
		events := tests[key]
		if key.Test != "" {
			events = events[printed[key].n:]
		}
		return events
	}

	// printFinished prints the result of key after a terminal event. A
	// package is printed again only if its status changes.
	printFinished := func(key Key) {
		events := unhandled(key)
		p := printed[key]
		status := events.Status()
		if status == StatusNone || (key.Test == "" && status == p.status) {
			return
		}
		printed[key] = printState{status: status, n: len(tests[key])}
		if flags.Results.Any(status) {
			renderer.PrintDetail(events, flags)
		}
	}

	if printHeader {
		renderer.PrintHeader()
	}

	for scanner.Scan() {
		var e Event
		if flags.V > V3 {
			log.Println("LINE:", scanner.Text())
		}
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
		// A test that never finished, such as after an interrupt, has status
		// none. Under -count a test's last run may be unfinished after an
		// earlier run was reported, so that run alone is shown and stands for
		// the test in the summaries and the footer.
		unfinished := make(TestStorage)
		for key := range tests {
			if rest := unhandled(key); len(rest) > 0 && rest.Status() == StatusNone {
				unfinished[key] = rest
			}
		}
		if flags.Results.Any(StatusNone) {
			for _, key := range unfinished.OrderedKeys() {
				renderer.PrintDetail(unfinished[key], flags)
			}
		}
		results := maps.Clone(tests)
		maps.Copy(results, unfinished)

		if printSummaryFooter {
			// Print configured summaries
			for _, status := range flags.Summary {
				filtered := results.FindByStatus(status)
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

			renderer.PrintFooter(results, t0)
		}
		return results, scanner.Err()
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(w, "error reading go test output:", err)
		return nil, err
	}

	return tests, nil
}
