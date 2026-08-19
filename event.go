package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

// Action represents a Go test runner action.
type Action string

func (a Action) String() string {
	return string(a)
}

// Status converts a terminal Action to its corresponding Status.
// Returns StatusNone for non-terminal actions.
func (a Action) Status() Status {
	switch a {
	case ActionPass:
		return StatusPass
	case ActionFail:
		return StatusFail
	case ActionSkip:
		return StatusSkip
	case ActionBench:
		return StatusBench
	case ActionBuildFail:
		return StatusBuildFail
	default:
		return StatusNone
	}
}

// IsEnding returns true if the action is a terminal test action.
func (a Action) IsEnding() bool {
	return a.Status() != StatusNone
}

type Actions []Action

var (
	ActionStart       = Action("start")
	ActionRun         = Action("run")
	ActionPause       = Action("pause")
	ActionCont        = Action("cont")
	ActionPass        = Action("pass")
	ActionBench       = Action("bench")
	ActionFail        = Action("fail")
	ActionOutput      = Action("output")
	ActionSkip        = Action("skip")
	ActionFinish      = Action("finish")
	ActionBuildOutput = Action("build-output")
	ActionBuildFail   = Action("build-fail")

	AllActions = Actions{
		ActionRun, ActionPause, ActionCont, ActionPass,
		ActionBench, ActionFail, ActionOutput, ActionSkip,
		ActionStart, ActionFinish, ActionBuildOutput, ActionBuildFail,
	}

	EndingActions = Actions{ActionFail, ActionSkip, ActionPass, ActionBench, ActionBuildFail}
)

// Status represents terminal test state, including 'none' if test never reported as finished.
type Status string

func (s Status) String() string {
	return string(s)
}

type Statuses []Status

const (
	StatusPass      Status = "pass"
	StatusFail      Status = "fail"
	StatusSkip      Status = "skip"
	StatusBench     Status = "bench"
	StatusBuildFail Status = "build-fail"
	StatusNone      Status = "none"
)

var (
	AllStatuses = Statuses{
		StatusBench,
		StatusPass,
		StatusSkip,
		StatusNone,
		StatusFail,
		StatusBuildFail,
	}

	DefaultStatuses = Statuses{
		StatusNone,
		StatusFail,
		StatusBuildFail,
		StatusBench,
	}

	statusNames = map[Status]string{
		StatusFail:      "FAIL",
		StatusPass:      "PASS",
		StatusNone:      "NONE",
		StatusSkip:      "SKIP",
		StatusBench:     "BENCH",
		StatusBuildFail: "BUILD FAIL",
	}
)

// OutputType represents the type of output emitted by a test event.
type OutputType string

func (o OutputType) String() string {
	return string(o)
}

const (
	OutputTypeBlank         OutputType = ""
	OutputTypeFrame         OutputType = "frame"
	OutputTypeError         OutputType = "error"
	OutputTypeErrorContinue OutputType = "error-continue"
)

var (
	AllOutputTypes = []OutputType{
		OutputTypeBlank,
		OutputTypeFrame,
		OutputTypeError,
		OutputTypeErrorContinue,
	}
)

// Key identifies a package and test together.
type Key struct {
	Package string
	Test    string
}

func (t Key) String() string {
	if t.Test == "" {
		return t.Package
	}
	return t.Package + "." + t.Test
}

// Event represents a single JSON event emitted by go test -json.
type Event struct {
	Time        time.Time  `json:"Time"` // encodes as an RFC3339-format string
	Action      Action     `json:"Action,omitempty"`
	Package     string     `json:"Package,omitempty"`
	ImportPath  string     `json:"ImportPath,omitempty"` // new in go 1.24 for build-output
	Test        string     `json:"Test,omitempty"`
	Elapsed     float64    `json:"Elapsed,omitempty"` // seconds
	Output      string     `json:"Output,omitempty"`
	OutputType  OutputType `json:"OutputType,omitempty"`  // new in go 1.27
	FailedBuild string     `json:"FailedBuild,omitempty"` // package ID of the package that failed to build
}

func (e Event) IsFrame() bool {
	return e.OutputType == OutputTypeFrame
}

func (e Event) IsError() bool {
	return e.OutputType == OutputTypeError || e.OutputType == OutputTypeErrorContinue
}

func (e Event) IsErrorHeader() bool {
	return e.OutputType == OutputTypeError
}

func (e Event) IsErrorContinue() bool {
	return e.OutputType == OutputTypeErrorContinue
}

