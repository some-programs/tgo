package main

import (
	"encoding/json"
	"flag"
	"strings"
	"testing"
	"time"
)

func TestFlags_Register(t *testing.T) {
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
	var f Flags
	var sb strings.Builder
	f.PrintHelp(&sb)
	if sb.Len() == 0 {
		t.Error("PrintHelp produced no output")
	}
}

func TestFlags_PrintConfig(t *testing.T) {
	var f Flags
	f.Results = Statuses{StatusPass}
	var sb strings.Builder
	f.printConfig(&sb)
	if !strings.Contains(sb.String(), "TGO_RESULTS: pass") {
		t.Errorf("printConfig output missing expected content: %s", sb.String())
	}
}

func TestFlags_Setup(t *testing.T) {
	t.Run("All", func(t *testing.T) {
		f := Flags{All: true}
		f.Setup(nil)
		if len(f.Results) != len(AllStatuses) {
			t.Error("expected all statuses in Results")
		}
	})

	t.Run("V2", func(t *testing.T) {
		f := Flags{}
		f.Setup([]string{"-v"})
		if f.V != V2 {
			t.Errorf("expected V2, got %v", f.V)
		}
	})
}

func TestAction_Methods(t *testing.T) {
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
	if StatusPass.String() != "pass" {
		t.Errorf("expected 'pass', got %s", StatusPass.String())
	}
}

