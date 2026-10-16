package logx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	for i, n := range names {
		if l, err := ParseLevel(strings.ToUpper(n)); err != nil || l != Level(i) {
			t.Errorf("%s: %v %v", n, l, err)
		}
	}
	if _, err := ParseLevel("loud"); err == nil || !strings.Contains(err.Error(), "invalid log level") {
		t.Errorf("err = %v", err)
	}
}

func TestFiltering(t *testing.T) {
	var b bytes.Buffer
	l := New(Warning, &b)
	l.Criticalf("c")
	l.Errorf("e")
	l.Warnf("w")
	l.Normalf("n")
	l.Debugf("d")
	out := b.String()
	for _, want := range []string{"[critical] c", "[error] e", "[warning] w"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	for _, not := range []string{"] n", "] d"} {
		if strings.Contains(out, not) {
			t.Errorf("unexpected %q in %q", not, out)
		}
	}
}

func TestNilSafe(t *testing.T) {
	var l *Logger
	l.Debugf("x")
	if l.Enabled(Critical) || l.Close() != nil {
		t.Error("nil logger must be inert")
	}
	if FromContext(context.Background()) != nil {
		t.Error("expected nil logger")
	}
	if New(None, &bytes.Buffer{}) != nil {
		t.Error("none must give nil")
	}
}

func TestOpenCreatesFileOnlyWhenEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	l, err := Open(None, path)
	if err != nil || l != nil {
		t.Fatalf("none: %v %v", l, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file exists with level none: %v", err)
	}
	l, err = Open(Normal, path)
	if err != nil {
		t.Fatal(err)
	}
	l.Normalf("hello %d", 1)
	_ = l.Close()
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "[normal] hello 1") {
		t.Errorf("log = %q", b)
	}
}

func TestContext(t *testing.T) {
	l := New(Debug, &bytes.Buffer{})
	if FromContext(WithContext(context.Background(), l)) != l {
		t.Error("logger lost in context")
	}
}