func (t Event) Key() Key {
	pkg := t.Package
	if pkg == "" {
		pkg = t.ImportPath
	}
	if pkg == "" {
		pkg = t.FailedBuild
	}
	return Key{
		Package: pkg,
		Test:    t.Test,
	}
}

// Status returns the Status of the Event.
func (e Event) Status() Status {
	if e.Action == ActionFail && e.FailedBuild != "" {
		return StatusBuildFail
	}
	return e.Action.Status()
}

type Events []Event

func (es Events) Clone() Events {
	events := make(Events, len(es))
	copy(events, es)
	return events
}

func (es Events) Status() Status {
	for _, e := range es {
		if s := e.Status(); s != StatusNone {
			return s
		}
	}
	return StatusNone
}

func (es Events) FindFirstByAction(actions ...Action) *Event {
	for i := range es {
		if slices.Contains(actions, es[i].Action) {
			return &es[i]
		}
	}
	return nil
}

func (es Events) SortByTime() {
	sort.SliceStable(es, func(i, j int) bool {
		return es[i].Time.Before(es[j].Time)
	})
}

// Compact removes events that are boilerplate / uninteresting for printing
func (es Events) Compact() Events {
	var (
		failedAt  float64
		passedAt  float64
		skippedAt float64
	)

	if e := es.FindFirstByAction(ActionPass); e != nil {
		passedAt = e.Elapsed
	}
	if e := es.FindFirstByAction(ActionFail); e != nil {
		failedAt = e.Elapsed
	}
	if e := es.FindFirstByAction(ActionSkip); e != nil {
		skippedAt = e.Elapsed
	}

	var v Events
	for _, e := range es {
		if isIgnoredEvent(e, passedAt, failedAt, skippedAt) {
			continue
		}
		v = append(v, e)
	}
	return v
}

func isIgnoredEvent(e Event, passedAt, failedAt, skippedAt float64) bool {
	// Lifecycle actions without output
	switch e.Action {
	case ActionRun, ActionCont, ActionPause, ActionStart, ActionFinish:
		return true
	}

	if e.Action != ActionOutput {
		return false
	}

	if e.OutputType == OutputTypeFrame {
		return true
	}

	output := strings.TrimLeft(e.Output, " ")
	outputWS := strings.TrimSpace(e.Output)

	// Test-level standard banners
	if e.Test != "" {
		if output == fmt.Sprintf("=== RUN   %s\n", e.Test) ||
			output == fmt.Sprintf("=== CONT  %s\n", e.Test) ||
			output == fmt.Sprintf("=== PAUSE %s\n", e.Test) ||
			output == fmt.Sprintf("--- FAIL: %s (%.2fs)\n", e.Test, failedAt) ||
			output == fmt.Sprintf("--- SKIP: %s (%.2fs)\n", e.Test, skippedAt) ||
			output == fmt.Sprintf("--- PASS: %s (%.2fs)\n", e.Test, passedAt) {
			return true
		}
	}

	// Package-level standard summaries
	if e.Package != "" && e.Test == "" {
		if strings.HasPrefix(output, fmt.Sprintf("ok  	%s", e.Package)) ||
			strings.HasSuffix(output, "[no test files]\n") ||
			output == fmt.Sprintf("ok   %s\n", e.Package) ||
			output == "PASS\n" ||
			output == "FAIL\n" ||
			output == "testing: warning: no tests to run\n" ||
			strings.HasPrefix(outputWS, fmt.Sprintf("FAIL\t%s\t", e.Package)) ||
			(strings.HasPrefix(outputWS, "coverage:") && strings.HasSuffix(outputWS, "of statements")) {
			return true
		}
	}

	return false
}

func (es Events) IsPackageWithoutTest() bool {
	for _, e := range es {
		output := strings.TrimLeft(e.Output, " ")
		if e.Action == ActionOutput && e.Package != "" && e.Test == "" &&
			strings.HasSuffix(output, "[no test files]\n") {
			return true
		}
	}
	return false
}

func (es Events) FindCoverage() string {
	if len(es) == 0 {
		return ""
	}
	if es[0].Package == "" || es[0].Test != "" {
		return ""
	}
	for _, event := range es {
		if event.Action != ActionOutput {
			continue
		}
		output := strings.TrimSpace(event.Output)
		if strings.HasPrefix(output, "coverage: ") && strings.HasSuffix(output, " of statements") {
			output = strings.TrimPrefix(output, "coverage:")
			output = strings.TrimSuffix(output, "of statements")
			return strings.TrimSpace(output)
		}
	}
	return ""
}
