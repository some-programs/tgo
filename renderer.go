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
