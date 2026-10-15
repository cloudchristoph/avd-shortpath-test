// Command avd-shortpath-test checks whether RDP Shortpath for public networks
// is likely to work from the current network, similar to avdnettest.exe on Windows.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/cloudchristoph/avd-shortpath-test/internal/config"
	"github.com/cloudchristoph/avd-shortpath-test/internal/probe"
	"github.com/cloudchristoph/avd-shortpath-test/internal/report"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	defaults := config.Default()

	fs := flag.NewFlagSet("avd-shortpath-test", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	jsonOut := fs.Bool("json", false, "print the result as JSON")
	verbose := fs.Bool("verbose", false, "include transaction IDs and the original's exit code")
	noColor := fs.Bool("no-color", false, "disable colored output (also honors NO_COLOR)")
	timeout := fs.Duration("timeout", defaults.Timeout(), "wait for each response; no retransmissions")
	turnServer := fs.String("avd-turn-server", defaults.TURNServer, "AVD TURN server DNS name or IPv4 address")
	serversFile := fs.String("servers", "", "JSON file overriding the embedded server defaults")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: avd-shortpath-test [flags]\n\n%s\n\nFlags:\n", report.Disclaimer)
		fs.PrintDefaults()
		fmt.Fprintf(stderr, "\nExit codes: 0 direct UDP likely, 1 TURN relay only, 2 TCP only, 3 checks could not run.\n")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return report.ExitError
	}

	if *showVersion {
		fmt.Fprintf(stdout, "avd-shortpath-test %s\n%s\n", version, report.Disclaimer)
		return 0
	}

	servers := defaults
	if *serversFile != "" {
		b, err := os.ReadFile(*serversFile)
		if err == nil {
			servers, err = config.Parse(b)
		}
		if err != nil {
			fmt.Fprintf(stderr, "avd-shortpath-test: %v\n", err)
			return report.ExitError
		}
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["avd-turn-server"] {
		servers.TURNServer = *turnServer
	}
	wait := servers.Timeout()
	if set["timeout"] {
		wait = *timeout
	}
	if wait <= 0 || wait > time.Minute {
		fmt.Fprintln(stderr, "avd-shortpath-test: --timeout must be between 1ms and 1m")
		return report.ExitError
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res := probe.Run(ctx, probe.Config{TURNServer: servers.TURNServer, Port: servers.Port, Timeout: wait})

	if *jsonOut {
		if err := report.WriteJSON(stdout, res, version); err != nil {
			fmt.Fprintf(stderr, "avd-shortpath-test: %v\n", err)
			return report.ExitError
		}
	} else {
		report.WriteText(stdout, res, report.Options{
			Version: version,
			Color:   !*noColor && os.Getenv("NO_COLOR") == "" && isTerminal(stdout),
			Verbose: *verbose,
		})
	}
	return report.ExitCode(res)
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
