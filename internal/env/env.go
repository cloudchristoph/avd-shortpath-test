// Package env looks at the client environment that decides whether UDP
// reaches the TURN relays directly: routes, VPN tunnels, proxies, carrier
// grade NAT, known agents and, on macOS, the Windows App.
//
// System commands are run with fixed arguments only and their output is
// parsed; no user input is passed to them.
package env

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strings"
	"time"
)

// Report is what Collect found.
type Report struct {
	OS           string        `json:"os"`
	HasIPv4      bool          `json:"hasIPv4"`
	DefaultRoute *Route        `json:"defaultRoute,omitempty"`
	TargetRoutes []TargetRoute `json:"targetRoutes,omitempty"`
	Tunnels      []Interface   `json:"tunnels,omitempty"`
	VPNServices  []VPNService  `json:"vpnServices,omitempty"`
	Proxy        Proxy         `json:"proxy"`
	Agents       []string      `json:"agents,omitempty"`
	App          *App          `json:"windowsApp,omitempty"`
	Findings     []Finding     `json:"findings"`
	Errors       []string      `json:"errors,omitempty"`

	// allInterfaces holds every up interface with IPv4, for the CGNAT rule.
	allInterfaces []Interface
}

// Route is one routing table answer.
type Route struct {
	Destination string `json:"destination,omitempty"`
	Gateway     string `json:"gateway,omitempty"`
	Interface   string `json:"interface"`
	Source      string `json:"source,omitempty"`
}

// TargetRoute is the route to one probe target.
type TargetRoute struct {
	Target netip.Addr `json:"target"`
	Route
	Tunnel bool `json:"tunnel"`
}

// Interface is an up interface with its IPv4 addresses.
type Interface struct {
	Name  string         `json:"name"`
	Addrs []netip.Prefix `json:"addrs"`
}

// VPNService is a configured macOS network service of type VPN or IPSec.
type VPNService struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Enabled bool   `json:"enabled"`
}

// Connected reports whether the service is up.
func (v VPNService) Connected() bool { return v.State == "Connected" }

// Proxy is the configured proxy, from the system (macOS) or the environment.
type Proxy struct {
	Source        string   `json:"source"`
	HTTP          string   `json:"http,omitempty"`
	HTTPS         string   `json:"https,omitempty"`
	SOCKS         string   `json:"socks,omitempty"`
	PACURL        string   `json:"pacUrl,omitempty"`
	AutoDiscovery bool     `json:"autoDiscovery,omitempty"`
	Exceptions    []string `json:"exceptions,omitempty"`
}

// Configured reports whether any proxy is set.
func (p Proxy) Configured() bool {
	return p.HTTP != "" || p.HTTPS != "" || p.SOCKS != "" || p.PACURL != "" || p.AutoDiscovery
}

// App is the installed Windows App (macOS only).
type App struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Build   string `json:"build,omitempty"`
}

// Severity of a finding.
type Severity string

const (
	Warning Severity = "warning"
	Info    Severity = "info"
)

// Finding is a hint for the reader, derived from the report.
type Finding struct {
	Severity Severity `json:"severity"`
	Text     string   `json:"text"`
}

// Collect gathers the environment. targets are the probe servers; the route
// to each is checked.
func Collect(ctx context.Context, targets []netip.Addr) Report {
	r := collect(ctx, targets)
	r.Findings = Findings(r)
	return r
}

const commandTimeout = 3 * time.Second

// command runs a system command with fixed arguments and returns its output.
func command(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{"LANG=C", "LC_ALL=C", "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return string(out), nil
}

// interfaces returns up, non loopback interfaces with IPv4 addresses.
func interfaces() ([]Interface, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []Interface
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		var v4 []netip.Prefix
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipn.IP)
			if !ok || !ip.Unmap().Is4() || ip.IsLinkLocalUnicast() {
				continue
			}
			ones, _ := ipn.Mask.Size()
			v4 = append(v4, netip.PrefixFrom(ip.Unmap(), ones))
		}
		if len(v4) > 0 {
			out = append(out, Interface{Name: ifc.Name, Addrs: v4})
		}
	}
	return out, nil
}

var tunnelPrefixes = []string{"utun", "ipsec", "ppp", "tun", "tap", "wg", "tailscale", "zt", "gpd", "vti", "nordlynx", "cscotun"}

// isTunnel reports whether an interface name belongs to a VPN tunnel.
func isTunnel(name string) bool {
	for _, p := range tunnelPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// IsCGNAT reports whether a is in the shared address space of RFC 6598.
func IsCGNAT(a netip.Addr) bool { return cgnat.Contains(a) }
