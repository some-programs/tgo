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

func runWithRenderer(ctx context.Context, flags Flags, renderer *Renderer, argv []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var coverEnabled bool
	for _, v := range argv {
		if v == "-cover" || strings.HasPrefix(v, "-coverprofile") || strings.HasPrefix(v, "-coverpkg") {
			coverEnabled = true
		}
	}

	args := []string{"test", "-json"}
	args = append(args, argv...)
	log.Println("args", args)

	cmd := exec.CommandContext(ctx, flags.Bin, args...)
	cmd.Stderr = os.Stderr

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
	var ee *exec.ExitError
	if cmdErr != nil && errors.As(cmdErr, &ee) {
		if ee.Exited() {
			return ExitError(ee.ExitCode())
		}
		return cmdErr
	}
	return nil
}

func processEvents(r io.Reader, flags Flags, renderer *Renderer, coverEnabled bool, t0 time.Time) error {
	if renderer == nil {
		renderer = defaultRenderer
	}
	w := renderer.out()

	tests := make(TestStorage)
	printed := make(map[Key]bool)
	scanner := bufio.NewScanner(r)

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
		if !printed[key] && e.Status() != StatusNone && flags.Results.Any(e.Status()) {
			renderer.PrintDetail(tests[key], flags)
			printed[key] = true
		}
	}

	if len(tests) > 0 {
		// Print any non-finished tests if requested
		if flags.Results.Any(StatusNone) {
			noneTests := tests.
				FilterKeys(printed).
				FindByStatus(StatusNone)
			for _, key := range noneTests.OrderedKeys() {
				renderer.PrintDetail(tests[key], flags)
				printed[key] = true
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
