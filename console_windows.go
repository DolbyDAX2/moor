//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// Windows GUI-subsystem binaries have no console at launch. CLI commands
// attach to the invoking terminal without allocating a new console window.
func prepareCLIConsole() {
	attach := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
	attached, _, _ := attach.Call(uintptr(^uint32(0))) // ATTACH_PARENT_PROCESS
	if attached == 0 {
		return
	} // already attached, or no parent console
	// Preserve redirected handles (pipes/files), but repair the invalid handles
	// captured by Go before AttachConsole initialized the console handles.
	for _, stream := range []struct {
		target **os.File
		id     uint32
		name   string
	}{
		{&os.Stdin, windows.STD_INPUT_HANDLE, "stdin"},
		{&os.Stdout, windows.STD_OUTPUT_HANDLE, "stdout"},
		{&os.Stderr, windows.STD_ERROR_HANDLE, "stderr"},
	} {
		handle := windows.Handle((*stream.target).Fd())
		if _, err := windows.GetFileType(handle); err == nil {
			continue
		}
		if handle, err := windows.GetStdHandle(stream.id); err == nil && handle != 0 && handle != windows.InvalidHandle {
			*stream.target = os.NewFile(uintptr(handle), stream.name)
		}
	}
}
