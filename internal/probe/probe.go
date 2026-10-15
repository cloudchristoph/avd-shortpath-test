// Package probe runs the network checks of avdnettest.exe: DNS, TURN Allocate,
// STUN Binding and the NAT evaluation, in the same order and with the same
// rules (see docs/reference-avdnettest.md).
package probe

import (
	"context"
	"encoding/hex"
	"net"
	"net/netip"
	"time"
)

// Status is the outcome of a single request.
type Status string

const (
	StatusOK      Status = "ok"
	StatusTimeout Status = "timeout"
	StatusInvalid Status = "invalid"
	StatusError   Status = "error"
)

// NATType is the result of comparing the mapped addresses.
type NATType string

const (
	NATNone      NATType = "none"      // no mapped address at all
	NATUnknown   NATType = "unknown"   // exactly one mapped address
	NATCone      NATType = "cone"      // all mappings equal (endpoint independent)
	NATSymmetric NATType = "symmetric" // mappings differ (endpoint dependent)
)

// Config controls a run.
type Config struct {
	// TURNServer is a DNS name or an IPv4 literal.
	TURNServer string
	Port       uint16
	// Timeout is the wait for each response.
	Timeout time.Duration
	// Resolver is used for DNS; nil means net.DefaultResolver.
	Resolver *net.Resolver
}

// DNSResult is the outcome of resolving the TURN server name.
type DNSResult struct {
	Name      string        `json:"name"`
	IPLiteral bool          `json:"ipLiteral"`
	Addrs     []netip.Addr  `json:"addrs"`
	Duration  time.Duration `json:"durationNs"`
	Err       string        `json:"error,omitempty"`
}

// TURNResult is the outcome of one Allocate request.
type TURNResult struct {
	Server        netip.AddrPort  `json:"server"`
	Status        Status          `json:"status"`
	RTT           time.Duration   `json:"rttNs,omitempty"`
	Class         string          `json:"class,omitempty"`
	ErrorCode     int             `json:"errorCode,omitempty"`
	ErrorReason   string          `json:"errorReason,omitempty"`
	Realm         string          `json:"realm,omitempty"`
	HasNonce      bool            `json:"hasNonce,omitempty"`
	Alternate     *netip.AddrPort `json:"alternate,omitempty"`
	TransactionID string          `json:"transactionId"`
	Err           string          `json:"error,omitempty"`
}

// STUNResult is the outcome of one Binding request.
type STUNResult struct {
	Server        netip.AddrPort `json:"server"`
	Status        Status         `json:"status"`
	RTT           time.Duration  `json:"rttNs,omitempty"`
	Mapped        netip.AddrPort `json:"mapped,omitzero"`
	TransactionID string         `json:"transactionId"`
	Err           string         `json:"error,omitempty"`
}

// Result is everything a run found.
type Result struct {
	Port       uint16           `json:"port"`
	LocalAddr  netip.AddrPort   `json:"localAddr,omitzero"`
	DNS        DNSResult        `json:"dns"`
	TURN       []TURNResult     `json:"turn"`
	STUN       []STUNResult     `json:"stun"`
	TURNAccess bool             `json:"turnAccess"`
	Mapped     []netip.AddrPort `json:"mapped"`
	NAT        NATType          `json:"nat"`
	// Err is set when the local UDP socket could not be opened.
	Err string `json:"error,omitempty"`
}

// UDPBlocked reports whether no STUN server answered at all.
func (r Result) UDPBlocked() bool {
	return len(r.STUN) > 0 && len(r.Mapped) == 0
}

// Run performs the checks in the original's order:
//  1. resolve the TURN server (A records only) and keep every address;
//  2. send an Allocate to every address;
//  3. send a Binding to every address, then to every ALTERNATE-SERVER;
//  4. compare the mapped addresses.
//
// All requests use one local UDP socket. A DNS failure ends the run with an
// empty server list; a failed request is recorded and the run goes on.
func Run(ctx context.Context, cfg Config) Result {
	res := Result{Port: cfg.Port}
	res.DNS = resolve(ctx, cfg)
	servers := make([]netip.AddrPort, 0, len(res.DNS.Addrs))
	for _, a := range res.DNS.Addrs {
		servers = append(servers, netip.AddrPortFrom(a, cfg.Port))
	}
	return run(ctx, cfg, res, servers)
}

