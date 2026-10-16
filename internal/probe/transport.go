package probe

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/pion/stun/v3"

	"github.com/cloudchristoph/avd-shortpath-test/internal/logx"
)

var (
	errTimeout = errors.New("timeout")
	errInvalid = errors.New("response is not a valid STUN message")
)

// conn is the single UDP socket used for the whole run, like the original:
// every request goes out from the same local port.
type conn struct {
	udp     *net.UDPConn
	timeout time.Duration
	log     *logx.Logger // nil logs nothing
}

func openConn(timeout time.Duration) (*conn, error) {
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	return &conn{udp: udp, timeout: timeout}, nil
}

func (c *conn) localAddr() netip.AddrPort {
	return c.udp.LocalAddr().(*net.UDPAddr).AddrPort()
}

func (c *conn) close() error { return c.udp.Close() }

// roundTrip sends req once and waits up to the timeout for a STUN message
// with the same transaction ID from dst. There are no retransmissions, as in
// the original. STUN messages with other transaction IDs and datagrams from
// other addresses are ignored; a datagram from dst that is not STUN is
// reported as errInvalid.
func (c *conn) roundTrip(req *stun.Message, dst netip.AddrPort) (*stun.Message, time.Duration, error) {
	start := time.Now()
	c.log.Debugf("sending %d bytes to %s (timeout %s)", len(req.Raw), dst, c.timeout)
	if _, err := c.udp.WriteToUDPAddrPort(req.Raw, dst); err != nil {
		return nil, 0, fmt.Errorf("send: %w", err)
	}
	if err := c.udp.SetReadDeadline(start.Add(c.timeout)); err != nil {
		return nil, 0, err
	}
	buf := make([]byte, 1500)
	for {
		n, from, err := c.udp.ReadFromUDPAddrPort(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return nil, 0, errTimeout
			}
			return nil, 0, err
		}
		if unmap(from) != dst {
			continue
		}
		rtt := time.Since(start)
		c.log.Debugf("received %d bytes from %s after %s", n, from, rtt)
		if !stun.IsMessage(buf[:n]) {
			return nil, rtt, errInvalid
		}
		resp := &stun.Message{Raw: append([]byte(nil), buf[:n]...)}
		if err := resp.Decode(); err != nil {
			return nil, rtt, errInvalid
		}
		if resp.TransactionID != req.TransactionID {
			continue
		}
		return resp, rtt, nil
	}
}

func unmap(ap netip.AddrPort) netip.AddrPort {
	return netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
}

func statusOf(err error) Status {
	switch {
	case err == nil:
		return StatusOK
	case errors.Is(err, errTimeout):
		return StatusTimeout
	case errors.Is(err, errInvalid):
		return StatusInvalid
	default:
		return StatusError
	}
}
