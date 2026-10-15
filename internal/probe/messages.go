package probe

import (
	"encoding/binary"
	"net/netip"

	"github.com/pion/stun/v3"
)

const software = "avd-shortpath-test"

// newAllocate builds an unauthenticated TURN Allocate request with the same
// attributes the original sends, except SOFTWARE.
func newAllocate() *stun.Message {
	m := stun.MustBuild(stun.TransactionID,
		stun.NewType(stun.MethodAllocate, stun.ClassRequest),
		stun.NewSoftware(software))
	m.Add(stun.AttrRequestedTransport, []byte{17, 0, 0, 0}) // UDP
	lifetime := make([]byte, 4)
	binary.BigEndian.PutUint32(lifetime, 3600)
	m.Add(stun.AttrLifetime, lifetime)
	m.Add(stun.AttrDontFragment, nil)
	m.Add(stun.AttrRequestedAddressFamily, []byte{1, 0, 0, 0}) // IPv4
	m.WriteHeader()
	return m
}

func newBinding() *stun.Message {
	return stun.MustBuild(stun.TransactionID, stun.BindingRequest)
}

// parseAllocateResponse fills the TURN fields of r from any STUN response.
// Whatever the class and code, a well formed response means TURN is reachable.
func parseAllocateResponse(m *stun.Message, r *TURNResult) {
	r.Class = m.Type.Class.String()
	var ec stun.ErrorCodeAttribute
	if ec.GetFrom(m) == nil {
		r.ErrorCode = int(ec.Code)
		r.ErrorReason = string(ec.Reason)
	}
	var realm stun.Realm
	if realm.GetFrom(m) == nil {
		r.Realm = string(realm)
	}
	var nonce stun.Nonce
	r.HasNonce = nonce.GetFrom(m) == nil
	var alt stun.MappedAddress
	if alt.GetFromAs(m, stun.AttrAlternateServer) == nil {
		if a, ok := netip.AddrFromSlice(alt.IP); ok {
			ap := netip.AddrPortFrom(a.Unmap(), uint16(alt.Port))
			r.Alternate = &ap
		}
	}
}

// mappedAddress reads XOR-MAPPED-ADDRESS, falling back to MAPPED-ADDRESS.
func mappedAddress(m *stun.Message) (netip.AddrPort, bool) {
	var x stun.XORMappedAddress
	if x.GetFrom(m) == nil {
		if a, ok := netip.AddrFromSlice(x.IP); ok {
			return netip.AddrPortFrom(a.Unmap(), uint16(x.Port)), true
		}
	}
	var p stun.MappedAddress
	if p.GetFrom(m) == nil {
		if a, ok := netip.AddrFromSlice(p.IP); ok {
			return netip.AddrPortFrom(a.Unmap(), uint16(p.Port)), true
		}
	}
	return netip.AddrPort{}, false
}
