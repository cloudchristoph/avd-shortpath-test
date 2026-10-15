package report

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/cloudchristoph/avd-shortpath-test/internal/probe"
)

func TestVerdictTable(t *testing.T) {
	// Pairs and colours as measured and confirmed in the original's code.
	tests := []struct {
		nat         probe.NATType
		turn        bool
		line1Prefix string
		line2       string
		color       Color
	}{
		{probe.NATCone, true, "Your have access to TURN servers", veryLikely, Green},
		{probe.NATCone, false, "You do not have access to TURN servers, but", likely, Green},
		{probe.NATSymmetric, true, "Your NAT type appears to be 'symmetric', but", likely, Green},
		{probe.NATSymmetric, false, "Your NAT type appears to be 'symmetric' and", veryUnlikely, Red},
		{probe.NATUnknown, true, "We were unable to determine your NAT type, but", veryLikely, Green},
		{probe.NATUnknown, false, "We were unable to determine your NAT type, and", undetermined, Yellow},
	}
	for _, tt := range tests {
		v, ok := VerdictFor(tt.nat, tt.turn)
		if !ok || !strings.HasPrefix(v.Line1, tt.line1Prefix) || v.Line2 != tt.line2 || v.Color != tt.color {
			t.Errorf("%s/%v: %+v", tt.nat, tt.turn, v)
		}
	}
	for _, turn := range []bool{true, false} {
		if _, ok := VerdictFor(probe.NATNone, turn); ok {
			t.Errorf("no mapped address must give no verdict (turn=%v)", turn)
		}
	}
}

func ok(server string, mapped string) probe.STUNResult {
	return probe.STUNResult{Server: netip.MustParseAddrPort(server), Status: probe.StatusOK, Mapped: netip.MustParseAddrPort(mapped), RTT: 20 * time.Millisecond}
}

func failed(server string, s probe.Status) probe.STUNResult {
	return probe.STUNResult{Server: netip.MustParseAddrPort(server), Status: s}
}

func result(nat probe.NATType, turn bool, stun ...probe.STUNResult) probe.Result {
	r := probe.Result{
		Port:       3478,
		DNS:        probe.DNSResult{Name: "world.turn.wvd.microsoft.com", Addrs: []netip.Addr{netip.MustParseAddr("51.5.68.1")}},
		TURNAccess: turn,
		STUN:       stun,
		NAT:        nat,
	}
	for _, s := range stun {
		if s.Status == probe.StatusOK {
			r.Mapped = append(r.Mapped, s.Mapped)
		}
	}
	return r
}

func TestExitCodes(t *testing.T) {
	m := "203.0.113.5:40000"
	tests := []struct {
		name     string
		r        probe.Result
		want     int
		original int
	}{
		{"cone with TURN", result(probe.NATCone, true, ok("51.5.68.1:3478", m), ok("51.5.68.7:3478", m)), ExitDirect, 0},
		{"cone without TURN", result(probe.NATCone, false, ok("51.5.68.1:3478", m), ok("51.5.68.7:3478", m)), ExitDirect, 0},
		{"symmetric with TURN", result(probe.NATSymmetric, true, ok("51.5.68.1:3478", m), ok("51.5.68.7:3478", "203.0.113.5:40001")), ExitRelayOnly, 0},
		{"unknown with TURN", result(probe.NATUnknown, true, ok("51.5.68.1:3478", m)), ExitRelayOnly, 0},
		{"alternate silent", result(probe.NATUnknown, true, ok("51.5.68.1:3478", m), failed("51.5.68.7:3478", probe.StatusTimeout)), ExitRelayOnly, 1},
		{"symmetric without TURN", result(probe.NATSymmetric, false, ok("51.5.68.1:3478", m), ok("51.5.68.2:3478", "203.0.113.5:40001")), ExitTCPOnly, 0},
		{"unknown without TURN", result(probe.NATUnknown, false, ok("51.5.68.1:3478", m)), ExitTCPOnly, 0},
		{"UDP blocked", result(probe.NATNone, false, failed("51.5.68.1:3478", probe.StatusTimeout)), ExitTCPOnly, 1},
		{"DNS failed", probe.Result{DNS: probe.DNSResult{Name: "x", Err: "no such host"}, NAT: probe.NATNone}, ExitError, 0},
		{"no socket", func() probe.Result { r := result(probe.NATNone, false); r.Err = "denied"; return r }(), ExitError, 0},
	}
	for _, tt := range tests {
		if got := ExitCode(tt.r); got != tt.want {
			t.Errorf("%s: exit %d, want %d", tt.name, got, tt.want)
		}
		if got := OriginalExitCode(tt.r); got != tt.original {
			t.Errorf("%s: original exit %d, want %d", tt.name, got, tt.original)
		}
	}
}

