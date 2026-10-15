package env

import (
	"context"
	"net/netip"
	"os"
)

func collect(ctx context.Context, targets []netip.Addr) Report {
	r := Report{OS: "linux"}
	fail := func(err error) { r.Errors = append(r.Errors, err.Error()) }

	if ifs, err := interfaces(); err != nil {
		fail(err)
	} else {
		r.allInterfaces = ifs
		r.HasIPv4 = len(ifs) > 0
		for _, i := range ifs {
			if isTunnel(i.Name) {
				r.Tunnels = append(r.Tunnels, i)
			}
		}
	}

	ip := lookPath("ip", "/usr/sbin/ip", "/sbin/ip", "/usr/bin/ip", "/bin/ip")
	if ip == "" {
		fail(errNoIP)
	} else {
		if out, err := command(ctx, ip, "-4", "route", "show", "default"); err == nil {
			if rt, ok := parseIPRoute(out); ok {
				r.DefaultRoute = &rt
			}
		}
		for _, t := range targets {
			out, err := command(ctx, ip, "-4", "route", "get", t.String())
			if err != nil {
				continue
			}
			if rt, ok := parseIPRoute(out); ok {
				r.TargetRoutes = append(r.TargetRoutes, TargetRoute{Target: t, Route: rt, Tunnel: isTunnel(rt.Interface)})
			}
		}
	}

	r.Proxy = proxyFromEnv(os.Getenv)

	if ps := lookPath("ps", "/usr/bin/ps", "/bin/ps"); ps != "" {
		if out, err := command(ctx, ps, "-eo", "comm"); err == nil {
			r.Agents = parseProcessList(out)
		}
	}
	return r
}
