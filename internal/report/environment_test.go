package report

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"

	"github.com/cloudchristoph/avd-shortpath-test/internal/env"
	"github.com/cloudchristoph/avd-shortpath-test/internal/probe"
)

func TestEnvironmentSection(t *testing.T) {
	e := env.Report{
		HasIPv4:      true,
		DefaultRoute: &env.Route{Gateway: "192.168.1.1", Interface: "en0"},
		TargetRoutes: []env.TargetRoute{{Target: netip.MustParseAddr("51.5.68.1"), Route: env.Route{Interface: "utun5"}, Tunnel: true}},
		VPNServices:  []env.VPNService{{Name: "Corp VPN", State: "Connected"}},
		Tunnels:      []env.Interface{{Name: "utun5", Addrs: []netip.Prefix{netip.MustParsePrefix("10.8.0.2/32")}}},
		Proxy:        env.Proxy{Source: "system", PACURL: "http://wpad/proxy.pac"},
		Agents:       []string{"Zscaler"},
		App:          &env.App{Version: "11.4.3", Build: "3115"},
		Errors:       []string{"scutil: exit status 1"},
	}
	e.Findings = env.Findings(e)

	var b bytes.Buffer
	WriteText(&b, result(probe.NATCone, true), Options{Version: "test", Env: &e, Color: true})
	got := b.String()
	for _, s := range []string{
		"Default route      en0 via 192.168.1.1",
		"Route to relays    51.5.68.1 via utun5 (tunnel)",
		"VPN services       Corp VPN (connected)",
		"Tunnel interfaces  utun5 10.8.0.2/32",
		"Proxy              PAC http://wpad/proxy.pac (system)",
		"Agents             Zscaler",
		"Windows App        11.4.3 (3115)",
		"Not checked        scutil: exit status 1",
		"\x1b[33m  ! Traffic to the TURN relays (51.5.68.1) goes through tunnel interface utun5",
		"  i A proxy is configured",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q in:\n%s", s, got)
		}
	}
}

func TestEnvironmentSectionClean(t *testing.T) {
	e := env.Report{HasIPv4: true}
	var b bytes.Buffer
	WriteText(&b, result(probe.NATCone, true), Options{Version: "test", Env: &e})
	got := b.String()
	if !strings.Contains(got, "Proxy  none") || strings.Contains(got, "Hints") {
		t.Errorf("unexpected:\n%s", got)
	}
}
