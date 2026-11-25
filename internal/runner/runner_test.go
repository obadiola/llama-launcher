package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"llama-launcher/internal/config"
)

// TestFlagPrefixShortFlags — alias con un solo guion documentados por llama-server → "-";
// claves largas → "--"; clave que ya empieza por "-" se deja tal cual; 1 letra → "-".
func TestFlagPrefixShortFlags(t *testing.T) {
	short := []string{
		"cl", "tb", "Cr", "Cb", "Crb", "ub", "fa", "kvo", "nkvo", "nr", "ctk", "ctv", "lm", "lzm",
		"dev", "ot", "cmoe", "ncmoe", "ncffn", "ngl", "sm", "ts", "mg", "fit", "fitt", "fitc",
		"mu", "dr", "hf", "hfr", "hff", "hft", "lv", "jf", "bs", "lcs", "lcd", "ctxcp", "cms",
		"cram", "kvu", "no-kvu", "sp", "np", "cb", "nocb", "mm", "mmu", "mmdev", "ag", "no-ag",
		"to", "rea", "sps", "ctkd", "ctvd", "hfd", "hfrd", "td", "tbd", "Cd", "Crd", "Cbd",
		"otd", "cmoed", "ncmoed", "devd", "ngld", "md",
		"c", "t", "b", "n", "C", "a", "s", "m", "l", "j", "r", "e", "v", "h",
	}
	for _, k := range short {
		if got := flagPrefix(k); got != "-" {
			t.Errorf("flagPrefix(%q) = %q; quiero \"-\"", k, got)
		}
	}

	long := []string{"port", "temp", "jinja", "chat-template-file", "api-key", "n-gpu-layers", "n-cpu-moe"}
	for _, k := range long {
		if got := flagPrefix(k); got != "--" {
			t.Errorf("flagPrefix(%q) = %q; quiero \"--\"", k, got)
		}
	}

	if got := flagPrefix("-ngl"); got != "" {
		t.Errorf("flagPrefix(\"-ngl\") = %q; quiero \"\"", got)
	}
}

// TestResolvePathValue — claves de ruta (mmproj/lora...): relativo → modelsDir; absoluto/URL intactos.
func TestResolvePathValue(t *testing.T) {
	modelsDir := t.TempDir()
	cases := []struct {
		name string
		key  string
		val  string
		want string
	}{
		{"mmproj relativo", "mmproj", "mmproj-vision.gguf", filepath.Join(modelsDir, "mmproj-vision.gguf")},
		{"mmproj subcarpeta", "mmproj", filepath.Join("vision", "mmproj.gguf"), filepath.Join(modelsDir, "vision", "mmproj.gguf")},
		{"mmproj absoluto", "mmproj", filepath.Join(modelsDir, "otro.gguf"), filepath.Join(modelsDir, "otro.gguf")},
		{"mmproj URL", "mmproj", "https://example.com/mmproj.gguf", "https://example.com/mmproj.gguf"},
		{"clave con guiones", "--mmproj", "mmproj-x.gguf", filepath.Join(modelsDir, "mmproj-x.gguf")},
		{"lora relativo", "lora", "adapter.gguf", filepath.Join(modelsDir, "adapter.gguf")},
		{"clave no-ruta", "temp", "0.6", "0.6"},
	}
	for _, c := range cases {
		if got := resolvePathValue(c.key, c.val, modelsDir); got != c.want {
			t.Errorf("%s: resolvePathValue(%q, %q) = %q; quiero %q", c.name, c.key, c.val, got, c.want)
		}
	}
}

// TestBuildArgsResuelveRutas — BuildArgs envía mmproj (y rutas en arrays) completos bajo modelsDir.
func TestBuildArgsResuelveRutas(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "llama-server")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if err := os.WriteFile(bin, []byte{}, 0o755); err != nil {
		t.Fatalf("crear servidor falso: %v", err)
	}
	modelsDir := filepath.Join(dir, "models")
	cfg := &config.Config{ModelsDir: modelsDir, ServerBin: bin}
	p := config.Profile{
		"alias":  "vision",
		"file":   "model.gguf",
		"mmproj": "mmproj-model.gguf",
		"lora":   []any{"adapter.gguf", "https://example.com/remote.gguf"},
	}

	args, err := BuildArgs(p, cfg)
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}
	want := []string{"--lora", filepath.Join(modelsDir, "adapter.gguf"), "--lora", "https://example.com/remote.gguf",
		"--mmproj", filepath.Join(modelsDir, "mmproj-model.gguf")}
	joined := make(map[string]bool, len(args))
	for i := 1; i < len(args); i++ {
		joined[args[i-1]+"="+args[i]] = true
	}
	for i := 0; i < len(want); i += 2 {
		if !joined[want[i]+"="+want[i+1]] {
			t.Errorf("falta %q %q en %q", want[i], want[i+1], args)
		}
	}
}

// TestBuildArgsUsaAliasCortos — claves con alias corto se emiten con "-" (valor o solas si bool);
// las claves sin alias siguen saliendo con "--".
func TestBuildArgsUsaAliasCortos(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "llama-server")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if err := os.WriteFile(bin, []byte{}, 0o755); err != nil {
		t.Fatalf("crear servidor falso: %v", err)
	}
	cfg := &config.Config{ModelsDir: dir, ServerBin: bin}
	p := config.Profile{
		"alias":  "router",
		"file":   "model.gguf",
		"ctk":    "q8_0",
		"sm":     "row",
		"ts":     "0.5,0.5",
		"np":     2,
		"no-kvu": true,
		"port":   8080,
	}

	args, err := BuildArgs(p, cfg)
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	// flag → siguiente argumento (los bool true van solos y no entran aquí).
	got := make(map[string]string, len(args))
	for i := 0; i+1 < len(args); i++ {
		if len(args[i]) > 1 && args[i][0] == '-' {
			got[args[i]] = args[i+1]
		}
	}
	want := map[string]string{
		"-ctk":   "q8_0",
		"-sm":    "row",
		"-ts":    "0.5,0.5",
		"-np":    "2",
		"--port": "8080",
	}
	for flag, val := range want {
		if got[flag] != val {
			t.Errorf("%s = %q; quiero %q (args: %q)", flag, got[flag], val, args)
		}
	}

	seen := func(flag string) bool {
		for _, a := range args {
			if a == flag {
				return true
			}
		}
		return false
	}
	if !seen("-no-kvu") {
		t.Errorf("falta -no-kvu en %q", args)
	}
	if seen("--no-kvu") || seen("--ctk") || seen("--ts") {
		t.Errorf("se emitió la forma larga de un alias corto: %q", args)
	}
}
