package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// Values of --pause and --color, as in avdnettest.exe.
const (
	modeAuto = "auto"
	modeOn   = "on"
	modeOff  = "off"
)

// validMode reports whether s is auto, on or off.
func validMode(s string) bool {
	return s == modeAuto || s == modeOn || s == modeOff
}

// useColor decides about colored output. noColorFlag is the --no-color alias
// for --color off. In auto mode NO_COLOR and a non-terminal disable color.
func useColor(mode string, noColorFlag bool, noColorEnv string, stdoutIsTerminal bool) bool {
	if noColorFlag {
		return false
	}
	switch mode {
	case modeOn:
		return true
	case modeOff:
		return false
	}
	return noColorEnv == "" && stdoutIsTerminal
}

// shouldPause decides whether to wait for Enter. on pauses only with an
// interactive stdin so scripts never hang; auto needs a console of its own.
func shouldPause(mode string, stdinIsTerminal, ownConsole bool) bool {
	switch mode {
	case modeOn:
		return stdinIsTerminal
	case modeAuto:
		return stdinIsTerminal && ownConsole
	}
	return false
}

// pauseAtEnd waits for Enter if the mode and the environment ask for it.
func pauseAtEnd(mode string, stdin *os.File, stderr io.Writer) {
	if !shouldPause(mode, isTerminalFile(stdin), ownsConsole()) {
		return
	}
	fmt.Fprintln(stderr, "Press Enter to exit")
	_, _ = bufio.NewReader(stdin).ReadString('\n')
}

func isTerminalFile(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
