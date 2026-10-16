//go:build !darwin && !linux

package env

import (
	"context"
	"net/netip"
	"runtime"
)

func collect(ctx context.Context, targets []netip.Addr) Report {
	return Report{OS: runtime.GOOS, HasIPv4: true, Errors: []string{"environment checks are not implemented on " + runtime.GOOS}}
}

// tunnelByKind has nothing to add to the name check here.
func tunnelByKind(string) bool { return false }
