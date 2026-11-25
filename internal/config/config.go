// Package config gestiona config.json: perfiles de modelos, rutas de trabajo y
// escaneo de modelos .gguf. Sin dependencias externas (solo stdlib).
package config

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FileName — nombre del config.json; la ruta SIEMPRE se resuelve junto al ejecutable (Path()).
const FileName = "config.json"

// Profile — perfil de modelo (esquema abierto: cualquier clave extra se conserva).
type Profile = map[string]any

// Config — raíz de config.json v3: {"modelsDir","serverBin","defaults","profiles"}.
// Defaults son los valores que hereda cada modelo NUEVO (los añade Update/escaneo).
type Config struct {
	ModelsDir string    `json:"modelsDir"`
	ServerBin string    `json:"serverBin"`
	Defaults  Profile   `json:"defaults"`
	Profiles  []Profile `json:"profiles"`
}

// Default — configuración inicial (SIN escaneo): modelsDir "models", serverBin "llama-server",
// defaults de fábrica y profiles vacío.
func Default() *Config {
	return &Config{ModelsDir: "models", ServerBin: "llama-server", Defaults: builtinDefaults(), Profiles: make([]Profile, 0)}
}

// builtinDefaults — valores de fábrica de un modelo NUEVO (config.json → "defaults").
// api-key es la clave del flag --api-key de llama-server: un UUID v4 distinto por configuración.
func builtinDefaults() Profile {
	return Profile{
		"host":    "127.0.0.1",
		"port":    8080,
		"c":       8192,
		"ngl":     99,
		"t":       8,
		"fa":      "on",
		"api-key": newUUID(),
	}
}

// newUUID — UUID v4 (8-4-4-4-12) a partir de crypto/rand; si rand falla, se deriva del reloj.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t := uint64(time.Now().UnixNano())
		for i := 0; i < 8; i++ {
			b[i] = byte(t >> (8 * (7 - i)))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40 // versión 4
	b[8] = (b[8] & 0x3f) | 0x80 // variante RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ensureDefaults — crea cfg.Defaults si es nil y añade cada clave ausente de builtinDefaults()
// (incluida api-key, que se genera con un UUID nuevo); true = hubo cambios (el llamador debe guardar).
func ensureDefaults(cfg *Config) bool {
	changed := false
	if cfg.Defaults == nil {
		cfg.Defaults = make(Profile, 8)
		changed = true
	}
	for k, v := range builtinDefaults() {
		if _, ok := cfg.Defaults[k]; !ok {
			cfg.Defaults[k] = v
			changed = true
		}
	}
	return changed
}

// Dir — directorio del ejecutable (os.Executable); si falla → ".".
func Dir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

// Path — ruta del config.json: SIEMPRE junto al ejecutable.
func Path() string {
	return filepath.Join(Dir(), FileName)
}

// sanitizeJSONBackslashes — convierte \X dentro de strings (X no escape JSON válido) en \\X
// (rutas Windows C:\AI → C:\\AI); deja intactas las escapes válidas; idempotente ante JSON válido.
func sanitizeJSONBackslashes(data []byte) []byte {
	var out []byte
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			if c == '\\' {
				if i+1 >= len(data) {
					out = append(out, '\\', '\\') // backslash al final: escaparlo
					continue
				}
				next := data[i+1]
				switch next {
				case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
					out = append(out, c, next) // escape válido: copiar ambos
					i++                        // saltar el carácter de escape
					continue
				case 'u':
					hexOK := true
					for j := i + 2; j < i+6; j++ {
						if j >= len(data) {
							hexOK = false
							break
						}
						h := data[j]
						if !((h >= '0' && h <= '9') || (h >= 'a' && h <= 'f') || (h >= 'A' && h <= 'F')) {
							hexOK = false
							break
						}
					}
					if hexOK {
						out = append(out, c, next) // \uXXXX válido
						i++                        // saltar el 'u'; los 4 hex se copian después como literales
						continue
					}
				}
				out = append(out, '\\', '\\') // backslash suelto (ruta Windows): escaparlo
				continue
			}
			if c == '"' {
				inString = false
			}
			out = append(out, c)
			continue
		}
		if c == '"' {
			inString = true
		}
		out = append(out, c)
	}
	return out
}

