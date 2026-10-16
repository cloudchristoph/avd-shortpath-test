package env

import (
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseBSDRouteGet(t *testing.T) {
	r, ok := parseBSDRouteGet(fixture(t, "route_default.txt"))
	if !ok || r.Gateway != "192.168.1.1" || r.Interface != "en0" || r.Destination != "default" {
		t.Errorf("default: %+v", r)
	}
	r, ok = parseBSDRouteGet(fixture(t, "route_target_utun.txt"))
	if !ok || r.Interface != "utun5" || r.Gateway != "" || r.Destination != "51.5.0.0" {
		t.Errorf("utun: %+v", r)
	}
	if _, ok := parseBSDRouteGet("route: writing to routing socket: not in table\n"); ok {
		t.Error("error output parsed as route")
	}
}

func TestParseIPRoute(t *testing.T) {
	r, ok := parseIPRoute(fixture(t, "ip_route_get.txt"))
	if !ok || r.Gateway != "10.0.0.1" || r.Interface != "eth0" || r.Source != "10.0.0.5" {
		t.Errorf("get: %+v", r)
	}
	r, ok = parseIPRoute(fixture(t, "ip_route_get_wg.txt"))
	if !ok || r.Interface != "wg0" || r.Gateway != "" || !isTunnel(r.Interface) {
		t.Errorf("wg: %+v", r)
	}
	r, ok = parseIPRoute(fixture(t, "ip_route_default.txt"))
	if !ok || r.Gateway != "100.64.0.1" || r.Interface != "wwan0" {
		t.Errorf("default: %+v", r)
	}
	if _, ok := parseIPRoute(""); ok {
		t.Error("empty output parsed as route")
	}
}

func TestParseNCList(t *testing.T) {
	s := parseNCList(fixture(t, "nc_list.txt"))
	if len(s) != 3 {
		t.Fatalf("got %d services: %+v", len(s), s)
	}
	if s[0].Name != "Tailscale" || !s[0].Connected() || !s[0].Enabled || !strings.HasPrefix(s[0].Kind, "VPN") {
		t.Errorf("tailscale: %+v", s[0])
	}
	if s[1].Name != "Cisco Secure Client" || s[1].Connected() {
		t.Errorf("cisco: %+v", s[1])
	}
	if s[2].Name != "Office IPSec" || s[2].Enabled || s[2].Kind != "IPSec" {
		t.Errorf("ipsec: %+v", s[2])
	}
	if s := parseNCList(fixture(t, "nc_list_empty.txt")); len(s) != 0 {
		t.Errorf("empty: %+v", s)
	}
}

func TestParseScutilProxy(t *testing.T) {
	if p := parseScutilProxy(fixture(t, "proxy_none.txt")); p.Configured() || len(p.Exceptions) != 0 {
		t.Errorf("none: %+v", p)
	}
	p := parseScutilProxy(fixture(t, "proxy_full.txt"))
	if p.HTTP != "proxy.corp.example:8080" || p.HTTPS != "proxy.corp.example:8443" || p.SOCKS != "" {
		t.Errorf("hosts: %+v", p)
	}
	if p.PACURL != "http://wpad.corp.example/proxy.pac" || !p.AutoDiscovery || len(p.Exceptions) != 3 {
		t.Errorf("pac/wpad/exceptions: %+v", p)
	}
}

func TestProxyFromEnv(t *testing.T) {
	env := map[string]string{"HTTPS_PROXY": "http://proxy:3128", "no_proxy": "localhost,.corp"}
	p := proxyFromEnv(func(k string) string { return env[k] })
	if p.HTTPS != "http://proxy:3128" || p.HTTP != "" || len(p.Exceptions) != 2 || p.Source != "environment" {
		t.Errorf("%+v", p)
	}
	if p := proxyFromEnv(func(string) string { return "" }); p.Configured() {
		t.Errorf("empty env: %+v", p)
	}
}

func TestParseProcessListDocumentedNames(t *testing.T) {
	cases := map[string]string{
		"stAgentSvc":      "Netskope",
		"nsAuxiliarySvc":  "Netskope",
		"fctservctl2":     "FortiClient",
		"fctctld":         "FortiClient",
		"CloudflareWARP":  "Cloudflare WARP",
		"warp-svc":        "Cloudflare WARP",
		"Cloudflare WARP": "Cloudflare WARP",
		"tailscaled":      "Tailscale",
		"PanGPA":          "GlobalProtect",
		"ZscalerService":  "Zscaler",
		"launchd":         "",
	}
	for proc, want := range cases {
		got := strings.Join(parseProcessList(proc+"\n"), ",")
		if got != want {
			t.Errorf("%s: got %q, want %q", proc, got, want)
		}
	}
}

func TestParseProcessList(t *testing.T) {
	out := "launchd\nTailscale\nio.tailscale.ipn.macsys.network-extension\nZscalerTunnel\nPanGPS\nFinder\n"
	got := parseProcessList(out)
	want := []string{"Tailscale", "Zscaler", "GlobalProtect"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestVersionAtLeast(t *testing.T) {
	for _, c := range []struct {
		v    string
		want bool
	}{{"11.4.3", true}, {"11.3.8", true}, {"11.3.7", false}, {"10.9.10", false}, {"12", true}, {"11.3", false}} {
		if got := versionAtLeast(c.v, MinMacAppVersion); got != c.want {
			t.Errorf("%s: got %v", c.v, got)
		}
	}
}

func has(f []Finding, sev Severity, substr string) bool {
	for _, x := range f {
		if x.Severity == sev && strings.Contains(x.Text, substr) {
			return true
		}
	}
	return false
}

func TestFindingsClean(t *testing.T) {
	r := Report{HasIPv4: true, DefaultRoute: &Route{Gateway: "192.168.1.1", Interface: "en0"},
		TargetRoutes: []TargetRoute{{Target: netip.MustParseAddr("51.5.68.1"), Route: Route{Interface: "en0"}}}}
	if f := Findings(r); len(f) != 0 {
		t.Errorf("clean network gave findings: %+v", f)
	}
}

func TestFindingsVPNWithoutSplitTunnel(t *testing.T) {
	r := Report{HasIPv4: true,
		VPNServices: []VPNService{{Name: "Corp VPN", State: "Connected"}},
		TargetRoutes: []TargetRoute{
			{Target: netip.MustParseAddr("51.5.68.1"), Route: Route{Interface: "utun5"}, Tunnel: true},
			{Target: netip.MustParseAddr("51.5.68.7"), Route: Route{Interface: "utun5"}, Tunnel: true},
		}}
	f := Findings(r)
	if !has(f, Warning, "51.5.68.1, 51.5.68.7) goes through tunnel interface utun5 (connected VPN: Corp VPN)") {
		t.Errorf("%+v", f)
	}
	if has(f, Info, "split tunnel)") {
		t.Error("split tunnel info must not appear when the target is tunneled")
	}
}

func TestFindingsVPNWithSplitTunnel(t *testing.T) {
	// Like this Mac: Tailscale connected with a 100.x address on utun, but
	// the relays go out through en0. Neither tunnel warning nor CGNAT.
	r := Report{HasIPv4: true,
		DefaultRoute:  &Route{Gateway: "10.1.10.1", Interface: "en0"},
		VPNServices:   []VPNService{{Name: "Tailscale", State: "Connected"}},
		TargetRoutes:  []TargetRoute{{Target: netip.MustParseAddr("51.5.68.1"), Route: Route{Interface: "en0"}}},
		allInterfaces: []Interface{{Name: "utun4", Addrs: []netip.Prefix{netip.MustParsePrefix("100.74.136.89/32")}}},
		Agents:        []string{"Tailscale"},
	}
	f := Findings(r)
	if !has(f, Info, `VPN "Tailscale" is connected, but traffic to the TURN relays does not use it`) {
		t.Errorf("%+v", f)
	}
	if has(f, Info, "Tailscale is running") {
		t.Error("agent hint duplicates the VPN hint")
	}
	if has(f, Warning, "carrier grade NAT") || has(f, Warning, "tunnel interface") {
		t.Errorf("false warning: %+v", f)
	}
}

func TestFindingsCGNAT(t *testing.T) {
	gw := Report{HasIPv4: true, DefaultRoute: &Route{Gateway: "100.64.0.1", Interface: "wwan0"}}
	if f := Findings(gw); !has(f, Warning, "default gateway 100.64.0.1 is in 100.64.0.0/10") {
		t.Errorf("gateway: %+v", f)
	}
	addr := Report{HasIPv4: true, DefaultRoute: &Route{Gateway: "192.168.0.1", Interface: "en0"},
		allInterfaces: []Interface{{Name: "en0", Addrs: []netip.Prefix{netip.MustParsePrefix("100.100.5.9/24")}}}}
	if f := Findings(addr); !has(f, Warning, "Interface en0 has address 100.100.5.9") {
		t.Errorf("address: %+v", f)
	}
}

func TestFindingsNoIPv4(t *testing.T) {
	if f := Findings(Report{}); !has(f, Warning, "No IPv4 address") {
		t.Errorf("%+v", f)
	}
}

func TestFindingsProxyAndApp(t *testing.T) {
	r := Report{HasIPv4: true,
		Proxy: Proxy{Source: "system", HTTP: "proxy:8080", PACURL: "http://wpad/proxy.pac"},
		App:   &App{Version: "11.2.0"}}
	f := Findings(r)
	if !has(f, Info, "A proxy is configured (HTTP proxy:8080, PAC http://wpad/proxy.pac, from system)") {
		t.Errorf("proxy: %+v", f)
	}
	if !has(f, Warning, "Windows App 11.2.0 is installed") {
		t.Errorf("app: %+v", f)
	}
	r.App.Version = "11.4.3"
	if has(Findings(r), Warning, "Windows App") {
		t.Error("current app version must not warn")
	}
}
