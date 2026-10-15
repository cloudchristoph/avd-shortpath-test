package probe

import (
	"context"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/cloudchristoph/avd-shortpath-test/internal/probe/probetest"
	"github.com/pion/stun/v3"
)

func ap(s string) netip.AddrPort { return netip.MustParseAddrPort(s) }

func TestEvaluateNAT(t *testing.T) {
	a, b := ap("198.51.100.1:5000"), ap("198.51.100.1:5001")
	tests := []struct {
		name   string
		mapped []netip.AddrPort
		want   NATType
	}{
		{"none", nil, NATNone},
		{"one", []netip.AddrPort{a}, NATUnknown},
		{"equal", []netip.AddrPort{a, a}, NATCone},
		{"port differs", []netip.AddrPort{a, b}, NATSymmetric},
		{"ip differs", []netip.AddrPort{a, ap("198.51.100.2:5000")}, NATSymmetric},
		// No sorting, like the original: A B A has two adjacent changes.
		{"non adjacent duplicate", []netip.AddrPort{a, b, a}, NATSymmetric},
		{"three equal", []netip.AddrPort{a, a, a}, NATCone},
	}
	for _, tt := range tests {
		if got := EvaluateNAT(tt.mapped); got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestAllocateMatchesOriginalAttributes(t *testing.T) {
	m := newAllocate()
	dec := &stun.Message{Raw: append([]byte(nil), m.Raw...)}
	if err := dec.Decode(); err != nil {
		t.Fatal(err)
	}
	if dec.Type != stun.NewType(stun.MethodAllocate, stun.ClassRequest) {
		t.Fatalf("type %s", dec.Type)
	}
	check := func(at stun.AttrType, want []byte) {
		t.Helper()
		v, err := dec.Get(at)
		if err != nil {
			t.Fatalf("%s missing", at)
		}
		if string(v) != string(want) {
			t.Errorf("%s = %x, want %x", at, v, want)
		}
	}
	check(stun.AttrRequestedTransport, []byte{17, 0, 0, 0})
	check(stun.AttrDontFragment, nil)
	check(stun.AttrRequestedAddressFamily, []byte{1, 0, 0, 0})
	lt, _ := dec.Get(stun.AttrLifetime)
	if binary.BigEndian.Uint32(lt) != 3600 {
		t.Errorf("lifetime %d", binary.BigEndian.Uint32(lt))
	}
	for _, forbidden := range []stun.AttrType{stun.AttrUsername, stun.AttrMessageIntegrity} {
		if dec.Contains(forbidden) {
			t.Errorf("%s must not be sent", forbidden)
		}
	}
}

const testTimeout = 300 * time.Millisecond

func runAgainst(t *testing.T, servers ...netip.AddrPort) Result {
	t.Helper()
	return run(context.Background(), Config{Timeout: testTimeout}, Result{}, servers)
}

// Like Microsoft: 300 with ALTERNATE-SERVER, second server answers the
// Binding with the same mapping. Measured verdict: cone + TURN.
func TestTryAlternateCone(t *testing.T) {
	alt := probetest.Start(t, probetest.Config{})
	first := probetest.Start(t, probetest.Config{Alternate: alt.Addr()})

	r := runAgainst(t, first.Addr())

	if !r.TURNAccess || len(r.TURN) != 1 {
		t.Fatalf("turn: %+v", r.TURN)
	}
	tr := r.TURN[0]
	if tr.ErrorCode != 300 || tr.Realm != "rtcmedia" || !tr.HasNonce || tr.Alternate == nil || *tr.Alternate != alt.Addr() {
		t.Errorf("allocate result: %+v", tr)
	}
	if len(r.STUN) != 2 || r.STUN[0].Server != first.Addr() || r.STUN[1].Server != alt.Addr() {
		t.Fatalf("stun order: %+v", r.STUN)
	}
	if r.NAT != NATCone {
		t.Errorf("nat %s, mapped %v", r.NAT, r.Mapped)
	}
	if r.Mapped[0].Port() != r.LocalAddr.Port() {
		t.Errorf("mapped %v, local %v", r.Mapped[0], r.LocalAddr)
	}
	if got := alt.Requests(); len(got) != 1 || got[0] != stun.MethodBinding {
		t.Errorf("alternate got %v, want one Binding", got)
	}
}

// Measured with VMPal's NAT: same IP, different port.
func TestTryAlternateSymmetric(t *testing.T) {
	alt := probetest.Start(t, probetest.Config{PortOffset: 7})
	first := probetest.Start(t, probetest.Config{Alternate: alt.Addr()})

	r := runAgainst(t, first.Addr())
	if r.NAT != NATSymmetric || !r.TURNAccess {
		t.Errorf("nat %s turn %v mapped %v", r.NAT, r.TURNAccess, r.Mapped)
	}
}

// f1/f2: any well formed response counts as TURN access; without an
// alternate there is one mapping, so the NAT type is unknown.
func TestAnyWellFormedAllocateResponseIsTURNAccess(t *testing.T) {
	for _, mode := range []probetest.AllocateMode{probetest.AllocateUnauthorized, probetest.AllocateSuccess} {
		s := probetest.Start(t, probetest.Config{Allocate: mode})
		r := runAgainst(t, s.Addr())
		if !r.TURNAccess || r.TURN[0].Status != StatusOK || r.NAT != NATUnknown {
			t.Errorf("mode %d: turn %+v nat %s", mode, r.TURN[0], r.NAT)
		}
	}
	s := probetest.Start(t, probetest.Config{Allocate: probetest.AllocateUnauthorized})
	if r := runAgainst(t, s.Addr()); r.TURN[0].ErrorCode != 401 || r.TURN[0].Class != "error response" {
		t.Errorf("401 not reported: %+v", r.TURN[0])
	}
}

// f3: garbage answer to Allocate is "invalid", Binding still runs.
func TestGarbageAllocate(t *testing.T) {
	s := probetest.Start(t, probetest.Config{Allocate: probetest.AllocateGarbage})
	r := runAgainst(t, s.Addr())
	if r.TURNAccess || r.TURN[0].Status != StatusInvalid || r.STUN[0].Status != StatusOK || r.NAT != NATUnknown {
		t.Errorf("turn %+v stun %+v nat %s", r.TURN[0], r.STUN[0], r.NAT)
	}
}

// stun.cloudflare.com: Allocate ignored, Binding answered.
func TestSilentAllocate(t *testing.T) {
	s := probetest.Start(t, probetest.Config{Allocate: probetest.AllocateSilent})
	r := runAgainst(t, s.Addr())
	if r.TURNAccess || r.TURN[0].Status != StatusTimeout || r.NAT != NATUnknown {
		t.Errorf("turn %+v nat %s", r.TURN[0], r.NAT)
	}
}

// stun.sipgate.net: two addresses, no TURN, same mapping: cone without TURN.
func TestTwoResolvedAddressesWithoutTURN(t *testing.T) {
	a := probetest.Start(t, probetest.Config{Allocate: probetest.AllocateSilent})
	b := probetest.Start(t, probetest.Config{Allocate: probetest.AllocateSilent})
	r := runAgainst(t, a.Addr(), b.Addr())
	if len(r.TURN) != 2 || r.TURNAccess || len(r.STUN) != 2 || r.NAT != NATCone {
		t.Errorf("turn %d access %v stun %d nat %s", len(r.TURN), r.TURNAccess, len(r.STUN), r.NAT)
	}
	// Order: both Allocates first, then both Bindings.
	if got := a.Requests(); len(got) != 2 || got[0] != stun.MethodAllocate || got[1] != stun.MethodBinding {
		t.Errorf("server a got %v", got)
	}
}

// f4: alternate silent. The original keeps the mapping it has, prints a
// verdict and exits 1; we keep the result and let the report decide.
func TestSilentAlternate(t *testing.T) {
	alt := probetest.Start(t, probetest.Config{Binding: probetest.BindingSilent})
	first := probetest.Start(t, probetest.Config{Alternate: alt.Addr()})
	r := runAgainst(t, first.Addr())
	if !r.TURNAccess || r.STUN[1].Status != StatusTimeout || r.NAT != NATUnknown || r.UDPBlocked() {
		t.Errorf("stun %+v nat %s", r.STUN, r.NAT)
	}
}

// f5: garbage Binding answer.
func TestGarbageBinding(t *testing.T) {
	s := probetest.Start(t, probetest.Config{Allocate: probetest.AllocateUnauthorized, Binding: probetest.BindingGarbage})
	r := runAgainst(t, s.Addr())
	if r.STUN[0].Status != StatusInvalid || r.NAT != NATNone {
		t.Errorf("stun %+v nat %s", r.STUN[0], r.NAT)
	}
}

// UDP blocked: nothing answers at all.
func TestNothingAnswers(t *testing.T) {
	s := probetest.Start(t, probetest.Config{Allocate: probetest.AllocateSilent, Binding: probetest.BindingSilent})
	start := time.Now()
	r := runAgainst(t, s.Addr())
	if !r.UDPBlocked() || r.TURNAccess || r.NAT != NATNone {
		t.Errorf("result %+v", r)
	}
	// One Allocate and one Binding, each waits the full timeout, no retries.
	if el := time.Since(start); el < 2*testTimeout || el > 4*testTimeout {
		t.Errorf("took %v", el)
	}
	if got := s.Requests(); len(got) != 2 {
		t.Errorf("requests %v, want exactly 2 (no retransmissions)", got)
	}
}

// f6: ALTERNATE-SERVER pointing to the server itself gives two equal
// mappings from the same server, which the original counts as cone.
func TestAlternateIsSameServer(t *testing.T) {
	s := probetest.Start(t, probetest.Config{AlternateSelf: true})
	r := runAgainst(t, s.Addr())
	if len(r.STUN) != 2 || r.STUN[1].Server != s.Addr() || r.NAT != NATCone {
		t.Errorf("stun %+v nat %s", r.STUN, r.NAT)
	}
}

func TestResolveIPLiteral(t *testing.T) {
	r := resolve(context.Background(), Config{TURNServer: "51.5.68.1"})
	if !r.IPLiteral || len(r.Addrs) != 1 || r.Err != "" {
		t.Errorf("%+v", r)
	}
	r = resolve(context.Background(), Config{TURNServer: "2001:db8::1"})
	if len(r.Addrs) != 0 || r.Err == "" {
		t.Errorf("IPv6 literal accepted: %+v", r)
	}
}

func TestDNSFailureGivesEmptyRun(t *testing.T) {
	r := Run(context.Background(), Config{TURNServer: "does-not-exist.invalid", Port: 3478, Timeout: testTimeout})
	if r.DNS.Err == "" || len(r.TURN) != 0 || len(r.STUN) != 0 || r.NAT != NATNone || r.UDPBlocked() {
		t.Errorf("%+v", r)
	}
}
