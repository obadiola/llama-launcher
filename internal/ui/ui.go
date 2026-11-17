// Package ui agrupa la identidad de la app, la cabecera del menú, la limpieza de
// pantalla y la entrada/salida por consola.
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Identidad de la app mostrada en la cabecera del menú.
const (
	// AppName — nombre mostrado en la cabecera.
	AppName = "llama-launcher"
	// AppVersion — versión mostrada en la cabecera (SemVer).
	AppVersion = "1.0.1"
)

// stdinReader — lector único global para ReadLine (pausas y menú).
var stdinReader = bufio.NewReader(os.Stdin)

// isTerminal — true si f es una consola (no pipe/redirección); evita escapes ANSI en scripts/tests.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// PrintHeader — cabecera del menú: nombre + versión + línea separadora + línea en blanco.
func PrintHeader() {
	fmt.Printf("%s v%s\n", AppName, AppVersion)
	fmt.Println(strings.Repeat("=", len(AppName)+len(AppVersion)+2))
	fmt.Println()
}

// ReadLine — línea de stdinReader (absorbe \r\n y BOM UTF-8); (text, ok=false) si EOF.
func ReadLine() (string, bool) {
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	line = strings.TrimRight(line, "\r\n")
	return strings.TrimPrefix(line, "\uFEFF"), true
}

// Pause — imprime prompt y espera ENTER (ReadLine); no bloquea si os.Stdin == nil.
func Pause(prompt string) {
	fmt.Println(prompt)
	if os.Stdin != nil {
		ReadLine()
	}
}
