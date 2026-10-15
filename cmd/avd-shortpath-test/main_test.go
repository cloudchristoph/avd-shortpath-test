package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudchristoph/avd-shortpath-test/internal/probe/probetest"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersion(t *testing.T) {
	code, out, _ := runCLI(t, "--version")
	if code != 0 || !strings.Contains(out, "not affiliated with or endorsed by Microsoft") {
		t.Errorf("exit %d, out %q", code, out)
	}
}

func TestBadArguments(t *testing.T) {
	for _, args := range [][]string{{"--bogus"}, {"--timeout", "0s"}, {"--servers", "/does/not/exist.json"}} {
		if code, _, _ := runCLI(t, args...); code != 3 {
			t.Errorf("%v: exit %d, want 3", args, code)
		}
	}
	if code, _, _ := runCLI(t, "--help"); code != 0 {
		t.Errorf("--help: exit %d", code)
	}
}

// End to end against the test server: Microsoft-like 300 with an alternate.
func TestEndToEnd(t *testing.T) {
	alt := probetest.Start(t, probetest.Config{PortOffset: 1})
	first := probetest.Start(t, probetest.Config{Alternate: alt.Addr()})

	servers := filepath.Join(t.TempDir(), "servers.json")
	cfg := fmt.Sprintf(`{"turnServer":"127.0.0.1","port":%d,"timeoutMs":500}`, first.Addr().Port())
	if err := os.WriteFile(servers, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	code, out, _ := runCLI(t, "--servers", servers)
	if code != 1 || !strings.Contains(out, "Your NAT type appears to be 'symmetric', but you have access to TURN servers.") {
		t.Errorf("text: exit %d\n%s", code, out)
	}

	code, out, _ = runCLI(t, "--servers", servers, "--json")
	var j struct {
		ExitCode int `json:"exitCode"`
		Result   struct {
			NAT string `json:"nat"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &j); err != nil || code != 1 || j.ExitCode != 1 || j.Result.NAT != "symmetric" {
		t.Errorf("json: exit %d err %v\n%s", code, err, out)
	}
}
