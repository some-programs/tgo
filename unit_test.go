package main

import (
	"encoding/json"
	"flag"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFlags_Register(t *testing.T) {
	t.Parallel()
	var f Flags
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f.Register(fs)

	if fs.Lookup("bin") == nil {
		t.Error("flag 'bin' not registered")
	}
	if fs.Lookup("results") == nil {
		t.Error("flag 'results' not registered")
	}
	if fs.Lookup("summary") == nil {
		t.Error("flag 'summary' not registered")
	}
}

func TestFlags_PrintHelp(t *testing.T) {
	t.Parallel()
	var f Flags
	var sb strings.Builder
	f.PrintHelp(&sb)
	if sb.Len() == 0 {
		t.Error("PrintHelp produced no output")
	}
}

func TestFlags_PrintConfig(t *testing.T) {
	t.Parallel()
	var f Flags
	f.Results = Statuses{StatusPass}
	var sb strings.Builder
	f.printConfig(&sb)
	if !strings.Contains(sb.String(), "TGO_RESULTS: pass") {
		t.Errorf("printConfig output missing expected content: %s", sb.String())
	}
}

func TestFlags_Setup(t *testing.T) {
	t.Parallel()

	t.Run("All", func(t *testing.T) {
		t.Parallel()
		f := Flags{All: true}
		f.Setup(nil, nil)
		if len(f.Results) != len(AllStatuses) {
			t.Error("expected all statuses in Results")
		}
	})

	t.Run("V2", func(t *testing.T) {
		t.Parallel()
		f := Flags{}
		f.Setup([]string{"-v"}, nil)
		if f.V != V2 {
			t.Errorf("expected V2, got %v", f.V)
		}
		if !slices.Equal(f.Results, Statuses{StatusBench, StatusPass, StatusNone, StatusFail}) {
			t.Errorf("unexpected Results %v", f.Results)
		}
	})

	t.Run("NoV", func(t *testing.T) {
		t.Parallel()
		f := Flags{Results: Statuses{StatusFail}}
		f.Setup([]string{"./...", "-args", "-v"}, nil)
		if f.V != V0 || !slices.Equal(f.Results, Statuses{StatusFail}) {
			t.Errorf("Setup changed flags without -v: V=%v Results=%v", f.V, f.Results)
		}
	})

	t.Run("VOnlyRaises", func(t *testing.T) {
		t.Parallel()
		f := Flags{V: V4}
		f.Setup([]string{"-v"}, map[string]bool{"v": true})
		if f.V != V4 {
			t.Errorf("expected V4 to be kept, got %v", f.V)
		}
	})

	t.Run("KeepsExplicitSettings", func(t *testing.T) {
		t.Parallel()
		f := Flags{
			Results:          Statuses{StatusFail},
			Summary:          Statuses{StatusSkip},
			HideEmptyResults: Statuses{StatusPass},
		}
		f.Setup([]string{"-v"}, map[string]bool{"results": true, "summary": true})
		if !slices.Equal(f.Results, Statuses{StatusFail}) {
			t.Errorf("explicit Results replaced: %v", f.Results)
		}
		if !slices.Equal(f.Summary, Statuses{StatusSkip}) {
			t.Errorf("explicit Summary replaced: %v", f.Summary)
		}
		if !slices.Equal(f.HideEmptyResults, Statuses{StatusBench, StatusPass, StatusNone}) {
			t.Errorf("unset HideEmptyResults not filled in: %v", f.HideEmptyResults)
		}
	})

	t.Run("AllWins", func(t *testing.T) {
		t.Parallel()
		f := Flags{All: true}
		f.Setup([]string{"-v"}, map[string]bool{"all": true})
		if !slices.Equal(f.Results, AllStatuses) || !slices.Equal(f.Summary, AllStatuses) {
			t.Errorf("-v replaced -all settings: Results=%v Summary=%v", f.Results, f.Summary)
		}
		if f.V != V2 {
			t.Errorf("expected V2, got %v", f.V)
		}
	})
}

func TestVerboseFlags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args    []string
		verbose bool
		goArgs  []string
	}{
		{nil, false, nil},
		{[]string{"./..."}, false, []string{"./..."}},
		{[]string{"-v", "./..."}, true, []string{"./..."}},
		{[]string{"--v"}, true, nil},
		{[]string{"-v=true"}, true, nil},
		{[]string{"-v=false"}, false, nil},
		{[]string{"-v", "-v=false"}, false, nil},
		{[]string{"-test.v"}, true, nil},
		{[]string{"-test.v=true"}, true, nil},
		{[]string{"-vet=off"}, false, []string{"-vet=off"}},
		{[]string{"v"}, false, []string{"v"}},
		{[]string{"./...", "-args", "-v"}, false, []string{"./...", "-args", "-v"}},
		{[]string{"-v", "./...", "--args", "-v"}, true, []string{"./...", "--args", "-v"}},
	}
	for _, tt := range tests {
		if got := verboseRequested(tt.args); got != tt.verbose {
			t.Errorf("verboseRequested(%q) = %v, want %v", tt.args, got, tt.verbose)
		}
		if got := goTestArgs(tt.args); !slices.Equal(got, tt.goArgs) {
			t.Errorf("goTestArgs(%q) = %q, want %q", tt.args, got, tt.goArgs)
		}
	}
}

