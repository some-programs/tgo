package main

import (
	"flag"
	"io"
	"os"
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

// captureStdout is a test helper that intercepts os.Stdout and returns its content as a string.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var sb strings.Builder
	_, err = io.Copy(&sb, r)
	if err != nil {
		t.Fatalf("failed to copy output: %v", err)
	}
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
