// Package probetest provides a scriptable STUN/TURN server for tests. It
// answers the way Microsoft's TURN relays do (300 Try Alternate) and can also
// be told to answer in ways real servers don't, to reach the same code paths
// that were measured against avdnettest.exe (see docs/reference-avdnettest.md).
package probetest

import (
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/pion/stun/v3"
)

// AllocateMode selects the answer to a TURN Allocate request.
type AllocateMode int

const (
	AllocateTryAlternate AllocateMode = iota // 300 with REALM, NONCE and ALTERNATE-SERVER, like Microsoft
	AllocateUnauthorized                     // 401 with REALM and NONCE
	AllocateSuccess                          // Allocate success response
	AllocateGarbage                          // a datagram that is not STUN
	AllocateSilent                           // no answer
)

// BindingMode selects the answer to a STUN Binding request.
type BindingMode int

const (
	BindingOK      BindingMode = iota // success with XOR-MAPPED-ADDRESS
	BindingGarbage                    // a datagram that is not STUN
	BindingSilent                     // no answer
)

// Config describes how a Server answers.
type Config struct {
	Allocate AllocateMode
	Binding  BindingMode
	// Alternate is sent as ALTERNATE-SERVER with AllocateTryAlternate.
	Alternate netip.AddrPort
	// AlternateSelf sends the server's own address as ALTERNATE-SERVER.
	AlternateSelf bool
	// PortOffset is added to the mapped port, to make two servers report
	// different mappings for the same client socket (symmetric NAT).
	PortOffset int
}

// Server is a STUN/TURN test server on 127.0.0.1.
type Server struct {
	conn net.PacketConn
	cfg  Config

	mu       sync.Mutex
	requests []stun.Method
}

// Start starts a server on a random local port and stops it when the test ends.
func Start(t testing.TB, cfg Config) *Server {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probetest: listen: %v", err)
	}
	s := &Server{conn: conn, cfg: cfg}
	go s.serve()
	t.Cleanup(func() { _ = conn.Close() })
	return s
}

// Addr returns the address the server listens on.
func (s *Server) Addr() netip.AddrPort {
	return s.conn.LocalAddr().(*net.UDPAddr).AddrPort()
}

// Requests returns the methods of all STUN requests received so far.
func (s *Server) Requests() []stun.Method {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stun.Method(nil), s.requests...)
}

func (s *Server) serve() {
	buf := make([]byte, 1500)
	for {
		n, addr, err := s.conn.ReadFrom(buf)
		if err != nil {
			return
		}
		m := &stun.Message{Raw: append([]byte(nil), buf[:n]...)}
		if err := m.Decode(); err != nil {
			continue
		}
		s.mu.Lock()
		s.requests = append(s.requests, m.Type.Method)
		s.mu.Unlock()

		var out []byte
		switch m.Type.Method {
		case stun.MethodAllocate:
			out = s.allocate(m)
		case stun.MethodBinding:
			out = s.binding(m, addr.(*net.UDPAddr))
		}
		if out != nil {
			_, _ = s.conn.WriteTo(out, addr)
		}
	}
}

var garbage = []byte("this is not a STUN message")

func (s *Server) allocate(req *stun.Message) []byte {
	tid := stun.NewTransactionIDSetter(req.TransactionID)
	errType := stun.NewType(stun.MethodAllocate, stun.ClassErrorResponse)
	switch s.cfg.Allocate {
	case AllocateSilent:
		return nil
	case AllocateGarbage:
		return garbage
	case AllocateSuccess:
		return stun.MustBuild(tid, stun.NewType(stun.MethodAllocate, stun.ClassSuccessResponse),
			&stun.XORMappedAddress{IP: net.IPv4(192, 0, 2, 1), Port: 50000}).Raw
	case AllocateUnauthorized:
		return stun.MustBuild(tid, errType,
			stun.ErrorCodeAttribute{Code: stun.CodeUnauthorized, Reason: []byte("Unauthorized")},
			stun.NewNonce("probetestnonce01"), stun.NewRealm("probetest")).Raw
	default:
		alt := s.cfg.Alternate
		if s.cfg.AlternateSelf {
			alt = s.Addr()
		}
		return stun.MustBuild(tid, errType,
			stun.ErrorCodeAttribute{Code: stun.CodeTryAlternate, Reason: []byte("Try Alternate")},
			stun.NewNonce("probetestnonce01"), stun.NewRealm("rtcmedia"),
			&stun.AlternateServer{IP: alt.Addr().AsSlice(), Port: int(alt.Port())}).Raw
	}
}

func (s *Server) binding(req *stun.Message, from *net.UDPAddr) []byte {
	switch s.cfg.Binding {
	case BindingSilent:
		return nil
	case BindingGarbage:
		return garbage
	default:
		return stun.MustBuild(stun.NewTransactionIDSetter(req.TransactionID), stun.BindingSuccess,
			&stun.XORMappedAddress{IP: from.IP, Port: from.Port + s.cfg.PortOffset}).Raw
	}
}
