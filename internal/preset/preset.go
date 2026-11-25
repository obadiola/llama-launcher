// Package preset genera el fichero preset.ini que consume llama-server en modo
// router (--models-preset) a partir de los perfiles de config.json.
package preset

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"llama-launcher/internal/config"
)

// FileName — nombre del fichero de presets (se guarda dentro del directorio de modelos).
const FileName = "preset.ini"

// ownKeys — claves propias de la app o controladas por el router: NO se escriben en el preset.
// host/port/api-key/alias los elimina o sobrescribe llama-server al cargar el modelo.
var ownKeys = map[string]bool{
	"alias":    true,
	"file":     true, // se traduce a "model"
	"model":    true, // siempre se genera desde "file"
	"disabled": true, // no es un argumento de llama.cpp: abortaría la carga del preset
	"host":     true,
	"port":     true,
	"api-key":  true,
}

// shortKeys — clave corta de config.json → nombre largo del argumento de llama.cpp.
// Ambas formas son válidas en el INI; se normaliza a la larga (formato canónico).
var shortKeys = map[string]string{
	"c":   "ctx-size",
	"ngl": "n-gpu-layers",
	"fa":  "flash-attn",
	"t":   "threads",
	"tb":  "threads-batch",
	"b":   "batch-size",
	"ub":  "ubatch-size",
	"n":   "n-predict",
	"np":  "parallel",
	"ctk": "cache-type-k",
	"ctv": "cache-type-v",
	"s":   "seed",
}

// Path — ruta del preset.ini: dentro del directorio de modelos.
func Path(modelsDirAbs string) string {
	return filepath.Join(modelsDirAbs, FileName)
}

// Build — contenido INI: una sección por perfil HABILITADO ([alias]), "model" con la ruta
// absoluta del .gguf y el resto de claves traducidas a argumentos largos y ordenadas.
// Los alias repetidos se renumeran (-2, -3...) para no perder modelos.
func Build(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString("; generado por llama-launcher [S]; se sustituye en cada arranque\n")
	modelsDirAbs := config.ModelsDirAbs(cfg.ModelsDir)
	used := make(map[string]int, len(cfg.Profiles))
	for _, p := range config.Enabled(cfg.Profiles) {
		file, okFile := config.String(p, "file")
		if !okFile || file == "" {
			fmt.Fprintf(os.Stderr, "warning: perfil sin \"file\" omitido en %s\n", FileName)
			continue
		}
		model := resolveModelPath(file, modelsDirAbs)
		b.WriteString("\n[")
		b.WriteString(uniqueSectionName(sectionName(p, file), used))
		b.WriteString("]\n")
		b.WriteString("model = " + model + "\n")
		opts := options(p, modelsDirAbs)
		keys := make([]string, 0, len(opts))
		for k := range opts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(k + " = " + opts[k] + "\n")
		}
	}
	return b.String()
}

// Save — genera el contenido y SUSTITUYE path (lo trunca si ya existía).
func Save(path string, cfg *config.Config) error {
	if err := os.WriteFile(path, []byte(Build(cfg)), 0o644); err != nil {
		return fmt.Errorf("escribir %s: %w", path, err)
	}
	return nil
}

// sectionName — nombre de sección: alias del perfil saneado (sin [] = ni saltos de línea);
// si falta, el nombre base del modelo.
func sectionName(p config.Profile, file string) string {
	name, _ := config.String(p, "alias")
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(file)
	}
	name = strings.TrimSpace(name)
	name = strings.NewReplacer("\r", " ", "\n", " ", "[", "(", "]", ")", "=", "-").Replace(name)
	if name == "" {
		return "model"
	}
	return name
}

// uniqueSectionName — alias repetido → sufijo -2, -3... (el parser INI guarda una sección por nombre).
func uniqueSectionName(name string, used map[string]int) string {
	used[name]++
	if n := used[name]; n > 1 {
		return fmt.Sprintf("%s-%d", name, n)
	}
	return name
}

// options — claves del perfil → (clave INI larga, valor); omite las claves propias/router y los
// bools false (igual que BuildArgs). Procesa en orden alfabético: si dos claves apuntan a la misma
// clave INI (p.ej. "n-gpu-layers" y "ngl"), gana la última, como al lanzar con flags.
// Las claves de tipo RUTA (mmproj) se resuelven contra modelsDirAbs (ruta completa en el preset).
func options(p config.Profile, modelsDirAbs string) map[string]string {
	out := make(map[string]string, len(p))
	for _, k := range sortedProfileKeys(p) {
		v := p[k]
		if b, isBool := v.(bool); isBool && !b {
			continue
		}
		val, ok := formatValue(v)
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: valor de %q no soportado en %s, se omite\n", k, FileName)
			continue
		}
		key := iniKey(k)
		if key == "mmproj" {
			val = resolveModelPath(val, modelsDirAbs)
		}
		out[key] = val
	}
	return out
}

// sortedProfileKeys — claves del perfil que van al preset, ordenadas alfabéticamente.
func sortedProfileKeys(p config.Profile) []string {
	keys := make([]string, 0, len(p))
	for k := range p {
		if ownKeys[iniKey(k)] || ownKeys[strings.TrimLeft(k, "-")] {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// iniKey — clave de config.json → clave INI (argumento de llama.cpp sin guiones):
// corta conocida → nombre largo; clave con guiones → sin guiones iniciales; resto tal cual.
func iniKey(key string) string {
	k := strings.TrimLeft(key, "-")
	if long, ok := shortKeys[k]; ok {
		return long
	}
	return k
}

// resolveModelPath — wrapper de config.ResolveModelPath con separadores "/", que llama.cpp acepta
// en Windows y Linux. Sin esto el preset dejaría solo el nombre y llama-server lo resolvería
// contra SU CWD.
func resolveModelPath(path, modelsDirAbs string) string {
	return filepath.ToSlash(config.ResolveModelPath(path, modelsDirAbs))
}

// formatValue — string tal cual; bool → "true"/"false"; números sin decimales innecesarios.
// ok=false para arrays/mapas (el INI admite un único valor por clave).
func formatValue(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case bool:
		return strconv.FormatBool(val), true
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
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(val), true
	default:
		return "", false
	}
}
