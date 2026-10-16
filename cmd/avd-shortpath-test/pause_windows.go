//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// ownsConsole reports whether this process is the only one on its console,
// which is the double click case. avdnettest.exe pauses on fewer than 2.
func ownsConsole() bool {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	var ids [2]uint32
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&ids[0])), 2)
	return n == 1
}
