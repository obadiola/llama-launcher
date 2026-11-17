// Command llama-launcher es un menú de consola que gestiona perfiles de modelos
// GGUF y los lanza con llama-server (llama.cpp).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"llama-launcher/internal/config"
	"llama-launcher/internal/runner"
	"llama-launcher/internal/ui"
)

// main — Load(config.Path()): si nil → defaults + profiles vacíos (SIN escaneo) + Save; menuLoop.
func main() {
	path := config.Path()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error al cargar la configuración:", err)
		os.Exit(1)
	}
	if cfg == nil {
		cfg = config.Default()
		if err := config.Save(path, cfg); err != nil {
			fmt.Fprintln(os.Stderr, "error al guardar la configuración:", err)
			os.Exit(1)
		}
	}
	menuLoop(path, cfg)
}

// menuLoop — bucle por LÍNEAS: limpia pantalla + cabecera (nombre/versión) + menú; número → lanza directamente;
// u/U → config.Update (único escaneo); 0 → salir; vacío/ENTER → repintar (NO lanza); otra entrada → "Opción no
// válida." + ENTER (la pantalla se limpia en el siguiente repintado).
func menuLoop(path string, cfg *config.Config) {
	for {
		ui.ClearScreen()
		ui.PrintHeader()
		enabled := config.Enabled(cfg.Profiles)
		printMenu(enabled)
		line, ok := ui.ReadLine()
		if !ok {
			return // EOF: salir
		}
		switch {
		case line == "":
			continue // ENTER/vacío: repintar
		case line == "0":
			fmt.Println("Adiós.")
			return
		case line == "u", line == "U":
			if uErr := config.Update(path, cfg); uErr != nil {
				fmt.Fprintln(os.Stderr, "error al actualizar la configuración:", uErr)
				ui.Pause("Pulsa ENTER para continuar...")
			}
		default:
			nOpt, errAtoi := strconv.Atoi(line)
			if errAtoi != nil || nOpt <= 0 || nOpt > len(enabled) {
				fmt.Println("Opción no válida.")
				ui.Pause("Pulsa ENTER para continuar...")
				continue
			}
			runProfile(cfg, enabled[nOpt-1])
		}
	}
}

// runProfile — lanza un perfil: stat del modelo (ausente → error + pause sin lanzar) → BuildArgs → mostrar comando →
// RunServer → pause si exitCode != 0 && !interrupted.
func runProfile(cfg *config.Config, p config.Profile) {
	file, okFile := config.String(p, "file")
	if !okFile || file == "" {
		fmt.Println("Perfil sin campo \"file\"; no se puede lanzar.")
		ui.Pause("Pulsa ENTER para continuar...")
		return
	}
	modelPath := filepath.Join(config.ModelsDirAbs(cfg.ModelsDir), file)
	if _, statErr := os.Stat(modelPath); statErr != nil {
		fmt.Println("No se encontró", modelPath)
		ui.Pause("Pulsa ENTER para continuar...")
		return
	}
	args, buildErr := runner.BuildArgs(p, cfg)
	if buildErr != nil {
		fmt.Println("Error al construir los argumentos:", buildErr)
		ui.Pause("Pulsa ENTER para continuar...")
		return
	}
	fmt.Printf("$ %s\n", strings.Join(args, " "))
	exitCode, interrupted, runErr := runner.RunServer(args)
	if runErr != nil {
		fmt.Println("Error al ejecutar:", runErr)
	}
	if exitCode != 0 && !interrupted {
		ui.Pause("Pulsa ENTER para continuar...")
	}
}

// printMenu — lista numerada de los perfiles recibidos (YA filtrados): `N) alias — file (port: X)`;
// pies [U]/[0]; nota si no hay habilitados.
func printMenu(entries []config.Profile) {
	if len(entries) == 0 {
		fmt.Println("No hay perfiles habilitados. Usa [U] para escanear modelos.")
	} else {
		for i, p := range entries {
			alias, _ := config.String(p, "alias")
			file, _ := config.String(p, "file")
			if alias == "" {
				alias = file
			}
			fmt.Printf("%d) %s — %s (port: %d)\n", i+1, alias, file, int(config.Float(p, "port")))
		}
	}
	fmt.Println("[U] Actualizar config")
	fmt.Println("[0] Salir")
}
