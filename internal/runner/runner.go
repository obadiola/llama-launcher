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

// shortFlags — claves con ALIAS CORTO (un solo guion) documentado en llama.cpp; el resto se
// emite con "--". Fuente: tools/server/README.md de llama.cpp (consulta 2026-11-25).
// Las claves de 1 letra (h, t, c, n, b, e, v, s, m, l, j, r, a, C) ya reciben "-" por longitud
// (ver flagPrefix), así que no hace falta listarlas. Los alias distinguen mayúsculas (Cr ≠ cr).
var shortFlags = map[string]bool{
	// CPU, hilos, batch y memoria
	"cl":    true, // -cl    --cache-list
	"tb":    true, // -tb    --threads-batch
	"Cr":    true, // -Cr    --cpu-range
	"Cb":    true, // -Cb    --cpu-mask-batch
	"Crb":   true, // -Crb   --cpu-range-batch
	"ub":    true, // -ub    --ubatch-size
	"kvo":   true, // -kvo   --kv-offload
	"nkvo":  true, // -nkvo  --no-kv-offload
	"nr":    true, // -nr    --no-repack
	"ctk":   true, // -ctk   --cache-type-k
	"ctv":   true, // -ctv   --cache-type-v
	"lm":    true, // -lm    --load-mode
	"lzm":   true, // -lzm   --lazy-mode
	"dev":   true, // -dev   --device
	"ot":    true, // -ot    --override-tensor
	"cmoe":  true, // -cmoe  --cpu-moe
	"ncmoe": true, // -ncmoe --n-cpu-moe
	"ncffn": true, // -ncffn --n-cpu-ffn
	"ngl":   true, // -ngl   --gpu-layers, --n-gpu-layers
	"sm":    true, // -sm    --split-mode
	"ts":    true, // -ts    --tensor-split
	"mg":    true, // -mg    --main-gpu
	"fit":   true, // -fit   --fit
	"fitt":  true, // -fitt  --fit-target
	"fitc":  true, // -fitc  --fit-ctx

	// Modelo
	"mu":  true, // -mu  --model-url
	"dr":  true, // -dr  --docker-repo
	"hf":  true, // -hf  --hf-repo
	"hfr": true, // -hfr --hf-repo
	"hff": true, // -hff --hf-file
	"hft": true, // -hft --hf-token

	// Log
	"lv": true, // -lv --verbosity, --log-verbosity

	// Sampling, JSON y caché
	"fa":     true, // -fa    --flash-attn
	"jf":     true, // -jf    --json-schema-file
	"bs":     true, // -bs    --backend-sampling
	"lcs":    true, // -lcs   --lookup-cache-static
	"lcd":    true, // -lcd   --lookup-cache-dynamic
	"ctxcp":  true, // -ctxcp --ctx-checkpoints, --swa-checkpoints
	"cms":    true, // -cms   --checkpoint-min-step
	"cram":   true, // -cram  --cache-ram
	"kvu":    true, // -kvu   --kv-unified
	"no-kvu": true, // -no-kvu --no-kv-unified

	// Servidor
	"sp":    true, // -sp    --special
	"np":    true, // -np    --parallel
	"cb":    true, // -cb    --cont-batching
	"nocb":  true, // -nocb  --no-cont-batching
	"mm":    true, // -mm    --mmproj
	"mmu":   true, // -mmu   --mmproj-url
	"mmdev": true, // -mmdev --mmproj-device
	"ag":    true, // -ag    --agent
	"no-ag": true, // -no-ag --no-agent
	"to":    true, // -to    --timeout
	"rea":   true, // -rea   --reasoning
	"sps":   true, // -sps   --slot-prompt-similarity

	// Modelo borrador (speculative decoding)
	"ctkd":   true, // -ctkd   --cache-type-k-draft
	"ctvd":   true, // -ctvd   --cache-type-v-draft
	"hfd":    true, // -hfd    --hf-repo-draft
	"hfrd":   true, // -hfrd   --hf-repo-draft
	"td":     true, // -td     --threads-draft
	"tbd":    true, // -tbd    --threads-batch-draft
	"Cd":     true, // -Cd     --cpu-mask-draft
	"Crd":    true, // -Crd    --cpu-range-draft
	"Cbd":    true, // -Cbd    --cpu-mask-batch-draft
	"otd":    true, // -otd    --override-tensor-draft
	"cmoed":  true, // -cmoed  --cpu-moe-draft
	"ncmoed": true, // -ncmoed --n-cpu-moe-draft
	"devd":   true, // -devd   --device-draft
	"ngld":   true, // -ngld   --gpu-layers-draft, --n-gpu-layers-draft
	"md":     true, // -md     --model-draft
}

// pathFlags — claves cuyo valor es la RUTA de un fichero del modelo. Si es relativa se resuelve
// contra modelsDir (igual que -m), para que llama-server la encuentre desde cualquier CWD.
var pathFlags = map[string]bool{
	"mmproj":             true,
	"lora":               true,
	"control-vector":     true,
	"grammar-file":       true,
	"chat-template-file": true,
	"model-draft":        true,
}

// resolvePathValue — valor de una clave de ruta (pathFlags): relativo → bajo modelsDirAbs vía
// config.ResolveModelPath; absoluto o URL se deja tal cual; el resto de claves no se tocan.
func resolvePathValue(key, val, modelsDirAbs string) string {
	if !pathFlags[strings.TrimLeft(key, "-")] {
		return val
	}
	return config.ResolveModelPath(val, modelsDirAbs)
}

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

// ResolveServerBin — resuelve bin (absoluto tal cual; relativo → config.Dir()); exec.LookPath;
// fallback llama-server.exe (Windows).
func ResolveServerBin(bin string) (string, error) {
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
	bin, err := ResolveServerBin(cfg.ServerBin)
	if err != nil {
		return nil, err
	}

	modelsDirAbs := config.ModelsDirAbs(cfg.ModelsDir)
	args := []string{bin, "-m", filepath.Join(modelsDirAbs, file), "--alias", alias}

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
			args = append(args, flagPrefix(k)+k, resolvePathValue(k, val, modelsDirAbs))
		case bool:
			args = append(args, flagPrefix(k)+k) // true → flag sin valor (llama.cpp --jinja); false ya se omitió arriba
		case []any:
			for _, item := range val {
				s, okItem := formatJSONValue(item)
				if !okItem {
					fmt.Fprintf(os.Stderr, "warning: elemento de %q no soportado, se omite\n", k)
					continue
				}
				if _, isStr := item.(string); isStr {
					s = resolvePathValue(k, s, modelsDirAbs) // rutas en arrays (p. ej. varios --lora)
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

// RunServer — ejecuta el servidor de forma bloqueante en el directorio actual;
// devuelve exitCode, interrupted (señal) y err.
func RunServer(args []string) (exitCode int, interrupted bool, err error) {
	return RunServerIn("", args)
}

// RunServerIn — como RunServer pero con directorio de trabajo dir ("" = el actual): permite resolver
// rutas relativas al ejecutable (p. ej. ./motor/llama-server y ./models/preset.ini del modo servidor).
func RunServerIn(dir string, args []string) (exitCode int, interrupted bool, err error) {
	if len(args) == 0 {
		return 0, false, errors.New("argumentos vacíos")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
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
