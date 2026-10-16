//go:build !windows

package main

// ownsConsole is false: there is no double click console case on Unix.
func ownsConsole() bool { return false }
