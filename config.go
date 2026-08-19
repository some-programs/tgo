package main

import (
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"slices"
	"strings"
)

// Verbosity represents the level of output detail.
type Verbosity int

var (
	V0 = Verbosity(0) // default
	V1 = Verbosity(1) // minor changes
	V2 = Verbosity(2) // a few more details
	V3 = Verbosity(3) // more stuff
	V4 = Verbosity(4) // debug
	V5 = Verbosity(5) // (reserved)
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
	All              bool
	PrintConfig      bool
}

// Register registers the CLI flags onto a flag.FlagSet.
func (f *Flags) Register(fs *flag.FlagSet) {
	f.Results = Statuses{StatusFail, StatusNone, StatusBuildFail}
	f.Summary = Statuses{StatusFail, StatusNone, StatusBuildFail}

	fs.StringVar(&f.Bin, "bin", "go", "go binary name")
	fs.Var(&f.Results, "results", "types of results to show")
	fs.Var(&f.Summary, "summary", "types of summary to show")
	fs.Var(&f.HideEmptyResults, "res-hide", "hide empty results")
	fs.IntVar((*int)(&f.V), "v", 0, "0(lowest) to 5(highest)")
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
  TGO_V=0           verbosity: 0(lowest) to 5(highest)
  TGO_RESULTS       types of results to show
  TGO_SUMMARY       types of summary to show
  TGO_RES_HIDE      types of results to hide when empty
  TGO_BIN=go        go binary name
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

`, f.Results.String(), f.Summary.String(), f.HideEmptyResults.String())
}

// Setup applies overrides based on CLI arguments and configuration.
func (f *Flags) Setup(args []string) {
	if f.All {
		f.Results = AllStatuses
		f.Summary = AllStatuses
		f.HideEmptyResults = Statuses{}
	}

	for _, v := range args {
		if v == "-v" {
			f.V = V2
			f.Results = Statuses{
				StatusBench,
				StatusPass,
				StatusNone,
				StatusFail,
			}
			f.HideEmptyResults = Statuses{
				StatusBench,
				StatusPass,
				StatusNone,
			}
			f.Summary = Statuses{
				StatusNone,
				StatusFail,
			}
		}
	}
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
	value = strings.ToLower(value)
	switch value {
	case "-":
		*ss = make([]Status, 0)
		return nil
	case "all":
		*ss = make([]Status, len(AllStatuses))
		copy(*ss, AllStatuses)
		return nil
	}
	split := strings.Split(value, ",")
	var statuses Statuses
	for _, v := range split {
		if !AllStatuses.Any(Status(v)) {
			return fmt.Errorf("%s is not a valid status", v)
		}
		statuses = append(statuses, Status(v))
	}

	*ss = statuses
	return nil
}
