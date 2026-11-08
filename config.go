package main

import (
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
)

// Verbosity represents the level of output detail.
type Verbosity int

const (
	V0 Verbosity = 0 // default
	V1 Verbosity = 1 // minor changes
	V2 Verbosity = 2 // a few more details
	V3 Verbosity = 3 // more stuff
	V4 Verbosity = 4 // debug
)

// Flags holds command-line and environment configuration for tgo.
type Flags struct {
	V                Verbosity
	Config           string
	Results          Statuses
	HideEmptyResults Statuses
	Summary          Statuses
	Bin              string
	Dir              string
	R                string
	All              bool
	PrintConfig      bool
}

// Register registers the CLI flags onto a flag.FlagSet.
func (f *Flags) Register(fs *flag.FlagSet) {
	f.Results = Statuses{StatusFail, StatusNone, StatusBuildFail}
	f.Summary = Statuses{StatusFail, StatusNone, StatusBuildFail}

	fs.StringVar(&f.Bin, "bin", "go", "go binary name")
	fs.StringVar(&f.R, "r", "", "search subtree for go.mod and run tests in each")
	fs.Var(&f.Results, "results", "types of results to show")
	fs.Var(&f.Summary, "summary", "types of summary to show")
	fs.Var(&f.HideEmptyResults, "res-hide", "hide empty results")
	fs.IntVar((*int)(&f.V), "v", 0, "0(lowest) to 4(highest)")
	fs.StringVar(&f.Config, "config", "", "config file")
	fs.BoolVar(&f.All, "all", false, "show mostly everything")
	fs.BoolVar(&f.PrintConfig, "print_config", false, "print config")
}

// PrintHelp writes the help information to the provided io.Writer.
func (f *Flags) PrintHelp(w io.Writer) {
	if bi, ok := debug.ReadBuildInfo(); ok {
		fmt.Fprintf(w, "tgo %s\n", bi.Main.Version)
	}

	fmt.Fprint(w, `
settings:

  tgo specific settings are controlled using environment variables so it
  doesn't clash with other arguments.

  TGO_ALL=1         show mostly everything
  TGO_V=0           verbosity: 0(lowest) to 4(highest)
  TGO_RESULTS       types of results to show
  TGO_SUMMARY       types of summary to show
  TGO_RES_HIDE      types of results to hide when empty
  TGO_BIN=go        go binary name
  TGO_R=path        search subtree for go.mod and run tests in each
  TGO_PRINT_CONFIG  print config on run

`)

	var statusNames []string
	for _, v := range AllStatuses {
		statusNames = append(statusNames, string(v))
	}

	fmt.Fprint(w, "  valid values for TGO_RESULTS, TGO_SUMMARY and TGO_RES_HIDE: ", strings.Join(statusNames, ","), "\n\n")
}

func (f *Flags) printConfig(w io.Writer) {
	fmt.Fprintf(w, `
TGO config:
  TGO_RESULTS: %s
  TGO_SUMMARY: %s
  TGO_RES_HIDE: %s
  TGO_R: %s

`, f.Results.String(), f.Summary.String(), f.HideEmptyResults.String(), f.R)
}

// Setup applies overrides based on the go test arguments and configuration.
// set holds the names of the flags given explicitly, by environment variable
// or config file.
//
// A -v argument raises the verbosity to at least V2 and shows passing tests,
// but does not replace results, summary or res-hide settings given
// explicitly, nor those implied by -all.
func (f *Flags) Setup(args []string, set map[string]bool) {
	if f.All {
		f.Results = AllStatuses
		f.Summary = AllStatuses
		f.HideEmptyResults = Statuses{}
	}

	if !verboseRequested(args) {
		return
	}
	f.V = max(f.V, V2)
	if f.All {
		return
	}
	if !set["results"] {
		f.Results = Statuses{StatusBench, StatusPass, StatusNone, StatusFail}
	}
	if !set["res-hide"] {
		f.HideEmptyResults = Statuses{StatusBench, StatusPass, StatusNone}
	}
	if !set["summary"] {
		f.Summary = Statuses{StatusNone, StatusFail}
	}
}

