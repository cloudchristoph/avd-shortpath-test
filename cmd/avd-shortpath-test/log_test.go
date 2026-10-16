package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudchristoph/avd-shortpath-test/internal/probe/probetest"
)

func TestInvalidLogLevel(t *testing.T) {
	code, _, errOut := runCLI(t, "--log-level", "loud")
	if code != 3 || !strings.Contains(errOut, "invalid log level") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestLogFile(t *testing.T) {
	alt := probetest.Start(t, probetest.Config{PortOffset: 1})
	srv := probetest.Start(t, probetest.Config{Alternate: alt.Addr()})
	servers := filepath.Join(t.TempDir(), "servers.json")
	cfg := fmt.Sprintf(`{"turnServer":"127.0.0.1","port":%d,"timeoutMs":500}`, srv.Addr().Port())
	if err := os.WriteFile(servers, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	// Level none: no file.
	none := filepath.Join(t.TempDir(), "none.log")
	runCLI(t, "--servers", servers, "--no-env", "--log-file", none)
	if _, err := os.Stat(none); !os.IsNotExist(err) {
		t.Errorf("log file created with level none: %v", err)
	}

	// Debug: file with the probe lines.
	path := filepath.Join(t.TempDir(), "debug.log")
	runCLI(t, "--servers", servers, "--no-env", "--log-level", "debug", "--log-file", path)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DNS 127.0.0.1 resolved", "Allocate request to", "Binding response from", "NAT decision"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("log lacks %q:\n%s", want, b)
		}
	}

	// Error level: probe progress is not logged.
	quiet := filepath.Join(t.TempDir(), "error.log")
	runCLI(t, "--servers", servers, "--no-env", "--log-level", "error", "--log-file", quiet)
	if b, _ := os.ReadFile(quiet); strings.Contains(string(b), "Allocate request") {
		t.Errorf("normal line at level error:\n%s", b)
	}
}
