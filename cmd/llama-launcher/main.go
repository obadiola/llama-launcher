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
	"llama-launcher/internal/preset"
	"llama-launcher/internal/runner"
	"llama-launcher/internal/ui"
)

// Parámetros fijos del modo servidor router ([S], para la Pi): host/port de llama-server.
const (
	serverModeHost = "0.0.0.0"
	serverModePort = "8080"
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
// u/U → config.Update (único escaneo); r/R → refreshConfig (relee config.json sin escanear); s/S → modo servidor
// (preset.ini + llama-server router); 0 → salir; vacío/ENTER → repintar (NO lanza); otra entrada → aviso + ENTER.
func menuLoop(path string, cfg *config.Config) {
	for {
		ui.ClearScreen()
		ui.PrintHeader()
		enabled := config.Enabled(cfg.Profiles)
		printMenu(enabled)
		fmt.Print("Introduce un código: ") // prompt de la opción (se borra al repintar)
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
		case line == "r", line == "R":
			if rErr := refreshConfig(path, cfg); rErr != nil {
				fmt.Println("No se pudo refrescar la configuración:", rErr)
				ui.Pause("Pulsa ENTER para continuar...")
			}
		case line == "s", line == "S":
			startServerMode(cfg)
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

// refreshConfig — relee config.json desde disco (SIN escaneo) y sustituye el contenido en memoria: sirve para
// aplicar cambios hechos con la app en marcha. Si el archivo no existe o es inválido devuelve error y la
// configuración actual queda intacta.
func refreshConfig(path string, cfg *config.Config) error {
	reloaded, err := config.Load(path)
	if err != nil {
		return err
	}
	if reloaded == nil {
		return fmt.Errorf("%s no existe", config.FileName)
	}
	*cfg = *reloaded
	return nil
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
	runArgs("", args)
}

// startServerMode — [S]: modo servidor router para la Pi. Genera/sustituye <modelsDir>/preset.ini con los
// perfiles habilitados y arranca `llama-server --models-preset <preset.ini> --port 8080 --host 0.0.0.0`
// (cwd = directorio del ejecutable, para resolver rutas relativas). NO escanea: usa la config en memoria.
func startServerMode(cfg *config.Config) {
	if len(config.Enabled(cfg.Profiles)) == 0 {
		fmt.Println("No hay perfiles habilitados. Usa [U] para escanear modelos.")
		ui.Pause("Pulsa ENTER para continuar...")
		return
	}
	presetPath := preset.Path(config.ModelsDirAbs(cfg.ModelsDir))
	if err := preset.Save(presetPath, cfg); err != nil {
		fmt.Println("No se pudo generar el preset:", err)
		ui.Pause("Pulsa ENTER para continuar...")
		return
	}
	fmt.Println("Preset generado:", presetPath)
	bin, err := runner.ResolveServerBin(cfg.ServerBin)
	if err != nil {
		fmt.Println("Error al resolver el servidor:", err)
		ui.Pause("Pulsa ENTER para continuar...")
		return
	}
	args := []string{bin, "--models-preset", presetPath, "--port", serverModePort, "--host", serverModeHost}
	runArgs(config.Dir(), args)
}

// runArgs — muestra el comando y lo ejecuta en primer plano; pausa solo si hubo fallo real
// (exitCode != 0 && !interrupted). dir = directorio de trabajo ("" = el actual).
func runArgs(dir string, args []string) {
	fmt.Printf("$ %s\n", strings.Join(args, " "))
	exitCode, interrupted, runErr := runner.RunServerIn(dir, args)
	if runErr != nil {
		fmt.Println("Error al ejecutar:", runErr)
	}
	if exitCode != 0 && !interrupted {
		ui.Pause("Pulsa ENTER para continuar...")
	}
}

// printMenu — lista numerada de los perfiles recibidos (YA filtrados): `N) alias — file (host:puerto)`;
// el fichero va en cian y host:puerto en amarillo (sin colores si la consola no admite ANSI);
// pies [U]/[R]/[S]/[0]; nota si no hay habilitados.
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
			host, _ := config.String(p, "host")
			if host == "" {
				host = "127.0.0.1" // host por defecto de llama-server
			}
			endpoint := fmt.Sprintf("%s:%d", host, int(config.Float(p, "port")))
			fmt.Printf("%d) %s — %s (%s)\n", i+1, alias,
				ui.Color(file, ui.ColorCyan),
				ui.Color(endpoint, ui.ColorYellow))
		}
	}
	fmt.Println("[U] Buscar nuevos modelos y actualizar config")
	fmt.Println("[R] Refrescar memoria con datos de config")
	fmt.Println("[S] Modo servidor (Pi): genera preset.ini y arranca llama-server")
	fmt.Println("[0] Salir")
}