func TestAction_Methods(t *testing.T) {
	t.Parallel()
	if ActionPass.String() != "pass" {
		t.Errorf("expected 'pass', got %s", ActionPass.String())
	}

	tests := []struct {
		action     Action
		wantStatus Status
		wantEnding bool
	}{
		{ActionPass, StatusPass, true},
		{ActionFail, StatusFail, true},
		{ActionSkip, StatusSkip, true},
		{ActionBench, StatusBench, true},
		{ActionBuildFail, StatusBuildFail, true},
		{ActionRun, StatusNone, false},
		{ActionOutput, StatusNone, false},
	}

	for _, tt := range tests {
		if got := tt.action.Status(); got != tt.wantStatus {
			t.Errorf("%s.Status() = %v, want %v", tt.action, got, tt.wantStatus)
		}
		if got := tt.action.IsEnding(); got != tt.wantEnding {
			t.Errorf("%s.IsEnding() = %v, want %v", tt.action, got, tt.wantEnding)
		}
	}
}

func TestStatus_Methods(t *testing.T) {
	t.Parallel()
	if StatusPass.String() != "pass" {
		t.Errorf("expected 'pass', got %s", StatusPass.String())
	}
}

func TestOutputType_Methods(t *testing.T) {
	t.Parallel()
	tests := []struct {
		outputType OutputType
		wantStr    string
	}{
		{OutputTypeBlank, ""},
		{OutputTypeFrame, "frame"},
		{OutputTypeError, "error"},
		{OutputTypeErrorContinue, "error-continue"},
	}

	for _, tt := range tests {
		if got := tt.outputType.String(); got != tt.wantStr {
			t.Errorf("%q.String() = %q, want %q", tt.outputType, got, tt.wantStr)
		}
	}

	if len(AllOutputTypes) != 4 {
		t.Errorf("expected 4 output types, got %d", len(AllOutputTypes))
	}

	frameEv := Event{OutputType: OutputTypeFrame}
	if !frameEv.IsFrame() {
		t.Error("expected IsFrame() to be true for OutputTypeFrame")
	}
	if frameEv.IsError() {
		t.Error("expected IsError() to be false for OutputTypeFrame")
	}

	errEv := Event{OutputType: OutputTypeError}
	if errEv.IsFrame() {
		t.Error("expected IsFrame() to be false for OutputTypeError")
	}
	if !errEv.IsError() {
		t.Error("expected IsError() to be true for OutputTypeError")
	}
	if !errEv.IsErrorHeader() {
		t.Error("expected IsErrorHeader() to be true for OutputTypeError")
	}
	if errEv.IsErrorContinue() {
		t.Error("expected IsErrorContinue() to be false for OutputTypeError")
	}

	errContEv := Event{OutputType: OutputTypeErrorContinue}
	if !errContEv.IsError() {
		t.Error("expected IsError() to be true for OutputTypeErrorContinue")
	}
	if errContEv.IsErrorHeader() {
		t.Error("expected IsErrorHeader() to be false for OutputTypeErrorContinue")
	}
	if !errContEv.IsErrorContinue() {
		t.Error("expected IsErrorContinue() to be true for OutputTypeErrorContinue")
	}
}

func TestEvent_Status(t *testing.T) {
	t.Parallel()
	tests := []struct {
		event Event
		want  Status
	}{
		{Event{Action: ActionPass}, StatusPass},
		{Event{Action: ActionFail}, StatusFail},
		{Event{Action: ActionFail, FailedBuild: "pkg"}, StatusBuildFail},
		{Event{Action: ActionBuildFail}, StatusBuildFail},
		{Event{Action: ActionRun}, StatusNone},
	}

	for _, tt := range tests {
		if got := tt.event.Status(); got != tt.want {
			t.Errorf("%+v.Status() = %v, want %v", tt.event, got, tt.want)
		}
	}
}

func TestStatuses_Methods(t *testing.T) {
	t.Parallel()
	ss := Statuses{StatusPass, StatusFail}

	t.Run("Any", func(t *testing.T) {
		t.Parallel()
		if !ss.Any(StatusPass) {
			t.Error("expected Any(StatusPass) to be true")
		}
		if ss.Any(StatusSkip) {
			t.Error("expected Any(StatusSkip) to be false")
		}
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()
		if ss.String() != "pass,fail" {
			t.Errorf("expected 'pass,fail', got %s", ss.String())
		}
	})

	t.Run("Set", func(t *testing.T) {
		t.Parallel()
		var sss Statuses
		if err := sss.Set("pass,skip"); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		if len(sss) != 2 || sss[0] != StatusPass || sss[1] != StatusSkip {
			t.Errorf("unexpected statuses: %v", sss)
		}

		if err := sss.Set(" fail, pass ,"); err != nil {
			t.Fatalf("Set with spaces failed: %v", err)
		}
		if len(sss) != 2 || sss[0] != StatusFail || sss[1] != StatusPass {
			t.Errorf("unexpected statuses: %v", sss)
		}

		if err := sss.Set("invalid"); err == nil {
			t.Error("expected error for invalid status")
		}

		if err := sss.Set("-"); err != nil {
			t.Fatalf("Set('-') failed: %v", err)
		}
		if len(sss) != 0 {
			t.Errorf("expected empty statuses, got %v", sss)
		}

		if err := sss.Set("all"); err != nil {
			t.Fatalf("Set('all') failed: %v", err)
		}
		if len(sss) != len(AllStatuses) {
			t.Errorf("expected all statuses, got %v", sss)
		}
	})
}

func TestEvent_Key(t *testing.T) {
	t.Parallel()
	e := Event{Package: "pkg", Test: "test"}
	if e.Key().Package != "pkg" || e.Key().Test != "test" {
		t.Errorf("unexpected key: %v", e.Key())
	}

	e2 := Event{ImportPath: "import", Test: "test"}
	if e2.Key().Package != "import" || e2.Key().Test != "test" {
		t.Errorf("unexpected key: %v", e2.Key())
	}

	e3 := Event{FailedBuild: "badpkg", Test: "test"}
	if e3.Key().Package != "badpkg" || e3.Key().Test != "test" {
		t.Errorf("unexpected key: %v", e3.Key())
	}
}

