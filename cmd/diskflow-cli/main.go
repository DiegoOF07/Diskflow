// Command diskflow-cli drives the engine from the terminal, to test and
// benchmark it without the desktop UI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
)

const usage = `usage: diskflow-cli <command> [flags] [ROOT]

commands:
  gen   --out DIR [--entries 10000] [--seed 1] [--size 4096]
  scan  [--db diskflow.db] [--workers 0] ROOT
  top   [--db diskflow.db] [--limit 20] ROOT

Flags go before ROOT. Ctrl+C cancels a running scan.`

var errUsage = errors.New("usage error")

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	switch {
	case errors.Is(err, errUsage):
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	case err != nil:
		fmt.Fprintln(os.Stderr, "diskflow-cli:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return errUsage
	}
	cmd, args := args[0], args[1:]
	switch cmd {
	case "gen":
		return runGen(args, stdout, stderr)
	case "scan":
		return runScan(ctx, args, stdout, stderr)
	case "top":
		return runTop(args, stdout, stderr)
	default:
		fmt.Fprintln(stderr, usage)
		return fmt.Errorf("%w: unknown command %q", errUsage, cmd)
	}
}

// parse parses a command's flags and checks it got exactly wantArgs
// positional arguments. It returns ok=false with a nil error for -h.
func parse(fs *flag.FlagSet, args []string, wantArgs int) (ok bool, err error) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return false, nil
		}
		return false, fmt.Errorf("%w: %s: %v", errUsage, fs.Name(), err)
	}
	if fs.NArg() != wantArgs {
		fs.Usage()
		return false, fmt.Errorf("%w: %s: expected %d argument(s), got %d", errUsage, fs.Name(), wantArgs, fs.NArg())
	}
	return true, nil
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}
