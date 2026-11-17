// Package runner traduce los perfiles de config.json a argumentos de llama-server
// y ejecuta el servidor en primer plano.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"llama-launcher/internal/config"
)

// shortFlags — flags con prefijo corto; 1 letra también "-"; 2+ letras "--".
var shortFlags = map[string]bool{"ngl": true, "fa": true}

// flagPrefix — prefijo de flag según longitud de clave / lista shortFlags; clave ya con "-" se usa tal cual.
func flagPrefix(key string) string {
	if strings.HasPrefix(key, "-") {
		return ""
	}
	if len(key) == 1 || shortFlags[key] {
		return "-"
	}
	return "--"
}

// resolveServerBin — resuelve bin (absoluto tal cual; relativo → config.Dir()); exec.LookPath;
// fallback llama-server.exe (Windows).
func resolveServerBin(bin string) (string, error) {
	if !filepath.IsAbs(bin) {
		bin = filepath.Join(config.Dir(), bin)
	}
	if p, err := exec.LookPath(bin); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("llama-server.exe"); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no se encontró %q en PATH", bin)
}

// formatJSONValue — string → literal; float64/float32 entero exacto → decimal; otro float → FormatFloat;
// enteros (int..uint64) → decimal; bool → "1"/"0"; ok=false solo si el tipo no soportado.
func formatJSONValue(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case float64:
		if val == math.Trunc(val) {
			return strconv.FormatInt(int64(val), 10), true
		}
		return strconv.FormatFloat(val, 'f', -1, 64), true
	case float32:
		f := float64(val)
		if f == math.Trunc(f) {
			return strconv.FormatInt(int64(f), 10), true
		}
		return strconv.FormatFloat(f, 'f', -1, 64), true
	case int:
		return strconv.FormatInt(int64(val), 10), true
	case int8:
		return strconv.FormatInt(int64(val), 10), true
	case int16:
		return strconv.FormatInt(int64(val), 10), true
	case int32:
		return strconv.FormatInt(int64(val), 10), true
	case int64:
		return strconv.FormatInt(val, 10), true
	case uint:
		return strconv.FormatUint(uint64(val), 10), true
	case uint8:
		return strconv.FormatUint(uint64(val), 10), true
	case uint16:
		return strconv.FormatUint(uint64(val), 10), true
	case uint32:
		return strconv.FormatUint(uint64(val), 10), true
	case uint64:
		return strconv.FormatUint(val, 10), true
	case bool:
		if val {
			return "1", true
		}
		return "0", true
	default:
		return "", false
	}
}

// BuildArgs — [bin, -m, <modelo>, --alias, <alias>] + claves restantes ordenadas alfabéticamente;
// resuelve config.ModelsDirAbs(cfg.ModelsDir) y resolveServerBin(cfg.ServerBin).
func BuildArgs(p config.Profile, cfg *config.Config) ([]string, error) {
	file, okFile := config.String(p, "file")
	if !okFile || file == "" {
		return nil, fmt.Errorf("perfil: campo \"file\" ausente o no es string")
	}
	alias, okAlias := config.String(p, "alias")
	if !okAlias || alias == "" {
		return nil, fmt.Errorf("perfil: campo \"alias\" ausente o no es string")
	}
	bin, err := resolveServerBin(cfg.ServerBin)
	if err != nil {
		return nil, err
	}

	args := []string{bin, "-m", filepath.Join(config.ModelsDirAbs(cfg.ModelsDir), file), "--alias", alias}

	keys := make([]string, 0, len(p))
	for k := range p {
		switch k {
		case "file", "alias", "disabled":
			continue
		}
		if v, isBool := p[k].(bool); isBool && !v {
			continue // bool false: se omite
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := p[k]
		switch val := v.(type) {
		case string:
			args = append(args, flagPrefix(k)+k, val)
		case bool:
			args = append(args, flagPrefix(k)+k) // true → flag sin valor (llama.cpp --jinja); false ya se omitió arriba
		case []any:
			for _, item := range val {
				s, okItem := formatJSONValue(item)
				if !okItem {
					fmt.Fprintf(os.Stderr, "warning: elemento de %q no soportado, se omite\n", k)
					continue
				}
				args = append(args, flagPrefix(k)+k, s)
			}
		default:
			s, okV := formatJSONValue(v)
			if !okV {
				fmt.Fprintf(os.Stderr, "warning: valor de %q no soportado (mapa anidado u otro tipo), se omite\n", k)
				continue
			}
			args = append(args, flagPrefix(k)+k, s)
		}
	}
	return args, nil
}

// RunServer — ejecuta el servidor de forma bloqueante; devuelve exitCode, interrupted (señal) y err.
func RunServer(args []string) (exitCode int, interrupted bool, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if os.Stdout != nil {
		cmd.Stdout = os.Stdout
	} else {
		cmd.Stdout = io.Discard
	}
	if os.Stderr != nil {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = io.Discard
	}

	startErr := make(chan error, 1)
	go func() { startErr <- cmd.Start() }()

	select {
	case <-sigCh:
		interrupted = true
		cancel() // señal al hijo vía CommandContext (CTRL_BREAK en Windows)
	case sErr := <-startErr:
		if sErr != nil {
			return 0, false, sErr
		}
	}

	wErr := cmd.Wait()
	if wErr != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.As(wErr, &exitErr):
			exitCode = exitErr.ExitCode()
		case interrupted:
			// salida por señal: no es error (el hijo terminó por la señal)
		default:
			err = wErr
		}
	}
	return exitCode, interrupted, err
}