// utf8BOM — marca de orden de bytes UTF-8 (Notepad/PowerShell antiguo la añaden al guardar).
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Load — si no existe → nil, nil; parsea objeto a *Config (sanitizando backslashes si el unmarshal falla);
// aplica defaults si ModelsDir/ServerBin vacíos.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("leer %s: %w", path, err)
	}
	data = bytes.TrimPrefix(data, utf8BOM) // tolerar BOM UTF-8 (Notepad / Out-File)
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Reintentar saneando backslashes no escapados (rutas Windows malformadas).
		sanitized := sanitizeJSONBackslashes(data)
		if err2 := json.Unmarshal(sanitized, &cfg); err2 != nil {
			return nil, fmt.Errorf("parsear %s: %w", path, err)
		}
		// Persistir la corrección: reescribir el archivo con los bytes saneados (config corregido en disco).
		if wErr := os.WriteFile(path, sanitized, 0o644); wErr != nil {
			fmt.Fprintf(os.Stderr, "warning: no se pudo guardar %s corregido: %v\n", path, wErr)
		}
	}
	if cfg.ModelsDir == "" {
		cfg.ModelsDir = "models"
	}
	if cfg.ServerBin == "" {
		cfg.ServerBin = "llama-server"
	}
	// Crear/completar "defaults" en disco si faltaba (config.json autoexplicativo).
	if ensureDefaults(&cfg) {
		if sErr := Save(path, &cfg); sErr != nil {
			fmt.Fprintf(os.Stderr, "warning: no se pudo guardar %s con los defaults: %v\n", path, sErr)
		}
	}
	return &cfg, nil
}

// Save — json.MarshalIndent (2 espacios) + salto de línea final.
func Save(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar config: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// scanModels — filepath.WalkDir recursivo en dir; rutas relativas .gguf (insensible), ordenadas;
// salta base que empieza por mmproj- (case-insensitive): son el parámetro --mmproj de un modelo superior, no modelos IA.
func scanModels(dir string) ([]string, error) {
	var models []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".gguf") {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(strings.ToLower(base), "mmproj-") {
			return nil // mmproj: parámetro --mmproj, no modelo IA
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		models = append(models, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("escanear %s: %w", dir, err)
	}
	sort.Strings(models)
	return models, nil
}

// defaultProfile — perfil de un modelo NUEVO: copia de defaults (host/port/c/ngl/t/fa/api-key y cualquier
// clave extra del usuario) + alias (nombre base sin .gguf), file y disabled:false.
func defaultProfile(modelFile string, defaults Profile) Profile {
	base := filepath.Base(modelFile)
	alias := base
	if i := strings.LastIndex(alias, "."); i > 0 && strings.EqualFold(alias[i:], ".gguf") {
		alias = alias[:i]
	}
	p := make(Profile, len(defaults)+3)
	for k, v := range defaults {
		p[k] = v
	}
	p["alias"] = alias
	p["file"] = modelFile
	p["disabled"] = false
	return p
}

// normalizeModelPath — clave de comparación de rutas relativas de modelo: separador "/" y minúsculas
// (Windows: FS y rutas de config.json insensibles a caja/separador; en Linux se acepta la tolerancia).
func normalizeModelPath(rel string) string {
	return strings.ToLower(filepath.ToSlash(rel))
}

// markMissing — pone disabled:true en los perfiles cuyo "file" NO aparece en el escaneo (modelo borrado o
// renombrado); no toca los ya deshabilitados (un disabled manual se respeta) ni reactiva ninguno.
// true = hubo cambios.
func markMissing(cfg *Config, models []string) bool {
	present := make(map[string]bool, len(models))
	for _, m := range models {
		present[normalizeModelPath(m)] = true
	}
	changed := false
	for _, p := range cfg.Profiles {
		file, ok := String(p, "file")
		if !ok || file == "" || present[normalizeModelPath(file)] {
			continue
		}
		if !Bool(p, "disabled") {
			p["disabled"] = true
			changed = true
		}
	}
	return changed
}

// companionJSONPath — resuelve la ruta del JSON compañero de un modelo (exacto o case-insensitive en el mismo dir); "" si no existe.
func companionJSONPath(modelsDirAbs, modelFile string) string {
	companion := strings.TrimSuffix(filepath.Join(modelsDirAbs, modelFile), filepath.Ext(modelFile)) + ".json"
	if _, err := os.Stat(companion); err == nil {
		return companion
	}
	dir := filepath.Dir(filepath.Join(modelsDirAbs, modelFile))
	base := strings.TrimSuffix(filepath.Base(modelFile), filepath.Ext(modelFile))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), base+".json") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// profileFromCompanionJSON — lee el JSON compañero de un modelo; admite un objeto único o un ARRAY de
// configuraciones (una por alias); siempre sobreescribe file por modelFile (ruta relativa);
// ok=false si no existe o es inválido.
func profileFromCompanionJSON(modelsDirAbs, modelFile string) ([]Profile, bool) {
	path := companionJSONPath(modelsDirAbs, modelFile)
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var raw []Profile
	if err := json.Unmarshal(data, &raw); err != nil {
		// Compatibilidad: el JSON puede ser un objeto único en lugar de un array.
		var single Profile
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return nil, false
		}
		raw = []Profile{single}
	}
	profiles := make([]Profile, 0, len(raw))
	for _, p := range raw {
		if p == nil {
			continue
		}
		p["file"] = modelFile // siempre ruta relativa (para el menú/stat)
		profiles = append(profiles, p)
	}
	if len(profiles) == 0 {
		return nil, false
	}
	return profiles, true
}

