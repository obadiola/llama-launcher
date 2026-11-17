package main

import (
	"fmt"
	"os"
	"strings"
)

// Identidad de la app mostrada en la cabecera del menú.
const (
	// AppName — nombre mostrado en la cabecera.
	AppName = "llama-launcher"
	// AppVersion — versión mostrada en la cabecera (SemVer).
	AppVersion = "1.0.0"
)

// isTerminal — true si f es una consola (no pipe/redirección); evita escapes ANSI en scripts/tests.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// printHeader — cabecera del menú: nombre + versión + línea separadora + línea en blanco.
func printHeader() {
	fmt.Printf("%s v%s\n", AppName, AppVersion)
	fmt.Println(strings.Repeat("=", len(AppName)+len(AppVersion)+2))
	fmt.Println()
}
