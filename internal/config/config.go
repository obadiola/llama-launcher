// Package config gestiona config.json: perfiles de modelos, rutas de trabajo y
// escaneo de modelos .gguf. Sin dependencias externas (solo stdlib).
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// FileName — nombre del config.json; la ruta SIEMPRE se resuelve junto al ejecutable (Path()).
const FileName = "config.json"

// Profile — perfil de modelo (esquema abierto: cualquier clave extra se conserva).
type Profile = map[string]any

// Config — raíz de config.json v2: {"modelsDir","serverBin","profiles"}.
type Config struct {
	ModelsDir string    `json:"modelsDir"`
	ServerBin string    `json:"serverBin"`
	Profiles  []Profile `json:"profiles"`
}

// Default — configuración inicial (SIN escaneo): modelsDir "models", serverBin "llama-server" y profiles vacío.
func Default() *Config {
	return &Config{ModelsDir: "models", ServerBin: "llama-server", Profiles: make([]Profile, 0)}
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

// defaultProfile — alias/file/disabled/host/port/c/ngl/t/fa para un modelo nuevo.
func defaultProfile(modelFile string) Profile {
	base := filepath.Base(modelFile)
	alias := base
	if i := strings.LastIndex(alias, "."); i > 0 && strings.EqualFold(alias[i:], ".gguf") {
		alias = alias[:i]
	}
	return Profile{
		"alias":    alias,
		"file":     modelFile,
		"disabled": false,
		"host":     "127.0.0.1",
		"port":     8080,
		"c":        8192,
		"ngl":      99,
		"t":        8,
		"fa":       "on",
	}
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

// Update — escanea ModelsDirAbs(cfg.ModelsDir); añade solo .gguf no referenciados (un perfil por alias si
// hay JSON compañero en array; uno por modelo si es objeto); guarda.
func Update(path string, cfg *Config) error {
	models, err := scanModels(ModelsDirAbs(cfg.ModelsDir))
	if err != nil {
		return err
	}
	referenced := make(map[string]bool, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		if f, ok := String(p, "file"); ok {
			referenced[f] = true
		}
	}
	for _, m := range models { // models ya está ordenado alfabéticamente
		if referenced[m] {
			continue
		}
		if profiles, ok := profileFromCompanionJSON(ModelsDirAbs(cfg.ModelsDir), m); ok {
			cfg.Profiles = append(cfg.Profiles, profiles...) // uno o varios perfiles según el JSON compañero
			continue
		}
		if companionJSONPath(ModelsDirAbs(cfg.ModelsDir), m) != "" {
			fmt.Fprintf(os.Stderr, "warning: JSON compañero de %q inválido; usando parámetros por defecto\n", m)
		}
		cfg.Profiles = append(cfg.Profiles, defaultProfile(m))
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