// ModelsDirAbs — absoluto → tal cual; relativo → filepath.Join(Dir(), modelsDir).
func ModelsDirAbs(modelsDir string) string {
	if filepath.IsAbs(modelsDir) {
		return modelsDir
	}
	return filepath.Join(Dir(), modelsDir)
}

// ResolveModelPath — ruta de un fichero del modelo (model, mmproj, lora…): vacía, absoluta o URL
// (contiene "://") se devuelve tal cual; relativa → filepath.Join(modelsDirAbs, path). Evita que
// llama-server resuelva el nombre contra SU directorio de trabajo y no encuentre el fichero.
func ResolveModelPath(path, modelsDirAbs string) string {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "://") {
		return path
	}
	return filepath.Join(modelsDirAbs, path)
}

// Update — escanea ModelsDirAbs(cfg.ModelsDir); marca disabled:true los perfiles cuyo modelo ya no existe;
// añade solo .gguf no referenciados (un perfil por alias si hay JSON compañero en array; uno por modelo si es
// objeto; si no hay compañero → defaults de cfg.Defaults); guarda.
func Update(path string, cfg *Config) error {
	models, err := scanModels(ModelsDirAbs(cfg.ModelsDir))
	if err != nil {
		return err
	}
	ensureDefaults(cfg)      // red de seguridad: Update puede recibir un Config construido a mano
	markMissing(cfg, models) // modelos borrados/renombrados → ocultos del menú (disabled:true)
	referenced := make(map[string]bool, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		if f, ok := String(p, "file"); ok && f != "" {
			referenced[normalizeModelPath(f)] = true // tolera "/" vs "\" y caja distinta
		}
	}
	for _, m := range models { // models ya está ordenado alfabéticamente
		if referenced[normalizeModelPath(m)] {
			continue
		}
		if profiles, ok := profileFromCompanionJSON(ModelsDirAbs(cfg.ModelsDir), m); ok {
			cfg.Profiles = append(cfg.Profiles, profiles...) // uno o varios perfiles según el JSON compañero
			continue
		}
		if companionJSONPath(ModelsDirAbs(cfg.ModelsDir), m) != "" {
			fmt.Fprintf(os.Stderr, "warning: JSON compañero de %q inválido; usando parámetros por defecto\n", m)
		}
		cfg.Profiles = append(cfg.Profiles, defaultProfile(m, cfg.Defaults))
	}
	return Save(path, cfg)
}

// String — getter seguro a string.
func String(p Profile, key string) (string, bool) {
	v, ok := p[key].(string)
	return v, ok
}

// Bool — false si ausente/no bool.
func Bool(p Profile, key string) bool {
	v, _ := p[key].(bool)
	return v
}

// Float — getter a float64 (para mostrar port); acepta float64/int/int64/float32 y string numérico.
func Float(p Profile, key string) float64 {
	switch v := p[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

// Enabled — copia de las entradas con disabled != true, en orden original.
func Enabled(entries []Profile) []Profile {
	out := make([]Profile, 0, len(entries))
	for _, p := range entries {
		if Bool(p, "disabled") {
			continue
		}
		out = append(out, p)
	}
	return out
}
