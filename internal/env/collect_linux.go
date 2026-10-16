package env

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
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

	ip := lookPath("/usr/sbin/ip", "/sbin/ip", "/usr/bin/ip", "/bin/ip")
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

	if ps := lookPath("/usr/bin/ps", "/bin/ps"); ps != "" {
		if out, err := command(ctx, ps, "-eo", "comm"); err == nil {
			r.Agents = parseProcessList(out)
		}
	}
	return r
}

var errNoIP = errors.New("the ip command was not found")

// lookPath returns the first existing path. Fixed candidates instead of
// PATH lookups keep the command set predictable.
func lookPath(candidates ...string) string {
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// sysfsNet is the directory with one entry per network interface.
var sysfsNet = "/sys/class/net"

// tunnelByKind recognizes tunnels whatever their name is: WireGuard (and a
// few other virtual tunnel types) report a DEVTYPE, tun and tap devices have
// a tun_flags file.
func tunnelByKind(name string) bool {
	if name == "" || strings.ContainsAny(name, "/\x00") {
		return false
	}
	dir := filepath.Join(sysfsNet, name)
	if _, err := os.Stat(filepath.Join(dir, "tun_flags")); err == nil {
		return true
	}
	b, err := os.ReadFile(filepath.Join(dir, "uevent"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if t, ok := strings.CutPrefix(line, "DEVTYPE="); ok {
			switch strings.TrimSpace(t) {
			case "wireguard", "ppp", "vti", "gre", "ipip", "sit":
				return true
			}
		}
	}
	return false
}
