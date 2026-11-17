//go:build windows

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// enableVTProcessing — ENABLE_VIRTUAL_TERMINAL_PROCESSING de SetConsoleMode (Win10+).
const enableVTProcessing = 0x0004

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

// ClearScreen — limpia la consola Windows: ANSI (VT) si el modo se pudo activar; si no, fallback `cmd /c cls`.
// No hace nada si stdout es un pipe/redirección (tests y scripts conservan la salida limpia).
func ClearScreen() {
	if !isTerminal(os.Stdout) {
		return
	}
	h := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	if err := syscall.GetConsoleMode(h, &mode); err == nil {
		if mode&enableVTProcessing == 0 {
			if ok, _, _ := procSetConsoleMode.Call(uintptr(h), uintptr(mode|enableVTProcessing)); ok != 0 {
				mode |= enableVTProcessing
			}
		}
		if mode&enableVTProcessing != 0 {
			fmt.Print("\x1b[2J\x1b[H")
			return
		}
	}
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}
