package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/peterbourgon/ff/v3"
)

func main() {
	log.SetFlags(log.Lshortfile)
	fs := flag.NewFlagSet("tgo", flag.ExitOnError)

	var flags Flags
	flags.Register(fs)

	if err := ff.Parse(fs, nil,
		ff.WithEnvVarPrefix("TGO"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	flags.Setup(os.Args[1:], set)

	if flags.PrintConfig {
		flags.printConfig(os.Stderr)
	}

	if len(os.Args) > 1 && os.Args[1] == "-h" {
		flags.PrintHelp(os.Stderr)
	}

	if flags.V <= V3 {
		log.SetOutput(io.Discard)
	}

	log.Printf("flags %+v", flags)
	log.Printf("args: %+v", os.Args)

	ctx, stop := signal.NotifyContext(context.Background(), interruptSignals()...)
	defer stop()

	if err := run(ctx, flags, os.Args[1:]); err != nil {
		if ee, ok := errors.AsType[ExitError](err); ok {
			os.Exit(int(ee))
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// interruptSignals returns the signals that interrupt a run. SIGHUP is left
// out when it was ignored at startup, as under nohup, since handling it
// would stop ignoring it and end the run on a terminal hangup.
func interruptSignals() []os.Signal {
	sigs := []os.Signal{os.Interrupt, syscall.SIGTERM}
	if !signal.Ignored(syscall.SIGHUP) {
		sigs = append(sigs, syscall.SIGHUP)
	}
	return sigs
}