func TestEvent_JSON(t *testing.T) {
	t.Parallel()
	raw := `{"Time":"2026-08-19T10:00:00Z","Action":"output","Package":"my/pkg","Test":"TestFoo","Output":"=== RUN TestFoo\n","OutputType":"frame"}`
	var e Event
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if e.OutputType != OutputTypeFrame {
		t.Errorf("expected OutputType %q, got %q", OutputTypeFrame, e.OutputType)
	}
	if !e.IsFrame() {
		t.Error("expected IsFrame() to be true")
	}

	rawErr := `{"Time":"2026-08-19T10:00:00Z","Action":"output","Package":"my/pkg","Test":"TestFoo","Output":"error here\n","OutputType":"error"}`
	var eErr Event
	if err := json.Unmarshal([]byte(rawErr), &eErr); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if eErr.OutputType != OutputTypeError {
		t.Errorf("expected OutputType %q, got %q", OutputTypeError, eErr.OutputType)
	}
	if !eErr.IsError() {
		t.Error("expected IsError() to be true")
	}
}

func TestKey_String(t *testing.T) {
	t.Parallel()
	k := Key{Package: "pkg", Test: "test"}
	if k.String() != "pkg.test" {
		t.Errorf("expected 'pkg.test', got %s", k.String())
	}

	k2 := Key{Package: "pkg"}
	if k2.String() != "pkg" {
		t.Errorf("expected 'pkg', got %s", k2.String())
	}
}

func TestEvents_Methods(t *testing.T) {
	t.Parallel()
	es := Events{
		{Action: ActionRun, Time: time.Now()},
		{Action: ActionOutput, Output: "foo\n"},
		{Action: ActionPass, Elapsed: 0.5},
	}

	t.Run("Clone", func(t *testing.T) {
		t.Parallel()
		cloned := es.Clone()
		if len(cloned) != len(es) {
			t.Error("clone length mismatch")
		}
		if cloned[0].Action != es[0].Action {
			t.Error("clone content mismatch")
		}
	})

	t.Run("Status", func(t *testing.T) {
		t.Parallel()
		if es.Status() != StatusPass {
			t.Errorf("expected StatusPass, got %s", es.Status())
		}
		if (Events{{Action: ActionFail}}).Status() != StatusFail {
			t.Error("expected StatusFail")
		}
		if (Events{{Action: ActionBuildFail}}).Status() != StatusBuildFail {
			t.Error("expected StatusBuildFail")
		}
		if (Events{{Action: ActionSkip}}).Status() != StatusSkip {
			t.Error("expected StatusSkip")
		}
		if (Events{{Action: ActionBench}}).Status() != StatusBench {
			t.Error("expected StatusBench")
		}
		if (Events{}).Status() != StatusNone {
			t.Error("expected StatusNone for empty events")
		}
	})

	t.Run("FindFirstByAction", func(t *testing.T) {
		t.Parallel()
		e := es.FindFirstByAction(ActionPass)
		if e == nil || e.Action != ActionPass {
			t.Error("failed to find ActionPass")
		}
		if es.FindFirstByAction(ActionFail) != nil {
			t.Error("found non-existent action")
		}
	})

	t.Run("Compact", func(t *testing.T) {
		t.Parallel()
		es3 := Events{
			{Action: ActionRun, Test: "test"},
			{Action: ActionOutput, Test: "test", Output: "=== RUN   test\n"},
			{Action: ActionOutput, Test: "test", Output: "actual output\n"},
			{Action: ActionPass, Test: "test", Elapsed: 0.1},
			{Action: ActionOutput, Test: "test", Output: "--- PASS: test (0.10s)\n"},
		}
		compacted := es3.Compact()
		if len(compacted) != 2 {
			t.Errorf("unexpected number of compacted events: %d, expected 2", len(compacted))
		}
		if compacted[0].Output != "actual output\n" {
			t.Errorf("expected 'actual output\n', got %s", compacted[0].Output)
		}
		if compacted[1].Action != ActionPass {
			t.Errorf("expected ActionPass, got %s", compacted[1].Action)
		}

		// Test OutputTypeFrame compaction
		esFrame := Events{
			{Action: ActionOutput, Test: "test", Output: "framing header\n", OutputType: OutputTypeFrame},
			{Action: ActionOutput, Test: "test", Output: "real log\n", OutputType: OutputTypeBlank},
			{Action: ActionOutput, Test: "test", Output: "error details\n", OutputType: OutputTypeError},
		}
		compactedFrame := esFrame.Compact()
		if len(compactedFrame) != 2 {
			t.Errorf("unexpected number of compacted frame events: %d, expected 2", len(compactedFrame))
		}
		if compactedFrame[0].Output != "real log\n" || compactedFrame[1].Output != "error details\n" {
			t.Errorf("unexpected compacted events: %+v", compactedFrame)
		}
	})

	t.Run("IsPackageWithoutTest", func(t *testing.T) {
		t.Parallel()
		es4 := Events{
			{Action: ActionOutput, Package: "pkg", Output: "ok  	pkg [no test files]\n"},
		}
		if !es4.IsPackageWithoutTest() {
			t.Error("expected IsPackageWithoutTest to be true")
		}
	})

	t.Run("FindCoverage", func(t *testing.T) {
		t.Parallel()
		es5 := Events{
			{Package: "pkg", Action: ActionOutput, Output: "coverage: 50.0% of statements\n"},
		}
		if es5.FindCoverage() != "50.0%" {
			t.Errorf("expected 50.0%%, got %s", es5.FindCoverage())
		}
	})
}

