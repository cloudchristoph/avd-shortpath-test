package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestUseColor(t *testing.T) {
	tests := []struct {
		mode    string
		flag    bool
		env     string
		term    bool
		want    bool
		comment string
	}{
		{"auto", false, "", true, true, "terminal"},
		{"auto", false, "", false, false, "not a terminal"},
		{"auto", false, "1", true, false, "NO_COLOR"},
		{"auto", true, "", true, false, "--no-color"},
		{"on", false, "", false, true, "forced"},
		{"on", false, "1", false, true, "forced beats NO_COLOR"},
		{"on", true, "", true, false, "--no-color wins"},
		{"off", false, "", true, false, "off"},
	}
	for _, tt := range tests {
		if got := useColor(tt.mode, tt.flag, tt.env, tt.term); got != tt.want {
			t.Errorf("%s: got %v", tt.comment, got)
		}
	}
}

func TestShouldPause(t *testing.T) {
	tests := []struct {
		mode       string
		stdin, own bool
		want       bool
	}{
		{"off", true, true, false},
		{"on", true, false, true},
		{"on", false, true, false},
		{"auto", true, true, true},
		{"auto", true, false, false},
		{"auto", false, true, false},
	}
	for _, tt := range tests {
		if got := shouldPause(tt.mode, tt.stdin, tt.own); got != tt.want {
			t.Errorf("%+v: got %v", tt, got)
		}
	}
}

func TestOwnsConsoleUnix(t *testing.T) {
	if ownsConsole() && runtime.GOOS != "windows" {
		t.Error("ownsConsole must be false off Windows")
	}
}

func TestInvalidPauseAndColor(t *testing.T) {
	for _, args := range [][]string{{"--pause", "maybe"}, {"--color", "rainbow"}} {
		code, _, errOut := runCLI(t, args...)
		if code != 3 || errOut == "" {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut)
		}
	}
}

func TestColorOnForcesColor(t *testing.T) {
	code, out, _ := runCLI(t, "--color", "on", "--no-env", "--timeout", "50ms", "--avd-turn-server", "127.0.0.1")
	if code == 0 && out == "" {
		t.Error("no output")
	}
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("no color codes: %q", out)
	}
}

func TestPauseDoesNotBlockWithoutTerminal(t *testing.T) {
	for _, m := range []string{"off", "auto", "on"} {
		if code, _, _ := runCLI(t, "--pause", m, "--version"); code != 0 {
			t.Errorf("--pause %s: exit %d", m, code)
		}
	}
}
