// Command avd-shortpath-test checks whether RDP Shortpath for public networks
// is likely to work from the current network, similar to avdnettest.exe on Windows.
package main

import (
	"flag"
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

const disclaimer = "This is an unofficial tool and is not affiliated with or endorsed by Microsoft."

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("avd-shortpath-test %s\n%s\n", version, disclaimer)
		return
	}

	fmt.Fprintln(os.Stderr, "avd-shortpath-test: checks not implemented yet")
	os.Exit(3)
}