func TestTestStorage_Methods(t *testing.T) {
	t.Parallel()

	newTS := func() TestStorage {
		ts := make(TestStorage)
		ts.Append(Event{Package: "pkg", Test: "test1", Action: ActionPass})
		ts.Append(Event{Package: "pkg", Test: "test2", Action: ActionFail})
		ts.Append(Event{Package: "pkg", Action: ActionPass})
		return ts
	}

	t.Run("Append_FailedBuild", func(t *testing.T) {
		t.Parallel()
		storage := make(TestStorage)
		storage.Append(Event{FailedBuild: "build/fail/pkg", Action: ActionFail})
		if _, ok := storage[Key{Package: "build/fail/pkg"}]; !ok {
			t.Errorf("expected storage to have key with package 'build/fail/pkg'")
		}
	})

	t.Run("Append_ImportPath", func(t *testing.T) {
		t.Parallel()
		storage := make(TestStorage)
		storage.Append(Event{ImportPath: "import/pkg", Action: ActionBuildOutput})
		events, ok := storage[Key{Package: "import/pkg"}]
		if !ok || len(events) != 1 || events[0].Action != ActionOutput {
			t.Errorf("expected storage to normalize ActionBuildOutput to ActionOutput and package 'import/pkg'")
		}
	})

	t.Run("OrderedKeys", func(t *testing.T) {
		t.Parallel()
		keys := newTS().OrderedKeys()
		if len(keys) != 3 {
			t.Fatalf("expected 3 keys, got %d", len(keys))
		}
	})

	t.Run("FilterPackageResults", func(t *testing.T) {
		t.Parallel()
		filtered := newTS().FilterPackageResults()
		if len(filtered) != 2 {
			t.Error("expected 2 test results")
		}
	})

	t.Run("FindPackageResults", func(t *testing.T) {
		t.Parallel()
		filtered := newTS().FindPackageResults()
		if len(filtered) != 1 {
			t.Error("expected 1 package result")
		}
	})

	t.Run("CountTests", func(t *testing.T) {
		t.Parallel()
		if newTS().CountTests() != 2 {
			t.Errorf("expected 2 tests, got %d", newTS().CountTests())
		}
	})

	t.Run("Union", func(t *testing.T) {
		t.Parallel()
		ts2 := make(TestStorage)
		ts2.Append(Event{Package: "pkg2", Action: ActionPass})
		union := newTS().Union(ts2)
		if len(union) != 4 {
			t.Errorf("union failed, expected 4 results, got %d", len(union))
		}
	})

	t.Run("FilterKeys", func(t *testing.T) {
		t.Parallel()
		exclude := map[Key]bool{{Package: "pkg", Test: "test1"}: true}
		filtered := newTS().FilterKeys(exclude)
		if len(filtered) != 2 {
			t.Error("FilterKeys failed")
		}
	})

	t.Run("FindPackageTests", func(t *testing.T) {
		t.Parallel()
		pkgTests := newTS().FindPackageTests("pkg")
		if len(pkgTests) != 3 {
			t.Error("FindPackageTests failed")
		}
	})

	t.Run("FindByAction", func(t *testing.T) {
		t.Parallel()
		failed := newTS().FindByAction(ActionFail)
		if len(failed) != 1 {
			t.Error("FindByAction failed")
		}
	})

	t.Run("FilterAction", func(t *testing.T) {
		t.Parallel()
		noFail := newTS().FilterAction(ActionFail)
		if len(noFail) != 2 {
			t.Error("FilterAction failed")
		}
	})

	t.Run("WithCoverage", func(t *testing.T) {
		t.Parallel()
		ts := newTS()
		ts.Append(Event{Package: "pkg_cov", Action: ActionOutput, Output: "coverage: 10.0% of statements\n"})
		cov := ts.WithCoverage()
		if len(cov) != 1 {
			t.Errorf("expected 1 coverage result, got %d", len(cov))
		}
	})

	t.Run("FilterNotests", func(t *testing.T) {
		t.Parallel()
		ts := newTS()
		ts.Append(Event{Package: "pkg_cov", Action: ActionOutput, Output: "coverage: 10.0% of statements\n"})
		ts.Append(Event{Package: "pkg_no", Action: ActionOutput, Output: "ok  	pkg_no [no test files]\n"})
		filtered := ts.FilterNotests()
		if len(filtered) != 4 {
			t.Errorf("expected 4 results, got %d", len(filtered))
		}
	})

	t.Run("Filter", func(t *testing.T) {
		t.Parallel()
		filtered := newTS().Filter(func(k Key, _ Events) bool {
			return k.Test == "test1"
		})
		if len(filtered) != 1 {
			t.Errorf("expected 1 result, got %d", len(filtered))
		}
	})

	t.Run("FindByStatus", func(t *testing.T) {
		t.Parallel()
		ts := newTS()
		ts.Append(Event{Package: "pkg_cov", Action: ActionOutput, Output: "coverage: 10.0% of statements\n"})
		ts.Append(Event{Package: "pkg_no", Action: ActionOutput, Output: "ok  	pkg_no [no test files]\n"})

		pass := ts.FindByStatus(StatusPass)
		if len(pass) != 2 {
			t.Errorf("expected 2 pass results, got %d", len(pass))
		}
		fail := ts.FindByStatus(StatusFail)
		if len(fail) != 1 {
			t.Errorf("expected 1 fail result, got %d", len(fail))
		}
		none := ts.FindByStatus(StatusNone)
		if len(none) != 2 { // pkg_cov and pkg_no have only output
			t.Errorf("expected 2 none results, got %d", len(none))
		}
	})

	t.Run("Stats", func(t *testing.T) {
		t.Parallel()
		ts := newTS()
		ts.Append(Event{Package: "pkg_cov", Action: ActionOutput, Output: "coverage: 10.0% of statements\n"})
		ts.Append(Event{Package: "pkg_no", Action: ActionOutput, Output: "ok  	pkg_no [no test files]\n"})

		stats := ts.Stats()
		if stats.Pass != 1 || stats.Fail != 1 || stats.None != 2 {
			t.Errorf("unexpected stats: %+v", stats)
		}
	})
}

