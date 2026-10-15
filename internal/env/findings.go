package env

import (
	"fmt"
	"net/netip"
	"strings"
)

// MinMacAppVersion is the Windows App for macOS version Microsoft Learn lists
// for RDP Shortpath for public networks (beta 11.3.8 (3048)).
const MinMacAppVersion = "11.3.8"

// Findings derives hints from a report. Pure function, so every rule is
// covered by tests.
func Findings(r Report) []Finding {
	var f []Finding
	add := func(s Severity, format string, args ...any) {
		f = append(f, Finding{s, fmt.Sprintf(format, args...)})
	}

	if !r.HasIPv4 {
		add(Warning, "No IPv4 address on any interface. The TURN relays for RDP Shortpath are IPv4 only, so UDP Shortpath cannot work on this network. avdnettest.exe reports this as a DNS failure.")
	}

	tunneled := map[string][]string{}
	var order []string
	for _, t := range r.TargetRoutes {
		if !t.Tunnel {
			continue
		}
		if _, ok := tunneled[t.Interface]; !ok {
			order = append(order, t.Interface)
		}
		tunneled[t.Interface] = append(tunneled[t.Interface], t.Target.String())
	}
	for _, ifc := range order {
		add(Warning, "Traffic to the TURN relays (%s) goes through tunnel interface %s%s. Without a split tunnel for 51.5.0.0/16 UDP 3478, RDP Shortpath runs through the VPN, which usually means relay through the VPN or TCP fallback.",
			strings.Join(tunneled[ifc], ", "), ifc, vpnHint(r))
	}
	if len(order) == 0 {
		for _, v := range r.VPNServices {
			if v.Connected() {
				add(Info, "VPN %q is connected, but traffic to the TURN relays does not use it (split tunnel).", v.Name)
			}
		}
	}

	if p := r.Proxy; p.Configured() {
		add(Info, "A proxy is configured (%s). UDP Shortpath never uses HTTP proxies, but the TCP fallback goes through it. A proxy or secure web gateway often comes with extra NAT and TLS inspection on the TCP path.", proxySummary(p))
	}

	if r.DefaultRoute != nil {
		if gw, err := netip.ParseAddr(r.DefaultRoute.Gateway); err == nil && IsCGNAT(gw) {
			add(Warning, "The default gateway %s is in 100.64.0.0/10 (carrier grade NAT). Expect an extra NAT layer, which often makes the NAT symmetric and leaves only the TURN relay.", gw)
		} else if a := cgnatAddrOn(r, r.DefaultRoute.Interface); a.IsValid() && !isTunnel(r.DefaultRoute.Interface) {
			add(Warning, "Interface %s has address %s in 100.64.0.0/10 (carrier grade NAT). Expect an extra NAT layer, which often makes the NAT symmetric and leaves only the TURN relay.", r.DefaultRoute.Interface, a)
		}
	}

	for _, a := range r.Agents {
		if vpnNamed(r, a) {
			continue // already covered by the VPN service hint
		}
		add(Info, "%s is running. Check whether it tunnels or inspects UDP 3478 to 51.5.0.0/16.", a)
	}

	if app := r.App; app != nil && app.Version != "" && !versionAtLeast(app.Version, MinMacAppVersion) {
		add(Warning, "Windows App %s is installed. Microsoft lists RDP Shortpath for public networks on macOS from Windows App %s (beta) on.", app.Version, MinMacAppVersion)
	}
	return f
}

func vpnNamed(r Report, agent string) bool {
	for _, v := range r.VPNServices {
		if strings.EqualFold(v.Name, agent) {
			return true
		}
	}
	return false
}

func vpnHint(r Report) string {
	var names []string
	for _, v := range r.VPNServices {
		if v.Connected() {
			names = append(names, v.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return " (connected VPN: " + strings.Join(names, ", ") + ")"
}

func proxySummary(p Proxy) string {
	var parts []string
	if p.HTTP != "" {
		parts = append(parts, "HTTP "+p.HTTP)
	}
	if p.HTTPS != "" {
		parts = append(parts, "HTTPS "+p.HTTPS)
	}
	if p.SOCKS != "" {
		parts = append(parts, "SOCKS "+p.SOCKS)
	}
	if p.PACURL != "" {
		parts = append(parts, "PAC "+p.PACURL)
	}
	if p.AutoDiscovery {
		parts = append(parts, "auto discovery")
	}
	return strings.Join(parts, ", ") + ", from " + p.Source
}

func cgnatAddrOn(r Report, ifc string) netip.Addr {
	for _, t := range r.allInterfaces {
		if t.Name != ifc {
			continue
		}
		for _, p := range t.Addrs {
			if IsCGNAT(p.Addr()) {
				return p.Addr()
			}
		}
	}
	return netip.Addr{}
}