// verboseRequested reports whether the go test arguments enable -v.
// Arguments after -args are for the test binary and are ignored.
func verboseRequested(args []string) bool {
	enabled := false
	for _, arg := range args {
		if isArgsFlag(arg) {
			break
		}
		if isV, on := parseVerboseFlag(arg); isV {
			enabled = on
		}
	}
	return enabled
}

// parseVerboseFlag reports whether arg is go test's -v flag in any of its
// forms (-v, --v, -v=true, -test.v, ...), and whether it enables verbose
// output.
func parseVerboseFlag(arg string) (isV, on bool) {
	name, value, hasValue, ok := parseFlag(arg)
	if !ok || (name != "v" && name != "test.v") {
		return false, false
	}
	if !hasValue {
		return true, true
	}
	on, err := strconv.ParseBool(value)
	return true, err == nil && on
}

// parseFlag splits a command-line flag of the form -name, --name or
// -name=value. ok is false if arg is not a flag.
func parseFlag(arg string) (name, value string, hasValue, ok bool) {
	flag, ok := strings.CutPrefix(arg, "-")
	if !ok {
		return "", "", false, false
	}
	name, value, hasValue = strings.Cut(strings.TrimPrefix(flag, "-"), "=")
	return name, value, hasValue, true
}

// isArgsFlag reports whether arg is go test's -args flag, after which all
// arguments are passed to the test binary.
func isArgsFlag(arg string) bool {
	return arg == "-args" || arg == "--args"
}

// failFastRequested reports whether the go test arguments enable -failfast.
func failFastRequested(args []string) bool {
	enabled := false
	for _, arg := range args {
		if isArgsFlag(arg) {
			break
		}
		if isFF, on := parseFailFastFlag(arg); isFF {
			enabled = on
		}
	}
	return enabled
}

// parseFailFastFlag reports whether arg is go test's -failfast flag in any of its
// forms (-failfast, --failfast, -failfast=true, -test.failfast, ...), and whether it enables failfast.
func parseFailFastFlag(arg string) (isFF, on bool) {
	name, value, hasValue, ok := parseFlag(arg)
	if !ok || (name != "failfast" && name != "test.failfast") {
		return false, false
	}
	if !hasValue {
		return true, true
	}
	on, err := strconv.ParseBool(value)
	return true, err == nil && on
}

var valueTakingTestFlags = map[string]bool{
	"run": true, "bench": true, "benchtime": true, "count": true,
	"coverpkg": true, "cpu": true, "list": true, "outputdir": true,
	"parallel": true, "skip": true, "tags": true, "timeout": true,
	"vet": true, "exec": true, "o": true, "coverprofile": true,
	"blockprofile": true, "cpuprofile": true, "memprofile": true,
	"mutexprofile": true, "trace": true, "test.run": true,
	"test.bench": true, "test.count": true, "test.timeout": true,
	"test.list": true, "test.parallel": true, "test.skip": true,
}

// hasPackageTarget reports whether args contains at least one package or directory argument.
func hasPackageTarget(args []string) bool {
	skipNext := false
	for _, arg := range args {
		if isArgsFlag(arg) {
			break
		}
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			name, _, hasValue, ok := parseFlag(arg)
			if ok && !hasValue && valueTakingTestFlags[name] {
				skipNext = true
			}
			continue
		}
		return true
	}
	return false
}

// Any returns true if any of the target statuses exist in ss.
func (ss Statuses) Any(statuses ...Status) bool {
	for _, s := range ss {
		if slices.Contains(statuses, s) {
			return true
		}
	}
	return false
}

// String implements flag.Value for Statuses.
func (ss *Statuses) String() string {
	var r []string
	for _, v := range *ss {
		r = append(r, string(v))
	}
	return strings.Join(r, ",")
}

// Set implements flag.Value for Statuses.
func (ss *Statuses) Set(value string) error {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "-":
		*ss = make([]Status, 0)
		return nil
	case "all":
		*ss = make([]Status, len(AllStatuses))
		copy(*ss, AllStatuses)
		return nil
	}
	var statuses Statuses
	for v := range strings.SplitSeq(value, ",") {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if !AllStatuses.Any(Status(v)) {
			return fmt.Errorf("%s is not a valid status", v)
		}
		statuses = append(statuses, Status(v))
	}
	if len(statuses) == 0 {
		return fmt.Errorf("%q contains no status, use - for none", value)
	}

	*ss = statuses
	return nil
}
