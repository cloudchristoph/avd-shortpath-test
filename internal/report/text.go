package report

import (
	"fmt"
	"io"
	"net/netip"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cloudchristoph/avd-shortpath-test/internal/probe"
)

// Messages the original prints, word for word.
const (
	msgDNSFailed      = "DNS resolution of %s failed. This might be a temporary failure, or your firewall might restrict DNS queries."
	msgTURNTimeout    = "Did not receive a response from TURN server. Trying with STUN only."
	msgTURNInvalid    = "UDP works, but the response from the TURN server is invalid. Trying with STUN only."
	msgSTUNTimeout    = "Did not receive a response on UDP. Check if firewall allows UDP access to 51.5.0.0/16:3478."
	msgSTUNInvalid    = "UDP works, but the response from the server is invalid. This is very unusual."
	msgSocketFailed   = "Failed to open local UDP port. Your computer might have UDP disabled by a policy or firewall. This is very unusual."
	footer            = "See https://go.microsoft.com/fwlink/?linkid=2204021 for more information."
	disclaimerShort   = "unofficial, not affiliated with Microsoft"
	checkLinePrefix   = "Checking "
	resultOK          = "OK"
	resultFail        = "FAIL"
	turnServerSubject = "Windows cloud global TURN server(%s)"
)

// Options control the text output.
type Options struct {
	Version string
	Color   bool
	Verbose bool
}

// WriteText prints the result in the style of avdnettest.exe, followed by a
// details section with what the original only writes to its debug log.
func WriteText(w io.Writer, r probe.Result, o Options) {
	p := &printer{w: w, color: o.Color}
	p.line("avd-shortpath-test %s (%s)", o.Version, disclaimerShort)

	subject := fmt.Sprintf(turnServerSubject, r.DNS.Name)
	if len(r.DNS.Addrs) == 0 {
		p.check(subject, false)
		p.line(msgDNSFailed, r.DNS.Name)
	} else {
		p.check(subject+" resolved to : "+joinAddrs(r.DNS.Addrs, r.Port), true)
	}
	if r.Err != "" {
		p.line(msgSocketFailed)
	}

	for _, t := range r.TURN {
		p.check("TURN support "+t.Server.String(), t.Status == probe.StatusOK)
		switch t.Status {
		case probe.StatusTimeout:
			p.line(msgTURNTimeout)
		case probe.StatusInvalid, probe.StatusError:
			p.line(msgTURNInvalid)
		}
	}
	for _, s := range r.STUN {
		p.check("STUN support "+s.Server.String(), s.Status == probe.StatusOK)
		switch s.Status {
		case probe.StatusTimeout:
			p.line(msgSTUNTimeout)
		case probe.StatusInvalid, probe.StatusError:
			p.line(msgSTUNInvalid)
		}
	}

	p.line("")
	if v, ok := VerdictFor(r.NAT, r.TURNAccess); ok {
		p.colored(v.Color, v.Line1)
		p.colored(v.Color, v.Line2)
	}
	p.line("")
	p.line(footer)

	writeDetails(p, r, o.Verbose)
}

func writeDetails(p *printer, r probe.Result, verbose bool) {
	p.line("")
	p.line("Details")
	tw := tabwriter.NewWriter(p.w, 0, 0, 2, ' ', 0)
	row := func(cols ...string) { fmt.Fprintln(tw, "  "+strings.Join(cols, "\t")) }

	dns := joinAddrs(r.DNS.Addrs, 0)
	switch {
	case r.DNS.IPLiteral:
		dns = "IP address, no DNS lookup"
	case r.DNS.Err != "":
		dns = r.DNS.Err
	}
	row("DNS", r.DNS.Name, ms(r.DNS.Duration), dns)

	for _, t := range r.TURN {
		desc := string(t.Status)
		if t.Status == probe.StatusOK {
			desc = turnResponse(t)
		}
		row("TURN", t.Server.String(), ms(t.RTT), desc)
		if verbose {
			row("", "", "", "transaction "+t.TransactionID)
		}
	}
	for _, s := range r.STUN {
		desc := string(s.Status)
		if s.Status == probe.StatusOK {
			desc = "mapped " + s.Mapped.String()
		}
		row("STUN", s.Server.String(), ms(s.RTT), desc)
		if verbose {
			row("", "", "", "transaction "+s.TransactionID)
		}
	}
	if verbose && r.LocalAddr.IsValid() {
		row("Local", "port "+fmt.Sprint(r.LocalAddr.Port()), "one socket for all requests")
	}
	code := ExitCode(r)
	row("NAT", natText(r.NAT))
	row("Result", fmt.Sprintf("%s (exit %d)", resultText(code), code))
	if verbose {
		row("Original", fmt.Sprintf("avdnettest.exe would exit %d", OriginalExitCode(r)))
	}
	tw.Flush()
}

func turnResponse(t probe.TURNResult) string {
	parts := []string{t.Class}
	if t.ErrorCode != 0 {
		parts = []string{fmt.Sprintf("%d %s", t.ErrorCode, t.ErrorReason)}
	}
	if t.Realm != "" {
		parts = append(parts, "realm "+t.Realm)
	}
	if t.Alternate != nil {
		parts = append(parts, "alternate "+t.Alternate.String())
	}
	return strings.Join(parts, ", ")
}

func natText(n probe.NATType) string {
	switch n {
	case probe.NATCone:
		return "cone shaped (endpoint independent mapping)"
	case probe.NATSymmetric:
		return "symmetric (endpoint dependent mapping)"
	case probe.NATUnknown:
		return "unknown (only one server answered)"
	}
	return "not determined (no STUN answer)"
}

func resultText(code int) string {
	switch code {
	case ExitDirect:
		return "direct UDP path via STUN likely"
	case ExitRelayOnly:
		return "UDP via TURN relay only"
	case ExitTCPOnly:
		return "no usable UDP path, TCP reverse connect only"
	}
	return "checks could not run"
}

func joinAddrs(addrs []netip.Addr, port uint16) string {
	s := make([]string, len(addrs))
	for i, a := range addrs {
		if port != 0 {
			s[i] = netip.AddrPortFrom(a, port).String()
		} else {
			s[i] = a.String()
		}
	}
	return strings.Join(s, ", ")
}

func ms(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	return fmt.Sprintf("%d ms", d.Round(time.Millisecond).Milliseconds())
}

type printer struct {
	w     io.Writer
	color bool
}

func (p *printer) line(format string, args ...any) {
	fmt.Fprintf(p.w, format+"\n", args...)
}

func (p *printer) check(subject string, ok bool) {
	res, c := resultOK, Green
	if !ok {
		res, c = resultFail, Red
	}
	fmt.Fprintf(p.w, "%s%s ... %s\n", checkLinePrefix, subject, p.paint(c, res))
}

func (p *printer) colored(c Color, s string) {
	fmt.Fprintln(p.w, p.paint(c, s))
}

func (p *printer) paint(c Color, s string) string {
	if !p.color {
		return s
	}
	code := map[Color]string{Green: "32", Red: "31", Yellow: "33"}[c]
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