// run does steps 2 to 4 against the given servers.
func run(ctx context.Context, cfg Config, res Result, servers []netip.AddrPort) Result {
	res.NAT = NATNone
	if len(servers) == 0 {
		return res
	}

	c, err := openConn(cfg.Timeout)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer c.close()
	res.LocalAddr = c.localAddr()

	var alternates []netip.AddrPort
	for _, s := range servers {
		if ctx.Err() != nil {
			break
		}
		t := allocate(c, s)
		if t.Status == StatusOK {
			res.TURNAccess = true
			if t.Alternate != nil {
				alternates = append(alternates, *t.Alternate)
			}
		}
		res.TURN = append(res.TURN, t)
	}
	servers = append(servers, alternates...)

	for _, s := range servers {
		if ctx.Err() != nil {
			break
		}
		b := binding(c, s)
		if b.Status == StatusOK {
			res.Mapped = append(res.Mapped, b.Mapped)
		}
		res.STUN = append(res.STUN, b)
	}
	res.NAT = EvaluateNAT(res.Mapped)
	return res
}

func resolve(ctx context.Context, cfg Config) DNSResult {
	r := DNSResult{Name: cfg.TURNServer}
	if a, err := netip.ParseAddr(cfg.TURNServer); err == nil {
		r.IPLiteral = true
		if a.Unmap().Is4() {
			r.Addrs = []netip.Addr{a.Unmap()}
		} else {
			r.Err = "only IPv4 addresses are supported, like the original"
		}
		return r
	}
	resolver := cfg.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	start := time.Now()
	addrs, err := resolver.LookupNetIP(ctx, "ip4", cfg.TURNServer)
	r.Duration = time.Since(start)
	if err != nil {
		r.Err = err.Error()
		return r
	}
	for _, a := range addrs {
		r.Addrs = append(r.Addrs, a.Unmap())
	}
	if len(r.Addrs) == 0 {
		r.Err = "no IPv4 address"
	}
	return r
}

func allocate(c *conn, server netip.AddrPort) TURNResult {
	req := newAllocate()
	r := TURNResult{Server: server, TransactionID: txID(req.TransactionID)}
	resp, rtt, err := c.roundTrip(req, server)
	r.Status, r.RTT = statusOf(err), rtt
	if err != nil {
		r.Err = err.Error()
		return r
	}
	parseAllocateResponse(resp, &r)
	return r
}

func binding(c *conn, server netip.AddrPort) STUNResult {
	req := newBinding()
	r := STUNResult{Server: server, TransactionID: txID(req.TransactionID)}
	resp, rtt, err := c.roundTrip(req, server)
	r.Status, r.RTT = statusOf(err), rtt
	if err != nil {
		r.Err = err.Error()
		return r
	}
	mapped, ok := mappedAddress(resp)
	if !ok {
		r.Status, r.Err = StatusInvalid, "no mapped address in response"
		return r
	}
	r.Mapped = mapped
	return r
}

// EvaluateNAT applies the original's rule: no address is none, one is
// unknown; otherwise adjacent duplicates are removed (no sorting) and a single
// remaining address is cone, more are symmetric. IP and port both count.
func EvaluateNAT(mapped []netip.AddrPort) NATType {
	switch len(mapped) {
	case 0:
		return NATNone
	case 1:
		return NATUnknown
	}
	distinct := 1
	for i := 1; i < len(mapped); i++ {
		if mapped[i] != mapped[i-1] {
			distinct++
		}
	}
	if distinct == 1 {
		return NATCone
	}
	return NATSymmetric
}

func txID(b [12]byte) string { return hex.EncodeToString(b[:]) }
