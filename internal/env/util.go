package env

import (
	"errors"
	"os"
	"strings"
)

var errNoIP = errors.New("the ip command was not found")

func trim(s string) string { return strings.TrimSpace(s) }

// lookPath returns the first existing path. Fixed candidates instead of
// PATH lookups keep the command set predictable.
func lookPath(_ string, candidates ...string) string {
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}
