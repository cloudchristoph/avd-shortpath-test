package report

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/cloudchristoph/avd-shortpath-test/internal/env"
)

func writeEnvironment(p *printer, e *env.Report) {
	p.line("")
	p.line("Environment")
	tw := tabwriter.NewWriter(p.w, 0, 0, 2, ' ', 0)
	row := func(k, v string) { fmt.Fprintf(tw, "  %s\t%s\n", k, v) }

	row("IPv4", yesNo(e.HasIPv4))
	if d := e.DefaultRoute; d != nil {
		row("Default route", routeText(*d))
	}
	if len(e.TargetRoutes) > 0 {
		parts := make([]string, len(e.TargetRoutes))
		for i, t := range e.TargetRoutes {
			parts[i] = t.Target.String() + " via " + t.Interface
			if t.Tunnel {
				parts[i] += " (tunnel)"
			}
		}
		row("Route to relays", strings.Join(parts, ", "))
	}
	if len(e.VPNServices) > 0 {
		parts := make([]string, len(e.VPNServices))
		for i, v := range e.VPNServices {
			parts[i] = fmt.Sprintf("%s (%s)", v.Name, strings.ToLower(v.State))
		}
		row("VPN services", strings.Join(parts, ", "))
	}
	if len(e.Tunnels) > 0 {
		parts := make([]string, len(e.Tunnels))
		for i, t := range e.Tunnels {
			addrs := make([]string, len(t.Addrs))
			for j, a := range t.Addrs {
				addrs[j] = a.String()
			}
			parts[i] = t.Name + " " + strings.Join(addrs, " ")
		}
		row("Tunnel interfaces", strings.Join(parts, ", "))
	}
	if e.Proxy.Configured() {
		row("Proxy", proxyText(e.Proxy))
	} else {
		row("Proxy", "none")
	}
	if len(e.Agents) > 0 {
		row("Agents", strings.Join(e.Agents, ", "))
	}
	if a := e.App; a != nil {
		v := a.Version
		if a.Build != "" {
			v += " (" + a.Build + ")"
		}
		row("Windows App", v+", UDP setting not readable (protected app data)")
	}
	for _, err := range e.Errors {
		row("Not checked", err)
	}
	tw.Flush()

	if len(e.Findings) == 0 {
		return
	}
	p.line("")
	p.line("Hints")
	for _, f := range e.Findings {
		mark, c := "i", Color("")
		if f.Severity == env.Warning {
			mark, c = "!", Yellow
		}
		text := "  " + mark + " " + f.Text
		if c != "" {
			p.colored(c, text)
		} else {
			p.line("%s", text)
		}
	}
}

func routeText(r env.Route) string {
	if r.Gateway != "" {
		return r.Interface + " via " + r.Gateway
	}
	return r.Interface
}

func proxyText(p env.Proxy) string {
	var parts []string
	for _, kv := range [][2]string{{"HTTP", p.HTTP}, {"HTTPS", p.HTTPS}, {"SOCKS", p.SOCKS}, {"PAC", p.PACURL}} {
		if kv[1] != "" {
			parts = append(parts, kv[0]+" "+kv[1])
		}
	}
	if p.AutoDiscovery {
		parts = append(parts, "auto discovery")
	}
	return strings.Join(parts, ", ") + " (" + p.Source + ")"
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
