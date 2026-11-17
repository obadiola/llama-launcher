//go:build !windows

package main

import (
	"fmt"
	"os"
)

// clearScreen — limpia la pantalla con secuencia ANSI (ESC[2J ESC[H).
// No hace nada si stdout es un pipe/redirección.
func clearScreen() {
	if !isTerminal(os.Stdout) {
		return
	}
	fmt.Print("\x1b[2J\x1b[H")
}