func TestOutputType_Methods(t *testing.T) {
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
	ss := Statuses{StatusPass, StatusFail}

	t.Run("Any", func(t *testing.T) {
		if !ss.Any(StatusPass) {
			t.Error("expected Any(StatusPass) to be true")
		}
		if ss.Any(StatusSkip) {
			t.Error("expected Any(StatusSkip) to be false")
		}
	})

	t.Run("String", func(t *testing.T) {
		if ss.String() != "pass,fail" {
			t.Errorf("expected 'pass,fail', got %s", ss.String())
		}
	})

	t.Run("Set", func(t *testing.T) {
		var sss Statuses
		if err := sss.Set("pass,skip"); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		if len(sss) != 2 || sss[0] != StatusPass || sss[1] != StatusSkip {
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
	es := Events{
		{Action: ActionRun, Time: time.Now()},
		{Action: ActionOutput, Output: "foo\n"},
		{Action: ActionPass, Elapsed: 0.5},
	}

	t.Run("Clone", func(t *testing.T) {
		cloned := es.Clone()
		if len(cloned) != len(es) {
			t.Error("clone length mismatch")
		}
		if cloned[0].Action != es[0].Action {
			t.Error("clone content mismatch")
		}
	})

	t.Run("Status", func(t *testing.T) {
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
		e := es.FindFirstByAction(ActionPass)
		if e == nil || e.Action != ActionPass {
			t.Error("failed to find ActionPass")
		}
		if es.FindFirstByAction(ActionFail) != nil {
			t.Error("found non-existent action")
		}
	})

	t.Run("SortByTime", func(t *testing.T) {
		now := time.Now()
		es2 := Events{
			{Time: now.Add(time.Second)},
			{Time: now},
		}
		es2.SortByTime()
		if !es2[0].Time.Before(es2[1].Time) {
			t.Error("events not sorted by time")
		}
	})

	t.Run("Compact", func(t *testing.T) {
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
		es4 := Events{
			{Action: ActionOutput, Package: "pkg", Output: "ok  	pkg [no test files]\n"},
		}
		if !es4.IsPackageWithoutTest() {
			t.Error("expected IsPackageWithoutTest to be true")
		}
	})

	t.Run("FindCoverage", func(t *testing.T) {
		es5 := Events{
			{Package: "pkg", Action: ActionOutput, Output: "coverage: 50.0% of statements\n"},
		}
		if es5.FindCoverage() != "50.0%" {
			t.Errorf("expected 50.0%%, got %s", es5.FindCoverage())
		}
	})
}

func TestTestStorage_Methods(t *testing.T) {
	ts := make(TestStorage)
	ts.Append(Event{Package: "pkg", Test: "test1", Action: ActionPass})
	ts.Append(Event{Package: "pkg", Test: "test2", Action: ActionFail})
	ts.Append(Event{Package: "pkg", Action: ActionPass})

	t.Run("Append_FailedBuild", func(t *testing.T) {
		storage := make(TestStorage)
		storage.Append(Event{FailedBuild: "build/fail/pkg", Action: ActionFail})
		if _, ok := storage[Key{Package: "build/fail/pkg"}]; !ok {
			t.Errorf("expected storage to have key with package 'build/fail/pkg'")
		}
	})

	t.Run("Append_ImportPath", func(t *testing.T) {
		storage := make(TestStorage)
		storage.Append(Event{ImportPath: "import/pkg", Action: ActionBuildOutput})
		events, ok := storage[Key{Package: "import/pkg"}]
		if !ok || len(events) != 1 || events[0].Action != ActionOutput {
			t.Errorf("expected storage to normalize ActionBuildOutput to ActionOutput and package 'import/pkg'")
		}
	})

	t.Run("OrderedKeys", func(t *testing.T) {
		keys := ts.OrderedKeys()
		if len(keys) != 3 {
			t.Fatalf("expected 3 keys, got %d", len(keys))
		}
	})

	t.Run("FilterPackageResults", func(t *testing.T) {
		filtered := ts.FilterPackageResults()
		if len(filtered) != 2 {
			t.Error("expected 2 test results")
		}
	})

	t.Run("FindPackageResults", func(t *testing.T) {
		filtered := ts.FindPackageResults()
		if len(filtered) != 1 {
			t.Error("expected 1 package result")
		}
	})

	t.Run("CountTests", func(t *testing.T) {
		if ts.CountTests() != 2 {
			t.Errorf("expected 2 tests, got %d", ts.CountTests())
		}
	})

	t.Run("Union", func(t *testing.T) {
		ts2 := make(TestStorage)
		ts2.Append(Event{Package: "pkg2", Action: ActionPass})
		union := ts.Union(ts2)
		if len(union) != 4 {
			t.Errorf("union failed, expected 4 results, got %d", len(union))
		}
	})

	t.Run("FilterKeys", func(t *testing.T) {
		exclude := map[Key]bool{{Package: "pkg", Test: "test1"}: true}
		filtered := ts.FilterKeys(exclude)
		if len(filtered) != 2 {
			t.Error("FilterKeys failed")
		}
	})

	t.Run("FindPackageTests", func(t *testing.T) {
		pkgTests := ts.FindPackageTests("pkg")
		if len(pkgTests) != 3 {
			t.Error("FindPackageTests failed")
		}
	})

	t.Run("FindByAction", func(t *testing.T) {
		failed := ts.FindByAction(ActionFail)
		if len(failed) != 1 {
			t.Error("FindByAction failed")
		}
	})

	t.Run("FilterAction", func(t *testing.T) {
		noFail := ts.FilterAction(ActionFail)
		if len(noFail) != 2 {
			t.Error("FilterAction failed")
		}
	})

	t.Run("WithCoverage", func(t *testing.T) {
		ts.Append(Event{Package: "pkg_cov", Action: ActionOutput, Output: "coverage: 10.0% of statements\n"})
		cov := ts.WithCoverage()
		if len(cov) != 1 {
			t.Errorf("expected 1 coverage result, got %d", len(cov))
		}
	})

	t.Run("FilterNotests", func(t *testing.T) {
		ts.Append(Event{Package: "pkg_no", Action: ActionOutput, Output: "ok  	pkg_no [no test files]\n"})
		filtered := ts.FilterNotests()
		if len(filtered) != 4 {
			t.Errorf("expected 4 results, got %d", len(filtered))
		}
	})

	t.Run("Filter", func(t *testing.T) {
		filtered := ts.Filter(func(k Key, _ Events) bool {
			return k.Test == "test1"
		})
		if len(filtered) != 1 {
			t.Errorf("expected 1 result, got %d", len(filtered))
		}
	})

	t.Run("FindByStatus", func(t *testing.T) {
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
		stats := ts.Stats()
		if stats.Pass != 1 || stats.Fail != 1 || stats.None != 2 {
			t.Errorf("unexpected stats: %+v", stats)
		}
	})
}

// captureStdout is a test helper that redirects defaultRenderer output and returns its content as a string.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
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
	ts := make(TestStorage)
	ts.Append(Event{Package: "pkg", Action: ActionPass, Elapsed: 0.1})
	ts.Append(Event{Package: "pkg", Test: "Test1", Action: ActionPass, Elapsed: 0.1})
	ts.Append(Event{Package: "pkg_cov", Action: ActionOutput, Output: "coverage: 50.0% of statements\n"})

	t.Run("PrintShortSummary", func(t *testing.T) {
		got := captureStdout(t, func() {
			ts.PrintShortSummary(StatusPass)
		})
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg") {
			t.Errorf("PrintShortSummary output missing expected content: %q", got)
		}
	})

	t.Run("PrintSummary", func(t *testing.T) {
		got := captureStdout(t, func() {
			ts.PrintSummary(StatusPass)
		})
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg.Test1") {
			t.Errorf("PrintSummary output missing expected content: %q", got)
		}
	})

	t.Run("PrintCoverage", func(t *testing.T) {
		got := captureStdout(t, func() {
			ts.PrintCoverage()
		})
		if !strings.Contains(got, "COVR") || !strings.Contains(got, "50.0%") {
			t.Errorf("PrintCoverage output missing expected content: %q", got)
		}
	})

	t.Run("PrintDetail", func(t *testing.T) {
		events := ts[Key{Package: "pkg", Test: "Test1"}]
		got := captureStdout(t, func() {
			events.PrintDetail(Flags{V: V0})
		})
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg") || !strings.Contains(got, "Test1") {
			t.Errorf("PrintDetail output missing expected content: %q", got)
		}
	})
}