var defaultRendererMu sync.Mutex

// captureStdout is a test helper that redirects defaultRenderer output and returns its content as a string.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	defaultRendererMu.Lock()
	defer defaultRendererMu.Unlock()
	var sb strings.Builder
	old := defaultRenderer.w
	defaultRenderer.w = &sb
	defer func() {
		defaultRenderer.w = old
	}()
	fn()
	return sb.String()
}

func TestPrintingFunctions(t *testing.T) {
	t.Parallel()
	newTS := func() TestStorage {
		ts := make(TestStorage)
		ts.Append(Event{Package: "pkg", Action: ActionPass, Elapsed: 0.1})
		ts.Append(Event{Package: "pkg", Test: "Test1", Action: ActionPass, Elapsed: 0.1})
		ts.Append(Event{Package: "pkg_cov", Action: ActionOutput, Output: "coverage: 50.0% of statements\n"})
		return ts
	}

	t.Run("PrintShortSummary", func(t *testing.T) {
		t.Parallel()
		got := captureStdout(t, func() {
			newTS().PrintShortSummary(StatusPass)
		})
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg") {
			t.Errorf("PrintShortSummary output missing expected content: %q", got)
		}
	})

	t.Run("PrintSummary", func(t *testing.T) {
		t.Parallel()
		got := captureStdout(t, func() {
			newTS().PrintSummary(StatusPass)
		})
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg.Test1") {
			t.Errorf("PrintSummary output missing expected content: %q", got)
		}
	})

	t.Run("PrintCoverage", func(t *testing.T) {
		t.Parallel()
		got := captureStdout(t, func() {
			newTS().PrintCoverage()
		})
		if !strings.Contains(got, "COVR") || !strings.Contains(got, "50.0%") {
			t.Errorf("PrintCoverage output missing expected content: %q", got)
		}
	})

	t.Run("PrintDetail", func(t *testing.T) {
		t.Parallel()
		events := newTS()[Key{Package: "pkg", Test: "Test1"}]
		got := captureStdout(t, func() {
			events.PrintDetail(Flags{V: V0})
		})
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg") || !strings.Contains(got, "Test1") {
			t.Errorf("PrintDetail output missing expected content: %q", got)
		}
	})
}

func TestRenderer_DirectWriter(t *testing.T) {
	t.Parallel()
	newTS := func() TestStorage {
		ts := make(TestStorage)
		ts.Append(Event{Package: "pkg", Action: ActionPass, Elapsed: 0.1})
		ts.Append(Event{Package: "pkg", Test: "Test1", Action: ActionPass, Elapsed: 0.1})
		ts.Append(Event{Package: "pkg_cov", Action: ActionOutput, Output: "coverage: 50.0% of statements\n"})
		return ts
	}

	t.Run("PrintShortSummary", func(t *testing.T) {
		t.Parallel()
		var sb strings.Builder
		r := NewRenderer(&sb)
		r.PrintShortSummary(newTS(), StatusPass)
		got := sb.String()
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg") {
			t.Errorf("PrintShortSummary output missing expected content: %q", got)
		}
	})

	t.Run("PrintSummary", func(t *testing.T) {
		t.Parallel()
		var sb strings.Builder
		r := NewRenderer(&sb)
		r.PrintSummary(newTS(), StatusPass)
		got := sb.String()
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg.Test1") {
			t.Errorf("PrintSummary output missing expected content: %q", got)
		}
	})

	t.Run("PrintCoverage", func(t *testing.T) {
		t.Parallel()
		var sb strings.Builder
		r := NewRenderer(&sb)
		r.PrintCoverage(newTS())
		got := sb.String()
		if !strings.Contains(got, "COVR") || !strings.Contains(got, "50.0%") {
			t.Errorf("PrintCoverage output missing expected content: %q", got)
		}
	})

	t.Run("PrintDetail", func(t *testing.T) {
		t.Parallel()
		var sb strings.Builder
		r := NewRenderer(&sb)
		events := newTS()[Key{Package: "pkg", Test: "Test1"}]
		r.PrintDetail(events, Flags{V: V0})
		got := sb.String()
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg") || !strings.Contains(got, "Test1") {
			t.Errorf("PrintDetail output missing expected content: %q", got)
		}
	})

	t.Run("PrintFooter", func(t *testing.T) {
		t.Parallel()
		var sb strings.Builder
		r := NewRenderer(&sb)
		r.PrintFooter(newTS(), time.Now())
		got := sb.String()
		if !strings.Contains(got, "PASS:1") {
			t.Errorf("PrintFooter output missing expected content: %q", got)
		}
	})

	t.Run("PrintHeader", func(t *testing.T) {
		t.Parallel()
		var sb strings.Builder
		r := NewRenderer(&sb)
		r.PrintHeader()
		got := sb.String()
		if !strings.Contains(got, "*****") {
			t.Errorf("PrintHeader output missing expected content: %q", got)
		}
	})
}

