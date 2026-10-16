package env

import (
	"context"
	"net/netip"
	"os"
	"strings"
)

const appPath = "/Applications/Windows App.app"

func collect(ctx context.Context, targets []netip.Addr) Report {
	r := Report{OS: "darwin"}
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

	if out, err := command(ctx, "/sbin/route", "-n", "get", "default"); err == nil {
		if rt, ok := parseBSDRouteGet(out); ok {
			r.DefaultRoute = &rt
		}
	} else if r.HasIPv4 {
		fail(err)
	}
	for _, t := range targets {
		out, err := command(ctx, "/sbin/route", "-n", "get", t.String())
		if err != nil {
			continue
		}
		if rt, ok := parseBSDRouteGet(out); ok {
			r.TargetRoutes = append(r.TargetRoutes, TargetRoute{Target: t, Route: rt, Tunnel: isTunnel(rt.Interface)})
		}
	}

	if out, err := command(ctx, "/usr/sbin/scutil", "--nc", "list"); err == nil {
		r.VPNServices = parseNCList(out)
	} else {
		fail(err)
	}
	if out, err := command(ctx, "/usr/sbin/scutil", "--proxy"); err == nil {
		r.Proxy = parseScutilProxy(out)
	} else {
		fail(err)
	}
	if !r.Proxy.Configured() {
		if p := proxyFromEnv(os.Getenv); p.Configured() {
			r.Proxy = p
		}
	}

	if out, err := command(ctx, "/bin/ps", "-axco", "comm"); err == nil {
		r.Agents = parseProcessList(out)
	}

	// The app's own settings (such as "Use UDP where possible") live in its
	// protected group container. Reading them would trigger a macOS privacy
	// prompt, so only the installed version is reported.
	if _, err := os.Stat(appPath); err == nil {
		app := &App{Path: appPath}
		if v, err := command(ctx, "/usr/bin/defaults", "read", appPath+"/Contents/Info", "CFBundleShortVersionString"); err == nil {
			app.Version = strings.TrimSpace(v)
		}
		if b, err := command(ctx, "/usr/bin/defaults", "read", appPath+"/Contents/Info", "CFBundleVersion"); err == nil {
			app.Build = strings.TrimSpace(b)
		}
		r.App = app
	}
	return r
}

// tunnelByKind has nothing to add to the name check here.
func tunnelByKind(string) bool { return false }