func TestRenderer_DirectWriter(t *testing.T) {
	ts := make(TestStorage)
	ts.Append(Event{Package: "pkg", Action: ActionPass, Elapsed: 0.1})
	ts.Append(Event{Package: "pkg", Test: "Test1", Action: ActionPass, Elapsed: 0.1})
	ts.Append(Event{Package: "pkg_cov", Action: ActionOutput, Output: "coverage: 50.0% of statements\n"})

	t.Run("PrintShortSummary", func(t *testing.T) {
		var sb strings.Builder
		r := NewRenderer(&sb)
		r.PrintShortSummary(ts, StatusPass)
		got := sb.String()
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg") {
			t.Errorf("PrintShortSummary output missing expected content: %q", got)
		}
	})

	t.Run("PrintSummary", func(t *testing.T) {
		var sb strings.Builder
		r := NewRenderer(&sb)
		r.PrintSummary(ts, StatusPass)
		got := sb.String()
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg.Test1") {
			t.Errorf("PrintSummary output missing expected content: %q", got)
		}
	})

	t.Run("PrintCoverage", func(t *testing.T) {
		var sb strings.Builder
		r := NewRenderer(&sb)
		r.PrintCoverage(ts)
		got := sb.String()
		if !strings.Contains(got, "COVR") || !strings.Contains(got, "50.0%") {
			t.Errorf("PrintCoverage output missing expected content: %q", got)
		}
	})

	t.Run("PrintDetail", func(t *testing.T) {
		var sb strings.Builder
		r := NewRenderer(&sb)
		events := ts[Key{Package: "pkg", Test: "Test1"}]
		r.PrintDetail(events, Flags{V: V0})
		got := sb.String()
		if !strings.Contains(got, "PASS") || !strings.Contains(got, "pkg") || !strings.Contains(got, "Test1") {
			t.Errorf("PrintDetail output missing expected content: %q", got)
		}
	})

	t.Run("PrintFooter", func(t *testing.T) {
		var sb strings.Builder
		r := NewRenderer(&sb)
		r.PrintFooter(ts, time.Now())
		got := sb.String()
		if !strings.Contains(got, "PASS:1") {
			t.Errorf("PrintFooter output missing expected content: %q", got)
		}
	})

	t.Run("PrintHeader", func(t *testing.T) {
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
	input := `{"Time":"2026-08-19T10:00:00Z","Action":"run","Package":"my/pkg","Test":"TestFoo"}
{"Time":"2026-08-19T10:00:01Z","Action":"pass","Package":"my/pkg","Test":"TestFoo","Elapsed":0.05}
`
	var sb strings.Builder
	r := NewRenderer(&sb)
	flags := Flags{
		Results: Statuses{StatusPass},
		Summary: Statuses{StatusPass},
	}
	err := processEvents(strings.NewReader(input), flags, r, false, time.Now())
	if err != nil {
		t.Fatalf("processEvents returned error: %v", err)
	}
	got := sb.String()
	if !strings.Contains(got, "my/pkg.TestFoo") {
		t.Errorf("expected test name in output, got: %s", got)
	}
}

func TestRenderer_EventTextColor(t *testing.T) {
	r := NewRenderer(nil)

	t.Run("with OutputType", func(t *testing.T) {
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
		plainEv := Event{OutputType: OutputTypeBlank, Action: ActionOutput}
		fn := r.eventTextColor(plainEv, StatusFail, false, failColor)
		if fn == nil {
			t.Fatal("expected fallback color function")
		}
	})
}

func TestRenderer_PrintDetail_OutputTypes(t *testing.T) {
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
