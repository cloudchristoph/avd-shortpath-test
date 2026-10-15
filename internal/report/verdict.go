// Package report turns a probe result into the verdict, the text and JSON
// output and the exit code.
package report

import "github.com/cloudchristoph/avd-shortpath-test/internal/probe"

// Color of a verdict, as the original prints it.
type Color string

const (
	Green  Color = "green"
	Red    Color = "red"
	Yellow Color = "yellow"
)

// Verdict is the original's two line conclusion, word for word, including
// its typo "Your have access".
type Verdict struct {
	Line1 string `json:"line1"`
	Line2 string `json:"line2"`
	Color Color  `json:"color"`
}

const (
	veryLikely   = "Shortpath for public networks is very likely to work on this host."
	likely       = "Shortpath for public networks is likely to work on this host."
	veryUnlikely = "Shortpath for public networks is very unlikely to work on this host."
	undetermined = "We are unable to determine if shortpath for public networks will work on this host."
)

// VerdictFor returns the original's verdict, or false when the original
// prints none (no mapped address at all).
func VerdictFor(nat probe.NATType, turn bool) (Verdict, bool) {
	switch {
	case nat == probe.NATCone && turn:
		return Verdict{"Your have access to TURN servers and your NAT type appears to be 'cone shaped'.", veryLikely, Green}, true
	case nat == probe.NATCone:
		return Verdict{"You do not have access to TURN servers, but your NAT appears to be 'cone shaped'.", likely, Green}, true
	case nat == probe.NATSymmetric && turn:
		return Verdict{"Your NAT type appears to be 'symmetric', but you have access to TURN servers.", likely, Green}, true
	case nat == probe.NATSymmetric:
		return Verdict{"Your NAT type appears to be 'symmetric' and you do not have access to TURN servers.", veryUnlikely, Red}, true
	case nat == probe.NATUnknown && turn:
		return Verdict{"We were unable to determine your NAT type, but you have access to TURN servers.", veryLikely, Green}, true
	case nat == probe.NATUnknown:
		return Verdict{"We were unable to determine your NAT type, and you do not have access to TURN servers.", undetermined, Yellow}, true
	}
	return Verdict{}, false
}

// Exit codes. They follow the best path that was confirmed, not the
// original's 0/1/99.
const (
	ExitDirect    = 0 // STUN works and the NAT is cone shaped: direct UDP likely
	ExitRelayOnly = 1 // TURN reachable, but no confirmed direct path
	ExitTCPOnly   = 2 // neither: UDP blocked, or symmetric/unknown NAT without TURN
	ExitError     = 3 // the checks could not run (DNS failure, no local socket, bad arguments)
)

// ExitCode maps a result to the exit code.
func ExitCode(r probe.Result) int {
	switch {
	case r.Err != "" || len(r.DNS.Addrs) == 0:
		return ExitError
	case r.NAT == probe.NATCone:
		return ExitDirect
	case r.TURNAccess:
		return ExitRelayOnly
	default:
		return ExitTCPOnly
	}
}

// OriginalExitCode is what avdnettest.exe would return for the same result:
// 1 as soon as one Binding got no valid answer, 0 otherwise (also on DNS
// failure).
func OriginalExitCode(r probe.Result) int {
	for _, s := range r.STUN {
		if s.Status != probe.StatusOK {
			return 1
		}
	}
	return 0
}
