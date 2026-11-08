package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/fatih/color"
)

var (
	defaultColor  = color.New().SprintFunc()
	hardLineColor = color.New().SprintFunc()
	packageColor  = color.New().SprintFunc()
	testColor     = color.New(color.FgMagenta).SprintFunc()
	testColorBold = color.New(color.FgMagenta, color.Bold).SprintFunc()
	timeColor     = color.New(color.FgCyan).SprintFunc()
	coverColor    = color.New(color.FgBlue).SprintFunc()
	attrColor     = color.New(color.FgCyan).SprintFunc()

	failColor     = color.New(color.FgRed).SprintFunc()
	failColorBold = color.New(color.FgRed, color.Bold).SprintFunc()

	noneColor     = color.New(color.FgYellow).SprintFunc()
	noneColorBold = color.New(color.FgYellow, color.Bold).SprintFunc()

	passColor     = color.New(color.FgGreen).SprintFunc()
	passColorBold = color.New(color.FgGreen, color.Bold).SprintFunc()

	skipColor     = color.New(color.FgHiMagenta).SprintFunc()
	skipColorBold = color.New(color.FgHiMagenta, color.Bold).SprintFunc()

	statusColors = map[Status](func(a ...any) string){
		StatusFail:      failColor,
		StatusPass:      passColor,
		StatusNone:      noneColor,
		StatusSkip:      skipColor,
		StatusBench:     passColor,
		StatusBuildFail: failColor,
	}

	statusColorsBold = map[Status](func(a ...any) string){
		StatusFail:      failColorBold,
		StatusPass:      passColorBold,
		StatusNone:      noneColorBold,
		StatusSkip:      skipColorBold,
		StatusBench:     passColorBold,
		StatusBuildFail: failColorBold,
	}
)

// Renderer formats and prints test events and summaries to an io.Writer.
type Renderer struct {
	w io.Writer
}

// NewRenderer creates a Renderer targeting the provided io.Writer.
func NewRenderer(w io.Writer) *Renderer {
	return &Renderer{w: w}
}

func (r *Renderer) out() io.Writer {
	if r == nil || r.w == nil {
		return os.Stdout
	}
	return r.w
}

var defaultRenderer = &Renderer{}

// PrintHeader outputs the initial delimiter banner before test execution output.
func (r *Renderer) PrintHeader() {
	fmt.Fprintln(r.out(), "*****")
}

// statusNameWidth is the length of the longest status name, so that the
// status prefixes of summary lines line up across sections.
var statusNameWidth = func() int {
	width := 0
	for _, name := range statusNames {
		width = max(width, len(name))
	}
	return width
}()

func (r *Renderer) statusHeader(status Status) (hr, header, prefix string) {
	statusColor := statusColors[status]
	statusBold := statusColorsBold[status]
	header = statusBold(statusNames[status])
	hr = statusColor("════════════")
	prefix = statusColor(fmt.Sprintf("%*s ", statusNameWidth, statusNames[status]))
	return hr, header, prefix
}

func (r *Renderer) formatBadges(events Events, elapsed float64) string {
	var sb strings.Builder
	if elapsed >= 0.01 {
		sb.WriteString("  ")
		sb.WriteString(timeColor(fmt.Sprintf("(%.2fs)", elapsed)))
	}
	if events.IsPackageWithoutTest() {
		sb.WriteString("  ")
		sb.WriteString("[no tests]")
	}
	if coverage := events.FindCoverage(); len(coverage) > 0 {
		sb.WriteString("  ")
		sb.WriteString(coverColor(fmt.Sprintf("{%s}", coverage)))
	}
	return sb.String()
}