func TestProcessEvents(t *testing.T) {
	t.Parallel()
	input := `{"Time":"2026-08-19T10:00:00Z","Action":"run","Package":"my/pkg","Test":"TestFoo"}
{"Time":"2026-08-19T10:00:01Z","Action":"pass","Package":"my/pkg","Test":"TestFoo","Elapsed":0.05}
`
	var sb strings.Builder
	r := NewRenderer(&sb)
	flags := Flags{
		Results: Statuses{StatusPass},
		Summary: Statuses{StatusPass},
	}
	err := processEvents(strings.NewReader(input), flags, r, time.Now())
	if err != nil {
		t.Fatalf("processEvents returned error: %v", err)
	}
	got := sb.String()
	if !strings.Contains(got, "my/pkg.TestFoo") {
		t.Errorf("expected test name in output, got: %s", got)
	}
}

func TestProcessEvents_NonJSONLine(t *testing.T) {
	t.Parallel()
	input := `{"Action":"run","Package":"my/pkg","Test":"TestFoo"}
not an event

{"Action":"pass","Package":"my/pkg","Test":"TestFoo"}
`
	var sb strings.Builder
	flags := Flags{Results: Statuses{StatusPass}}
	if err := processEvents(strings.NewReader(input), flags, NewRenderer(&sb), time.Now()); err != nil {
		t.Fatal(err)
	}
	got := normalizeOutput(sb.String())
	want := `*****
not an event
=== PASS my/pkg.TestFoo

══════ 12:00:00 | PASS:1 | FAIL:0 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════`
	if got != want {
		t.Errorf("output mismatch:\n--- GOT ---\n%s\n--- WANT ---\n%s", got, want)
	}
}

func TestRenderer_EventTextColor(t *testing.T) {
	t.Parallel()
	r := NewRenderer(nil)

	t.Run("with OutputType", func(t *testing.T) {
		t.Parallel()
		errHeaderEv := Event{OutputType: OutputTypeError, Action: ActionOutput}
		fn := r.eventTextColor(errHeaderEv, StatusFail, true, failColor)
		if fn == nil {
			t.Fatal("expected non-nil color function")
		}

		errContEv := Event{OutputType: OutputTypeErrorContinue, Action: ActionOutput}
		fnCont := r.eventTextColor(errContEv, StatusFail, true, failColor)
		if fnCont == nil {
			t.Fatal("expected non-nil color function")
		}

		plainEv := Event{OutputType: OutputTypeBlank, Action: ActionOutput}
		fnPlain := r.eventTextColor(plainEv, StatusFail, true, failColor)
		if fnPlain == nil {
			t.Fatal("expected non-nil color function")
		}

		skipEv := Event{OutputType: OutputTypeBlank, Action: ActionOutput}
		fnSkip := r.eventTextColor(skipEv, StatusSkip, true, skipColor)
		if fnSkip == nil {
			t.Fatal("expected non-nil color function")
		}
	})

	t.Run("without OutputType (fallback)", func(t *testing.T) {
		t.Parallel()
		plainEv := Event{OutputType: OutputTypeBlank, Action: ActionOutput}
		fn := r.eventTextColor(plainEv, StatusFail, false, failColor)
		if fn == nil {
			t.Fatal("expected fallback color function")
		}
	})
}

func TestRenderer_PrintDetail_OutputTypes(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	r := NewRenderer(&sb)

	events := Events{
		{Action: ActionRun, Package: "pkg", Test: "TestFail", Time: time.Now()},
		{Action: ActionOutput, Package: "pkg", Test: "TestFail", Output: "debug log\n", OutputType: OutputTypeBlank, Time: time.Now()},
		{Action: ActionOutput, Package: "pkg", Test: "TestFail", Output: "    fail_test.go:10: error message\n", OutputType: OutputTypeError, Time: time.Now()},
		{Action: ActionOutput, Package: "pkg", Test: "TestFail", Output: "        error details continuation\n", OutputType: OutputTypeErrorContinue, Time: time.Now()},
		{Action: ActionFail, Package: "pkg", Test: "TestFail", Elapsed: 0.05, Time: time.Now()},
	}

	r.PrintDetail(events, Flags{V: V0})
	got := sb.String()

	if !strings.Contains(got, "=== FAIL pkg.TestFail") {
		t.Errorf("expected FAIL header, got: %s", got)
	}
	if !strings.Contains(got, "debug log") {
		t.Errorf("expected debug log in output, got: %s", got)
	}
	if !strings.Contains(got, "error message") {
		t.Errorf("expected error message in output, got: %s", got)
	}
	if !strings.Contains(got, "error details continuation") {
		t.Errorf("expected continuation line in output, got: %s", got)
	}
}

