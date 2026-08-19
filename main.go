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
		fmt.Println(err)
		os.Exit(1)
	}

	flags.Setup(os.Args)

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, flags, os.Args[1:]); err != nil {
		if ee, ok := errors.AsType[ExitError](err); ok {
			os.Exit(int(ee))
		}
		fmt.Println(err)
		os.Exit(1)
	}
}