// PrintDetail renders detailed event information for a single test / package.
func (r *Renderer) PrintDetail(events Events, flags Flags) {
	if len(events) == 0 {
		return
	}
	es := events.Clone()
	if flags.V <= V3 {
		es = es.Compact()
	}
	if len(es) == 0 {
		return
	}

	var filteredEvents Events
	for _, e := range es {
		if flags.V <= V3 && strings.TrimSpace(e.Text()) == "" {
			continue
		}
		filteredEvents = append(filteredEvents, e)
	}

	status := es.Status()
	numberEvents := len(filteredEvents)
	if numberEvents == 0 && flags.HideEmptyResults.Any(status) {
		return
	}
	textColor := defaultColor
	var event *Event
	switch status {
	case StatusFail:
		event = es.FindFirstByAction(ActionFail)
		textColor = failColor
	case StatusBuildFail:
		event = es.FindFirstByAction(ActionBuildFail)
		textColor = failColor
	case StatusPass:
		event = es.FindFirstByAction(ActionPass)
	case StatusSkip:
		event = es.FindFirstByAction(ActionSkip)
		textColor = skipColor
	case StatusBench:
		event = es.FindFirstByAction(ActionBench)
	}

	if event == nil {
		event = &es[0]
	}

	var testName string
	if event.Test != "" {
		c := testColor
		if numberEvents > 0 {
			c = testColorBold
		}
		testName = "." + c(event.Test)
	}

	badges := r.formatBadges(events, event.Elapsed)

	statusColor := statusColors[status]
	statusBold := statusColorsBold[status]
	w := r.out()
	fmt.Fprintf(w, "%s %s %s%s%s\n",
		statusBold("==="),
		statusBold(statusNames[status]),
		statusColor(event.Package),
		testName,
		badges,
	)
	if len(filteredEvents) > 0 {
		fmt.Fprintln(w, "")
	}
	hasOutputType := slices.ContainsFunc(filteredEvents, func(e Event) bool {
		return e.OutputType != OutputTypeBlank
	})
	for _, e := range filteredEvents {
		var ss []string
		if flags.V >= V3 {
			ss = append(ss, fmt.Sprintf("%7s", e.Action), e.Time.Format("15:04:05.999"))
		}
		if text := strings.TrimSuffix(e.Text(), "\n"); text != "" {
			colorFn := r.eventTextColor(e, status, hasOutputType, textColor)
			ss = append(ss, colorFn(text))
		}
		fmt.Fprintln(w, strings.Join(ss, " "))
	}
	if len(filteredEvents) > 0 {
		fmt.Fprintln(w, "")
	}
}

func (r *Renderer) eventTextColor(e Event, status Status, hasOutputType bool, fallbackColor func(a ...any) string) func(a ...any) string {
	if e.Action == ActionAttr || e.Action == ActionArtifacts {
		return attrColor
	}
	if hasOutputType {
		switch {
		case e.IsErrorHeader():
			return failColorBold
		case e.IsErrorContinue():
			return failColor
		case status == StatusSkip:
			return skipColor
		default:
			return defaultColor
		}
	}
	return fallbackColor
}

// PrintSummary prints all tests and packages for a given status.
func (r *Renderer) PrintSummary(ts TestStorage, status Status) {
	hr, header, prefix := r.statusHeader(status)

	w := r.out()
	fmt.Fprintln(w, hr, header, hr)
	for _, key := range ts.OrderedKeys() {
		events := ts[key]

		var elapsed float64
		if fe := events.FindFirstByAction(EndingActions...); fe != nil {
			elapsed = fe.Elapsed
		}
		badges := r.formatBadges(events, elapsed)

		if key.Test == "" {
			fmt.Fprintf(w, "%s%s%s\n", prefix, packageColor(key.Package), badges)
		} else {
			fmt.Fprintf(w, "%s%s.%s%s\n", prefix, packageColor(key.Package), testColor(key.Test), badges)
		}
	}
}

