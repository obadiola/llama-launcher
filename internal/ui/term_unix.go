//go:build !windows

package ui

import (
	"fmt"
	"os"
)

// ClearScreen — limpia la pantalla con secuencia ANSI (ESC[2J ESC[H).
// No hace nada si stdout es un pipe/redirección.
func ClearScreen() {
	if !isTerminal(os.Stdout) {
		return
	}
	fmt.Print("\x1b[2J\x1b[H")
}

// colorEnabled — escapes ANSI solo si stdout es una consola (pipes/redirecciones quedan limpias).
func colorEnabled() bool {
	return isTerminal(os.Stdout)
}
