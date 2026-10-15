package report

import (
	"encoding/json"
	"io"

	"github.com/cloudchristoph/avd-shortpath-test/internal/probe"
)

// Disclaimer is printed with --version and included in the JSON output.
const Disclaimer = "This is an unofficial tool and is not affiliated with or endorsed by Microsoft."

// JSONReport is the --json output. Durations are in nanoseconds.
type JSONReport struct {
	Tool             string       `json:"tool"`
	Version          string       `json:"version"`
	Disclaimer       string       `json:"disclaimer"`
	Result           probe.Result `json:"result"`
	Verdict          *Verdict     `json:"verdict"`
	Summary          string       `json:"summary"`
	ExitCode         int          `json:"exitCode"`
	OriginalExitCode int          `json:"originalExitCode"`
}

// NewJSONReport builds the --json document.
func NewJSONReport(r probe.Result, version string) JSONReport {
	code := ExitCode(r)
	j := JSONReport{
		Tool:             "avd-shortpath-test",
		Version:          version,
		Disclaimer:       Disclaimer,
		Result:           r,
		Summary:          resultText(code),
		ExitCode:         code,
		OriginalExitCode: OriginalExitCode(r),
	}
	if v, ok := VerdictFor(r.NAT, r.TURNAccess); ok {
		j.Verdict = &v
	}
	return j
}

// WriteJSON writes the --json document.
func WriteJSON(w io.Writer, r probe.Result, version string) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(NewJSONReport(r, version))
}