// PrintCoverage prints code coverage report for packages.
func (r *Renderer) PrintCoverage(ts TestStorage) {
	hr := coverColor("════════════")

	w := r.out()
	fmt.Fprintln(w, hr, coverColor("COVR"), hr)
	for _, key := range ts.OrderedKeys() {
		events := ts[key]
		if key.Test == "" {
			coverage := events.FindCoverage()
			if len(coverage) > 0 {
				coverage = fmt.Sprintf("%6s ", coverage)
			}
			fmt.Fprintf(w, "%s%s\n", coverColor(coverage), packageColor(key.Package))
		}
	}
}

// PrintFooter prints the summary banner bar at the end of test run.
func (r *Renderer) PrintFooter(tests TestStorage, t0 time.Time) {
	stats := tests.Stats()

	pass := statusNames[StatusPass] + ":" + fmt.Sprint(stats.Pass)
	fail := statusNames[StatusFail] + ":" + fmt.Sprint(stats.Fail)
	buildfail := statusNames[StatusBuildFail] + ":" + fmt.Sprint(stats.BuildFail)
	none := statusNames[StatusNone] + ":" + fmt.Sprint(stats.None)
	skip := statusNames[StatusSkip] + ":" + fmt.Sprint(stats.Skip)

	statusColor := hardLineColor

	if stats.Pass > 0 {
		statusColor = passColorBold
		pass = statusColor(pass)
	}

	if stats.None > 0 {
		statusColor = noneColorBold
		none = statusColor(none)
	}

	if stats.Fail > 0 || stats.BuildFail > 0 {
		statusColor = failColorBold
		fail = statusColor(fail)
		buildfail = statusColor(buildfail)
	}

	w := r.out()
	fmt.Fprintln(w, "")
	now := time.Now()
	sep := " " + statusColor("|") + " "
	status := statusColor("══════") + " " +
		statusColor(now.Format("15:04:05")) +
		sep + pass +
		sep + fail +
		sep + buildfail +
		sep + none +
		sep + skip +
		sep + statusColor(now.Sub(t0).Round(time.Millisecond).String()) +
		"  " + statusColor("══════")

	fmt.Fprintln(w, status)
}

// PrintModuleHeader prints the banner before running tests in a module directory.
func (r *Renderer) PrintModuleHeader(dir string) {
	w := r.out()
	fmt.Fprintf(w, "\n%s %s\n", hardLineColor("--->"), attrColor("["+dir+"]"))
}

