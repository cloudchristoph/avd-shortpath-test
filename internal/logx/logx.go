// Package logx is a small leveled file logger with the levels of
// avdnettest.exe (--log-level, --log-file). A nil *Logger is valid and logs
// nothing, so callers never need a nil check.
package logx

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level orders the log levels; a message is written when its level is at or
// below the logger's level.
type Level int

const (
	None Level = iota
	Critical
	Error
	Warning
	Normal
	Debug
)

var names = [...]string{"none", "critical", "error", "warning", "normal", "debug"}

func (l Level) String() string {
	if l < None || int(l) >= len(names) {
		return "unknown"
	}
	return names[l]
}

// ParseLevel accepts the original's level names, case insensitive.
func ParseLevel(s string) (Level, error) {
	for i, n := range names {
		if strings.EqualFold(strings.TrimSpace(s), n) {
			return Level(i), nil
		}
	}
	return None, fmt.Errorf("invalid log level %q (use none, critical, error, warning, normal or debug)", s)
}

// Logger writes timestamped lines. It is safe for concurrent use.
type Logger struct {
	mu    sync.Mutex
	w     io.Writer
	c     io.Closer
	level Level
}

// Open returns a logger that appends to the file at path. With level None it
// returns nil and creates no file.
func Open(level Level, path string) (*Logger, error) {
	if level == None {
		return nil, nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &Logger{w: f, c: f, level: level}, nil
}

// New returns a logger that writes to w. With level None it returns nil.
func New(level Level, w io.Writer) *Logger {
	if level == None || w == nil {
		return nil
	}
	return &Logger{w: w, level: level}
}

// Close closes the log file, if any.
func (l *Logger) Close() error {
	if l == nil || l.c == nil {
		return nil
	}
	return l.c.Close()
}

// Enabled reports whether a message at level would be written.
func (l *Logger) Enabled(level Level) bool {
	return l != nil && level != None && level <= l.level
}

// Logf writes one line at the given level.
func (l *Logger) Logf(level Level, format string, args ...any) {
	if !l.Enabled(level) {
		return
	}
	line := fmt.Sprintf("%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), level, fmt.Sprintf(format, args...))
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.w, line)
}

func (l *Logger) Criticalf(format string, args ...any) { l.Logf(Critical, format, args...) }
func (l *Logger) Errorf(format string, args ...any)    { l.Logf(Error, format, args...) }
func (l *Logger) Warnf(format string, args ...any)     { l.Logf(Warning, format, args...) }
func (l *Logger) Normalf(format string, args ...any)   { l.Logf(Normal, format, args...) }
func (l *Logger) Debugf(format string, args ...any)    { l.Logf(Debug, format, args...) }

type ctxKey struct{}

// WithContext returns ctx carrying l, so packages can log without new
// parameters.
func WithContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the logger in ctx, or nil.
func FromContext(ctx context.Context) *Logger {
	l, _ := ctx.Value(ctxKey{}).(*Logger)
	return l
}
