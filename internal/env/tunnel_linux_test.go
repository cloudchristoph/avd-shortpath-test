//go:build linux

package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTunnelByKind(t *testing.T) {
	root := t.TempDir()
	old := sysfsNet
	sysfsNet = root
	t.Cleanup(func() { sysfsNet = old })

	write := func(name, file, content string) {
		t.Helper()
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("oc-split", "uevent", "INTERFACE=oc-split\nDEVTYPE=wireguard\nIFINDEX=7\n")
	write("homevpn", "tun_flags", "0x1002\n")
	write("enp0s1", "uevent", "INTERFACE=enp0s1\nIFINDEX=2\n")

	for name, want := range map[string]bool{
		"oc-split": true,
		"homevpn":  true,
		"enp0s1":   false,
		"missing":  false,
		"wg0":      true, // by name, even without sysfs entry
		"../etc":   false,
	} {
		if got := isTunnel(name); got != want {
			t.Errorf("isTunnel(%q) = %v, want %v", name, got, want)
		}
	}
}
