//go:build windows

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// enableVTProcessing — ENABLE_VIRTUAL_TERMINAL_PROCESSING de SetConsoleMode (Win10+).
const enableVTProcessing = 0x0004

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
	vtOnce             sync.Once
	vtOK               bool
)

// enableVT — activa ENABLE_VIRTUAL_TERMINAL_PROCESSING en stdout; true si quedó activo.
func enableVT() bool {
	h := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	if err := syscall.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	if mode&enableVTProcessing != 0 {
		return true
	}
	ok, _, _ := procSetConsoleMode.Call(uintptr(h), uintptr(mode|enableVTProcessing))
	return ok != 0
}

// colorEnabled — escapes ANSI (colores y `cls` por VT) solo si stdout es consola y VT está disponible;
// el intento se hace UNA sola vez (sync.Once) y se memoriza el resultado.
func colorEnabled() bool {
	if !isTerminal(os.Stdout) {
		return false
	}
	vtOnce.Do(func() { vtOK = enableVT() })
	return vtOK
}

// ClearScreen — limpia la consola Windows: ANSI (VT) si está disponible; si no, fallback `cmd /c cls`.
// No hace nada si stdout es un pipe/redirección (tests y scripts conservan la salida limpia).
func ClearScreen() {
	if !isTerminal(os.Stdout) {
		return
	}
	if colorEnabled() {
		fmt.Print("\x1b[2J\x1b[H")
		return
	}
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}