// PrintRecursiveSummary renders the aggregated failure summary and final footer for a recursive run.
func (r *Renderer) PrintRecursiveSummary(results []ModuleRunResult, skippedCount int, t0 time.Time) {
	w := r.out()

	var failedResults []ModuleRunResult
	var totalStats TestStats

	for _, res := range results {
		stats := res.Storage.Stats()
		totalStats.Pass += stats.Pass
		totalStats.Fail += stats.Fail
		totalStats.BuildFail += stats.BuildFail
		totalStats.None += stats.None
		totalStats.Skip += stats.Skip
		if res.Failed() {
			failedResults = append(failedResults, res)
		}
	}

	totalModules := len(results) + skippedCount
	failedModules := len(failedResults)
	passedModules := len(results) - failedModules

	if failedModules > 0 {
		statusColor := statusColors[StatusFail]
		statusBold := statusColorsBold[StatusFail]
		hr := statusColor("════════════")
		headerText := fmt.Sprintf("FAILURES (%d of %d modules failed)", failedModules, totalModules)
		header := statusBold(headerText)
		prefix := statusColor(fmt.Sprintf("%*s ", statusNameWidth, statusNames[StatusFail]))

		fmt.Fprintln(w, hr, header, hr)

		for _, res := range failedResults {
			modFailures := make(TestStorage)
			for _, st := range []Status{StatusFail, StatusBuildFail} {
				for k, v := range res.Storage.FindByStatus(st) {
					modFailures[k] = v
				}
			}

			if len(modFailures) == 0 {
				errStr := "failed"
				if res.Err != nil {
					errStr = res.Err.Error()
				}
				fmt.Fprintf(w, "%s%s %s\n", prefix, attrColor("["+res.Dir+"]"), failColor(errStr))
			} else {
				for _, key := range modFailures.OrderedKeys() {
					events := modFailures[key]
					var elapsed float64
					if fe := events.FindFirstByAction(EndingActions...); fe != nil {
						elapsed = fe.Elapsed
					}
					badges := r.formatBadges(events, elapsed)

					if key.Test == "" {
						fmt.Fprintf(w, "%s%s %s%s\n", prefix, attrColor("["+res.Dir+"]"), packageColor(key.Package), badges)
					} else {
						fmt.Fprintf(w, "%s%s %s.%s%s\n", prefix, attrColor("["+res.Dir+"]"), packageColor(key.Package), testColor(key.Test), badges)
					}
				}
			}
		}
	}

	// Print aggregated coverage for all modules if any coverage data was recorded
	var allCovered []struct {
		dir      string
		coverage string
		pkg      string
	}
	for _, res := range results {
		covered := res.Storage.WithCoverage()
		for _, key := range covered.OrderedKeys() {
			events := covered[key]
			if key.Test == "" {
				cov := events.FindCoverage()
				if len(cov) > 0 {
					cov = fmt.Sprintf("%6s ", cov)
				}
				allCovered = append(allCovered, struct {
					dir      string
					coverage string
					pkg      string
				}{
					dir:      res.Dir,
					coverage: cov,
					pkg:      key.Package,
				})
			}
		}
	}

	if len(allCovered) > 0 {
		hr := coverColor("════════════")
		fmt.Fprintln(w, hr, coverColor("COVR"), hr)
		for _, item := range allCovered {
			fmt.Fprintf(w, "%s%s %s\n", coverColor(item.coverage), attrColor("["+item.dir+"]"), packageColor(item.pkg))
		}
	}

	r.printRecursiveFooter(totalStats, passedModules, totalModules, skippedCount, t0)
}

func (r *Renderer) printRecursiveFooter(stats TestStats, passedModules, totalModules, skippedCount int, t0 time.Time) {
	pass := statusNames[StatusPass] + ":" + fmt.Sprint(stats.Pass)
	fail := statusNames[StatusFail] + ":" + fmt.Sprint(stats.Fail)
	buildfail := statusNames[StatusBuildFail] + ":" + fmt.Sprint(stats.BuildFail)
	none := statusNames[StatusNone] + ":" + fmt.Sprint(stats.None)
	skip := statusNames[StatusSkip] + ":" + fmt.Sprint(stats.Skip)

	statusColor := hardLineColor

	if stats.Pass > 0 {
		statusColor = passColorBold
		pass = statusColor(pass)
	}

	if stats.None > 0 {
		statusColor = noneColorBold
		none = statusColor(none)
	}

	if stats.Fail > 0 || stats.BuildFail > 0 {
		statusColor = failColorBold
		fail = statusColor(fail)
		buildfail = statusColor(buildfail)
	}

	modSummary := fmt.Sprintf("modules:%d/%d", passedModules, totalModules)
	if skippedCount > 0 {
		modSummary += fmt.Sprintf(",%d-skipped", skippedCount)
	}
	if passedModules < totalModules && (stats.Fail > 0 || stats.BuildFail > 0 || skippedCount > 0) {
		modSummary = failColorBold(modSummary)
	} else if passedModules == totalModules && totalModules > 0 {
		modSummary = passColorBold(modSummary)
	} else {
		modSummary = statusColor(modSummary)
	}

	w := r.out()
	fmt.Fprintln(w, "")
	now := time.Now()
	sep := " " + statusColor("|") + " "
	status := statusColor("══════") + " " +
		statusColor(now.Format("15:04:05")) +
		sep + modSummary +
		sep + pass +
		sep + fail +
		sep + buildfail +
		sep + none +
		sep + skip +
		sep + statusColor(now.Sub(t0).Round(time.Millisecond).String()) +
		"  " + statusColor("══════")

	fmt.Fprintln(w, status)
}
