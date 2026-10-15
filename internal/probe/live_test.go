package probe

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/cloudchristoph/avd-shortpath-test/internal/config"
)

// TestLive runs against Microsoft's real TURN relays. It needs network access
// and is skipped unless AVD_LIVE=1.
func TestLive(t *testing.T) {
	if os.Getenv("AVD_LIVE") != "1" {
		t.Skip("set AVD_LIVE=1 to run against world.turn.wvd.microsoft.com")
	}
	d := config.Default()
	r := Run(context.Background(), Config{TURNServer: d.TURNServer, Port: d.Port, Timeout: d.Timeout()})
	out, _ := json.MarshalIndent(r, "", "  ")
	t.Logf("%s", out)

	if r.DNS.Err != "" || len(r.DNS.Addrs) == 0 {
		t.Fatalf("dns: %+v", r.DNS)
	}
	if !r.TURNAccess {
		t.Fatalf("no TURN access: %+v", r.TURN)
	}
	if r.TURN[0].ErrorCode != 300 || r.TURN[0].Alternate == nil {
		t.Errorf("expected 300 Try Alternate with ALTERNATE-SERVER, got %+v", r.TURN[0])
	}
	if len(r.STUN) < 2 || r.NAT == NATNone {
		t.Errorf("stun %+v nat %s", r.STUN, r.NAT)
	}
}