func TestTextMatchesOriginalWording(t *testing.T) {
	alt := netip.MustParseAddrPort("51.5.68.7:3478")
	r := result(probe.NATCone, true, ok("51.5.68.1:3478", "203.0.113.5:40000"), ok("51.5.68.7:3478", "203.0.113.5:40000"))
	r.TURN = []probe.TURNResult{{Server: netip.MustParseAddrPort("51.5.68.1:3478"), Status: probe.StatusOK, Class: "error response",
		ErrorCode: 300, ErrorReason: "Try Alternate", Realm: "rtcmedia", HasNonce: true, Alternate: &alt, RTT: 34 * time.Millisecond}}

	var b bytes.Buffer
	WriteText(&b, r, Options{Version: "test"})
	got := b.String()

	// The block up to the footer must read like avdnettest.exe's output
	// (minus its "(Unicast(0))" tags and with our own first line).
	want := `avd-shortpath-test test (unofficial, not affiliated with Microsoft)
Checking Windows cloud global TURN server(world.turn.wvd.microsoft.com) resolved to : 51.5.68.1:3478 ... OK
Checking TURN support 51.5.68.1:3478 ... OK
Checking STUN support 51.5.68.1:3478 ... OK
Checking STUN support 51.5.68.7:3478 ... OK

Your have access to TURN servers and your NAT type appears to be 'cone shaped'.
Shortpath for public networks is very likely to work on this host.

See https://go.microsoft.com/fwlink/?linkid=2204021 for more information.
`
	if !strings.HasPrefix(got, want) {
		t.Fatalf("got:\n%s\nwant prefix:\n%s", got, want)
	}
	for _, s := range []string{"300 Try Alternate, realm rtcmedia, alternate 51.5.68.7:3478", "34 ms", "mapped 203.0.113.5:40000", "(exit 0)"} {
		if !strings.Contains(got, s) {
			t.Errorf("details miss %q:\n%s", s, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Error("colour codes without Color option")
	}
}

func TestTextUDPBlocked(t *testing.T) {
	r := result(probe.NATNone, false, failed("51.5.68.1:3478", probe.StatusTimeout))
	r.TURN = []probe.TURNResult{{Server: netip.MustParseAddrPort("51.5.68.1:3478"), Status: probe.StatusTimeout}}
	var b bytes.Buffer
	WriteText(&b, r, Options{Version: "test", Verbose: true})
	got := b.String()
	for _, s := range []string{
		"Checking TURN support 51.5.68.1:3478 ... FAIL\n" + msgTURNTimeout + "\n",
		"Checking STUN support 51.5.68.1:3478 ... FAIL\n" + msgSTUNTimeout + "\n",
		"TCP reverse connect only (exit 2)",
		"avdnettest.exe would exit 1",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q in:\n%s", s, got)
		}
	}
	if strings.Contains(got, "Shortpath for public networks is") {
		t.Error("no verdict expected when nothing answered")
	}
}

func TestTextDNSFailed(t *testing.T) {
	r := probe.Result{DNS: probe.DNSResult{Name: "does-not-exist.invalid", Err: "no such host"}, NAT: probe.NATNone}
	var b bytes.Buffer
	WriteText(&b, r, Options{Version: "test", Color: true})
	got := b.String()
	if !strings.Contains(got, "TURN server(does-not-exist.invalid) ... \x1b[31mFAIL\x1b[0m\nDNS resolution of does-not-exist.invalid failed.") {
		t.Errorf("unexpected:\n%q", got)
	}
}

func TestJSON(t *testing.T) {
	r := result(probe.NATSymmetric, true, ok("51.5.68.1:3478", "203.0.113.5:40000"), ok("51.5.68.7:3478", "203.0.113.5:40001"))
	var b bytes.Buffer
	if err := WriteJSON(&b, r, nil, "test"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["exitCode"].(float64) != ExitRelayOnly || got["verdict"].(map[string]any)["line2"] != likely || got["disclaimer"] != Disclaimer {
		t.Errorf("%s", b.String())
	}
	res := got["result"].(map[string]any)
	if res["nat"] != "symmetric" || res["stun"].([]any)[1].(map[string]any)["mapped"] != "203.0.113.5:40001" {
		t.Errorf("result: %v", res)
	}
}
