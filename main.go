package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfigFile — nombre del config.json; la ruta SIEMPRE se resuelve junto al ejecutable (configPath()).
const ConfigFile = "config.json"

// stdinReader — lector único global para readLine (pausas y menú).
var stdinReader = bufio.NewReader(os.Stdin)

// main — loadConfig(configPath()): si nil → defaults + profiles vacíos (SIN escaneo) + saveConfig; menuLoop.
func main() {
	path := configPath()
	cfg, err := loadConfig(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error al cargar la configuración:", err)
		os.Exit(1)
	}
	if cfg == nil {
		cfg = &Config{ModelsDir: "models", ServerBin: "llama-server", Profiles: make([]Profile, 0)}
		if err := saveConfig(path, cfg); err != nil {
			fmt.Fprintln(os.Stderr, "error al guardar la configuración:", err)
			os.Exit(1)
		}
	}
	menuLoop(path, cfg)
}

// menuLoop — bucle por LÍNEAS: limpia pantalla + cabecera (nombre/versión) + menú; número → lanza directamente;
// u/U → updateConfig (único escaneo); 0 → salir; vacío/ENTER → repintar (NO lanza); otra entrada → "Opción no
// válida." + ENTER (la pantalla se limpia en el siguiente repintado).
func menuLoop(path string, cfg *Config) {
	for {
		clearScreen()
		printHeader()
		printMenu(enabledProfiles(cfg.Profiles))
		line, ok := readLine()
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
			if uErr := updateConfig(path, cfg); uErr != nil {
				fmt.Fprintln(os.Stderr, "error al actualizar la configuración:", uErr)
				pause("Pulsa ENTER para continuar...")
			}
		default:
			nOpt, errAtoi := strconv.Atoi(line)
			if errAtoi != nil || nOpt <= 0 {
				fmt.Println("Opción no válida.")
				pause("Pulsa ENTER para continuar...")
				continue
			}
			enabled := enabledProfiles(cfg.Profiles)
			if nOpt > len(enabled) {
				fmt.Println("Opción no válida.")
				pause("Pulsa ENTER para continuar...")
				continue
			}
			runProfile(path, cfg, enabled[nOpt-1])
		}
	}
}

// runProfile — lanza un perfil: stat del modelo (ausente → error + pause sin lanzar) → buildArgs → mostrar comando →
// runServer → pause si exitCode != 0 && !interrupted.
func runProfile(path string, cfg *Config, p Profile) {
	file, okFile := profileString(p, "file")
	if !okFile || file == "" {
		fmt.Println("Perfil sin campo \"file\"; no se puede lanzar.")
		pause("Pulsa ENTER para continuar...")
		return
	}
	modelPath := filepath.Join(modelsDirAbs(cfg.ModelsDir), file)
	if _, statErr := os.Stat(modelPath); statErr != nil {
		fmt.Println("No se encontró", modelPath)
		pause("Pulsa ENTER para continuar...")
		return
	}
	args, buildErr := buildArgs(p, cfg)
	if buildErr != nil {
		fmt.Println("Error al construir los argumentos:", buildErr)
		pause("Pulsa ENTER para continuar...")
		return
	}
	fmt.Printf("$ %s\n", strings.Join(args, " "))
	exitCode, interrupted, runErr := runServer(args)
	if runErr != nil {
		fmt.Println("Error al ejecutar:", runErr)
	}
	if exitCode != 0 && !interrupted {
		pause("Pulsa ENTER para continuar...")
	}
}

// enabledProfiles — copia de entradas con profileBool(p,"disabled") != true, en orden original.
func enabledProfiles(entries []Profile) []Profile {
	out := make([]Profile, 0, len(entries))
	for _, p := range entries {
		if profileBool(p, "disabled") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// printMenu — lista numerada de habilitados: `N) alias — file (port: X)`; pies [U]/[0]; nota si no hay.
func printMenu(entries []Profile) {
	enabled := enabledProfiles(entries)
	if len(enabled) == 0 {
		fmt.Println("No hay perfiles habilitados. Usa [U] para escanear modelos.")
	} else {
		for i, p := range enabled {
			alias, _ := profileString(p, "alias")
			file, _ := profileString(p, "file")
			if alias == "" {
				alias = file
			}
			fmt.Printf("%d) %s — %s (port: %d)\n", i+1, alias, file, int(profileFloat(p, "port")))
		}
	}
	fmt.Println("[U] Actualizar config")
	fmt.Println("[0] Salir")
}

// readLine — línea de stdinReader (absorbe \r\n y BOM UTF-8); (text, ok=false) si EOF.
func readLine() (string, bool) {
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	line = strings.TrimRight(line, "\r\n")
	return strings.TrimPrefix(line, "\uFEFF"), true
}

// pause — imprime prompt y espera ENTER (readLine); no bloquea si os.Stdin == nil.
func pause(prompt string) {
	fmt.Println(prompt)
	if os.Stdin != nil {
		readLine()
	}
}
