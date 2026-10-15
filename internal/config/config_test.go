package config

import (
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	s := Default()
	if s.TURNServer != "world.turn.wvd.microsoft.com" || s.Port != 3478 || s.Timeout() != 10*time.Second {
		t.Fatalf("unexpected defaults: %+v", s)
	}
}

func TestParseRejectsIncomplete(t *testing.T) {
	for _, in := range []string{`{}`, `{"turnServer":"x","port":3478}`, `not json`} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q): want error", in)
		}
	}
}