func TestEvents_Status_MultipleRuns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		events Events
		want   Status
	}{
		{"PassThenFail", Events{{Action: ActionPass}, {Action: ActionFail}}, StatusFail},
		{"FailThenPass", Events{{Action: ActionFail}, {Action: ActionPass}}, StatusFail},
		{"PassThenPass", Events{{Action: ActionPass}, {Action: ActionPass}}, StatusPass},
		{"SkipThenFail", Events{{Action: ActionSkip}, {Action: ActionFail}}, StatusFail},
		{"FailThenBuildFail", Events{{Action: ActionFail}, {Action: ActionBuildFail}}, StatusBuildFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.events.Status(); got != tt.want {
				t.Errorf("Status() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestTestedPackage(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"ex/a", "ex/a"},
		{"ex/a [ex/a.test]", "ex/a"},
		{"ex/a_test [ex/a.test]", "ex/a"},
		{"ex/dep [ex/a.test]", "ex/a"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := testedPackage(tt.in); got != tt.want {
			t.Errorf("testedPackage(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTestStorage_Append_TestVariantBuildFail(t *testing.T) {
	t.Parallel()
	ts := make(TestStorage)
	ts.Append(Event{ImportPath: "ex/a_test [ex/a.test]", Action: ActionBuildOutput, Output: "undefined: x\n"})
	ts.Append(Event{ImportPath: "ex/a_test [ex/a.test]", Action: ActionBuildFail})
	ts.Append(Event{Package: "ex/a", Action: ActionStart})
	ts.Append(Event{Package: "ex/a", Action: ActionFail, FailedBuild: "ex/a_test [ex/a.test]"})

	if len(ts) != 1 {
		t.Fatalf("expected events under a single key, got keys %v", ts.OrderedKeys())
	}
	if got := ts.Stats().BuildFail; got != 1 {
		t.Errorf("BuildFail = %d, want 1", got)
	}
}

func TestTestStorage_OrderedKeys_Order(t *testing.T) {
	t.Parallel()
	want := []Key{
		{"a", "TestX"},
		{"a", "TestX/sub"},
		{"a", ""},
		{"a-b", "Test2"},
		{"a-b", "Test10"},
		{"a-b", ""},
		{"a/c", "TestZ"},
		{"a/c", ""},
	}
	ts := make(TestStorage)
	for _, k := range want {
		ts[k] = nil
	}
	// Map iteration order varies; with a consistent comparator the result
	// must be the same every time.
	for range 100 {
		if got := ts.OrderedKeys(); !slices.Equal(got, want) {
			t.Fatalf("OrderedKeys() = %v, want %v", got, want)
		}
	}
}

func TestTestStorage_FinishBenchmarks(t *testing.T) {
	t.Parallel()
	newTS := func() TestStorage {
		ts := make(TestStorage)
		ts.Append(Event{Package: "p", Test: "TestA", Action: ActionPass})
		ts.Append(Event{Package: "p", Test: "BenchmarkA", Action: ActionRun})
		ts.Append(Event{Package: "p", Test: "BenchmarkB", Action: ActionRun})
		ts.Append(Event{Package: "p", Test: "BenchmarkB/sub", Action: ActionRun})
		ts.Append(Event{Package: "q", Test: "BenchmarkQ", Action: ActionRun})
		return ts
	}

	t.Run("PackagePassed", func(t *testing.T) {
		t.Parallel()
		ts := newTS()
		got := ts.FinishBenchmarks(Event{Package: "p", Action: ActionPass}, "BenchmarkB/sub")
		want := []Key{{"p", "BenchmarkA"}, {"p", "BenchmarkB"}, {"p", "BenchmarkB/sub"}}
		if !slices.Equal(got, want) {
			t.Errorf("FinishBenchmarks() = %v, want %v", got, want)
		}
		for _, k := range want {
			if s := ts[k].Status(); s != StatusBench {
				t.Errorf("%v status = %s, want %s", k, s, StatusBench)
			}
		}
		if s := ts[Key{"q", "BenchmarkQ"}].Status(); s != StatusNone {
			t.Errorf("other package benchmark status = %s, want %s", s, StatusNone)
		}
	})

	t.Run("PackageCrashed", func(t *testing.T) {
		t.Parallel()
		ts := newTS()
		got := ts.FinishBenchmarks(Event{Package: "p", Action: ActionFail}, "BenchmarkB/sub")
		want := []Key{{"p", "BenchmarkA"}}
		if !slices.Equal(got, want) {
			t.Errorf("FinishBenchmarks() = %v, want %v", got, want)
		}
		for _, k := range []Key{{"p", "BenchmarkB"}, {"p", "BenchmarkB/sub"}} {
			if s := ts[k].Status(); s != StatusNone {
				t.Errorf("%v status = %s, want %s", k, s, StatusNone)
			}
		}
	})
}

func TestEvents_FindCoverage_Formats(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		output string
		want   string
		noTest bool
	}{
		{"Cover", "coverage: 66.7% of statements\n", "66.7%", false},
		{"CoverPkg", "coverage: 66.7% of statements in ./...\n", "66.7%", false},
		{"NoTestFiles", "\tex/p\t\tcoverage: 0.0% of statements\n", "0.0%", true},
		{"NoStatements", "coverage: [no statements]\n", "", false},
		{"OtherText", "coverage: 66.7% of statements, roughly\n", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			es := Events{{Action: ActionOutput, Package: "ex/p", Output: tt.output}}
			if got := es.FindCoverage(); got != tt.want {
				t.Errorf("FindCoverage() = %q, want %q", got, tt.want)
			}
			if got := es.IsPackageWithoutTest(); got != tt.noTest {
				t.Errorf("IsPackageWithoutTest() = %v, want %v", got, tt.noTest)
			}
			wantIgnored := tt.name != "OtherText"
			if got := isIgnoredEvent(es[0], 0, 0, 0); got != wantIgnored {
				t.Errorf("isIgnoredEvent() = %v, want %v", got, wantIgnored)
			}
		})
	}

	t.Run("TestOutputIgnored", func(t *testing.T) {
		t.Parallel()
		es := Events{{Action: ActionOutput, Package: "ex/p", Test: "TestX", Output: "coverage: 50.0% of statements\n"}}
		if got := es.FindCoverage(); got != "" {
			t.Errorf("FindCoverage() = %q for test output, want \"\"", got)
		}
	})
}

func TestEvent_AttrArtifacts(t *testing.T) {
	t.Parallel()
	input := `{"Action":"run","Package":"ex/a","Test":"TestA"}
{"Action":"attr","Package":"ex/a","Test":"TestA","Key":"issue","Value":"123"}
{"Action":"output","Package":"ex/a","Test":"TestA","Output":"=== ATTR  TestA issue 123\n","OutputType":"frame"}
{"Action":"artifacts","Package":"ex/a","Test":"TestA","Path":"/tmp/_artifacts/a/TestA/1"}
{"Action":"output","Package":"ex/a","Test":"TestA","Output":"=== ARTIFACTS TestA /tmp/_artifacts/a/TestA/1\n","OutputType":"frame"}
{"Action":"fail","Package":"ex/a","Test":"TestA"}
`
	var e Event
	if err := json.Unmarshal([]byte(strings.SplitN(input, "\n", 3)[1]), &e); err != nil {
		t.Fatal(err)
	}
	if e.AttrKey != "issue" || e.AttrValue != "123" {
		t.Errorf("attr event decoded as Key=%q Value=%q", e.AttrKey, e.AttrValue)
	}

	var sb strings.Builder
	flags := Flags{Results: Statuses{StatusFail}}
	if err := processEvents(strings.NewReader(input), flags, NewRenderer(&sb), time.Now()); err != nil {
		t.Fatal(err)
	}
	got := normalizeOutput(sb.String())
	want := `*****
=== FAIL ex/a.TestA

attr: issue=123
artifacts: /tmp/_artifacts/a/TestA/1


══════ 12:00:00 | PASS:0 | FAIL:1 | BUILD FAIL:0 | NONE:0 | SKIP:0 | 0ms  ══════`
	if got != want {
		t.Errorf("output mismatch:\n--- GOT ---\n%s\n--- WANT ---\n%s", got, want)
	}
}

func TestListRequested(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"./..."}, false},
		{[]string{"-list", "."}, true},
		{[]string{"-list=Test"}, true},
		{[]string{"--list", "."}, true},
		{[]string{"-test.list=."}, true},
		{[]string{"-run", "list"}, false},
		{[]string{"./...", "-args", "-list", "."}, false},
	}
	for _, tt := range tests {
		if got := listRequested(tt.args); got != tt.want {
			t.Errorf("listRequested(%q) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestParseFlag(t *testing.T) {
	t.Parallel()
	type result struct {
		name, value     string
		hasValue, isArg bool
	}
	tests := []struct {
		arg  string
		want result
	}{
		{"-v", result{"v", "", false, true}},
		{"--v", result{"v", "", false, true}},
		{"-run=TestA", result{"run", "TestA", true, true}},
		{"-run=a=b", result{"run", "a=b", true, true}},
		{"-cover=", result{"cover", "", true, true}},
		{"./...", result{}},
		{"TestA", result{}},
	}
	for _, tt := range tests {
		name, value, hasValue, ok := parseFlag(tt.arg)
		if got := (result{name, value, hasValue, ok}); got != tt.want {
			t.Errorf("parseFlag(%q) = %+v, want %+v", tt.arg, got, tt.want)
		}
	}
}

func TestRenderer_NoTrailingWhitespace(t *testing.T) {
	t.Parallel()
	input := `{"Action":"run","Package":"ex/a","Test":"TestA"}
{"Action":"output","Package":"ex/a","Test":"TestA","Output":"    a_test.go:5: log\n"}
{"Action":"output","Package":"ex/a","Test":"TestA","Output":"    a_test.go:6: boom\n","OutputType":"error"}
{"Action":"fail","Package":"ex/a","Test":"TestA"}
{"Action":"fail","Package":"ex/a"}
`
	for _, v := range []Verbosity{V0, V3, V4} {
		var sb strings.Builder
		flags := Flags{V: v, Results: Statuses{StatusFail}, Summary: Statuses{StatusFail}}
		if err := processEvents(strings.NewReader(input), flags, NewRenderer(&sb), time.Now()); err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(sb.String(), "\n") {
			if strings.TrimRight(line, " \t") != line {
				t.Errorf("V%d: line %d has trailing whitespace: %q", v, i+1, line)
			}
		}
	}
}

func TestRenderer_SummaryPrefixAligned(t *testing.T) {
	t.Parallel()
	ts := make(TestStorage)
	ts.Append(Event{Package: "ex/a", Action: ActionFail})
	ts.Append(Event{Package: "ex/b", Action: ActionFail, FailedBuild: "ex/b"})

	var sb strings.Builder
	r := NewRenderer(&sb)
	r.PrintSummary(ts.FindByStatus(StatusFail), StatusFail)
	r.PrintSummary(ts.FindByStatus(StatusBuildFail), StatusBuildFail)

	columns := map[string]int{}
	for _, line := range strings.Split(sb.String(), "\n") {
		for _, pkg := range []string{"ex/a", "ex/b"} {
			if i := strings.Index(line, pkg); i >= 0 {
				columns[pkg] = i
			}
		}
	}
	if len(columns) != 2 || columns["ex/a"] != columns["ex/b"] {
		t.Errorf("package names not aligned: %v\n%s", columns, sb.String())
	}
}

func TestTestStorage_Stats_None(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		events []Event
		want   int
	}{
		{"InterruptedTest", []Event{
			{Package: "p", Action: ActionStart},
			{Package: "p", Test: "TestA", Action: ActionRun},
		}, 1},
		{"InterruptedBeforeTests", []Event{
			{Package: "p", Action: ActionStart},
		}, 1},
		{"TwoTests", []Event{
			{Package: "p", Test: "TestA", Action: ActionRun},
			{Package: "p", Test: "TestB", Action: ActionRun},
		}, 2},
		{"CrashedPackageFinished", []Event{
			{Package: "p", Test: "TestA", Action: ActionRun},
			{Package: "p", Action: ActionFail},
		}, 1},
		{"Finished", []Event{
			{Package: "p", Test: "TestA", Action: ActionPass},
			{Package: "p", Action: ActionPass},
		}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts := make(TestStorage)
			for _, e := range tt.events {
				ts.Append(e)
			}
			if got := ts.Stats().None; got != tt.want {
				t.Errorf("Stats().None = %d, want %d", got, tt.want)
			}
		})
	}
}
