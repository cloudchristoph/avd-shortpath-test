// Command avd-shortpath-test checks whether RDP Shortpath for public networks
// is likely to work from the current network, similar to avdnettest.exe on Windows.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/signal"
	"time"

	"github.com/cloudchristoph/avd-shortpath-test/internal/config"
	"github.com/cloudchristoph/avd-shortpath-test/internal/env"
	"github.com/cloudchristoph/avd-shortpath-test/internal/logx"
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
	noColor := fs.Bool("no-color", false, "same as --color off")
	colorMode := fs.String("color", modeAuto, "colored output: auto, on, off (auto honors NO_COLOR and needs a terminal)")
	pauseMode := fs.String("pause", modeAuto, "wait for Enter before exiting: auto, on, off (auto never pauses on macOS and Linux; on needs a terminal)")
	timeout := fs.Duration("timeout", defaults.Timeout(), "wait for each response; no retransmissions")
	turnServer := fs.String("avd-turn-server", defaults.TURNServer, "AVD TURN server DNS name or IPv4 address")
	serversFile := fs.String("servers", "", "JSON file overriding the embedded server defaults")
	noEnv := fs.Bool("no-env", false, "skip the environment checks (routes, VPN, proxy)")
	logLevel := fs.String("log-level", "none", "log level: none, critical, error, warning, normal, debug")
	logFile := fs.String("log-file", "avd-shortpath-test.log", "log file (only created when --log-level is not none)")
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
	if !validMode(*colorMode) || !validMode(*pauseMode) {
		fmt.Fprintf(stderr, "avd-shortpath-test: --color and --pause take auto, on or off (got --color %q, --pause %q)\n", *colorMode, *pauseMode)
		return report.ExitError
	}
	defer pauseAtEnd(*pauseMode, os.Stdin, stderr)

	level, err := logx.ParseLevel(*logLevel)
	if err != nil {
		fmt.Fprintf(stderr, "avd-shortpath-test: --log-level: %v\n", err)
		return report.ExitError
	}
	logger, err := logx.Open(level, *logFile)
	if err != nil {
		fmt.Fprintf(stderr, "avd-shortpath-test: --log-file: %v\n", err)
		return report.ExitError
	}
	defer logger.Close()
	logger.Normalf("avd-shortpath-test %s, log level %s", version, level)

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
	ctx = logx.WithContext(ctx, logger)
	res := probe.Run(ctx, probe.Config{TURNServer: servers.TURNServer, Port: servers.Port, Timeout: wait})
	var envReport *env.Report
	if !*noEnv {
		e := env.Collect(ctx, targets(res))
		envReport = &e
	}

	if *jsonOut {
		if err := report.WriteJSON(stdout, res, envReport, version); err != nil {
			fmt.Fprintf(stderr, "avd-shortpath-test: %v\n", err)
			return report.ExitError
		}
	} else {
		report.WriteText(stdout, res, report.Options{
			Version: version,
			Color:   useColor(*colorMode, *noColor, os.Getenv("NO_COLOR"), isTerminal(stdout)),
			Verbose: *verbose,
			Env:     envReport,
		})
	}
	return report.ExitCode(res)
}

// targets are all servers the probe talked to, for the route checks.
func targets(r probe.Result) []netip.Addr {
	seen := map[netip.Addr]bool{}
	var out []netip.Addr
	add := func(a netip.Addr) {
		if a.IsValid() && !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	for _, a := range r.DNS.Addrs {
		add(a)
	}
	for _, s := range r.STUN {
		add(s.Server.Addr())
	}
	return out
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
