# CEREBRO.md — llama-launcher

> Memoria externa (protocolo de memoria incremental) · Go stdlib-only · Windows/Linux.
> App de consola: gestiona y lanza modelos GGUF vía `llama-server` (llama.cpp).
> Go de la máquina: go1.27.4 (directiva go.mod: `go 1.24`).

## 1. Estado Actual y Tareas

- [x] Análisis de arquitectura y diseño completo (plan aprobado).
- [x] Esquema abierto `map[string]any` + reglas de traducción dinámica de flags.
- [x] Crear `go.mod`: `module llama-launcher` (go 1.24, cero dependencias).
- [x] Implementar `config.go`: `Profile`, `loadConfig`, `saveConfig`, `scanModels`, `defaultProfile`, `updateConfig`, `profileString/Bool/Float`. (v2: `Config`, `configDir`, `configPath`, `modelsDirAbs`; gofmt/vet/build exit 0)
- [x] Implementar `runner.go`: `shortFlags`, `resolveServerBin`, `flagPrefix`, `formatJSONValue`, `buildArgs`, `runServer`. (gofmt limpio; go vet exit 0)
- [x] Implementar `main.go`: constantes, `main`, `menuLoop`, `enabledProfiles`, `printMenu`, `readLine`, `pause`. (gofmt/vet/build exit 0; smoke test: config auto-creada + menú + exit 0)
- [x] Mover `ModelsDir`/`ServerBin` a `config.json` (esquema objeto v2) + `configPath()` junto al ejecutable. (config.go v2: `Config`, `configDir`, `configPath`, defaults; build exit 0)
- [x] Excluir `mmproj-*.gguf` del escaneo (son `--mmproj`, no modelos IA). (scanModels salta base `mmproj-` case-insensitive)
- [x] Arranque sin escaneo: si config.json no existe → crear con defaults + `profiles` vacíos (SIN `scanModels`); el escaneo pasa SOLO a `[U]`. (main.go: `cfg=&Config{..., Profiles:make([]Profile,0)}`; smoke S1: exit 0, config `profiles:[]`, sin escaneo)
- [x] Menú accesible con cursor: selección `↑`/`↓` + ENTER vía `readKey`; números directos y `u/U`/`0` conservados. (main.go: `readKey`, `menuLoop` con `sel`, `printMenu(entries,sel)`, `runProfile`; smoke S2: `>` se movió model-a→model-b tras `↓`)
- [x] FIX: rutas Windows con backslash en config.json rompen el parseo (invalid escape sequence). Sanear en loadConfig (sanitizeJSONBackslashes). (config.go: sanitizeJSONBackslashes + reintento; smoke: ruta C:/AI/llama.cpp/models parseada OK, u escaneo 10 modelos reales, mmproj excluido, reescrito JSON valido)
- [x] Guardar config.json corregido: si `loadConfig` saneó backslashes, reescribir el archivo en disco con los bytes saneados (persistir la corrección). (config.go: loadConfig `os.WriteFile(path, sanitized)` tras reintento OK; warning si no se pudo guardar; smoke S1: ruta Windows con backslash simple corregida en disco + 2ª carga idempotente sin reescritura)
- [x] Perfiles desde JSON compañero: `<modelo>.json` junto al `.gguf` → añadirlo a config con SUS parámetros (no defaults) si el modelo no está referenciado; `file` sobreescrito por la ruta relativa; **inválido → warning + defaultProfile; ausente → defaultProfile silencioso** (solo en `[U]`). (config.go: `companionJSONPath` + `profileFromCompanionJSON`; smoke S2: model-a con alias custom/port 9999/c 4096, model-b defaults, model-c warning+defaults)
- [x] Configurar `.vscode/launch.json` + `tasks.json` para depurar con F5 (build pre-launch + terminal integrada). (.vscode: tasks.json tarea `build` shell go build; launch.json mode executable + integratedTerminal + preLaunchTask build; JSON validado + build exit 0)
- [x] FIX: `formatJSONValue` no soportaba int/bool → warnings + flags omitidos al lanzar (perfiles en memoria de `defaultProfile` guardan `int`/`bool`). Extender a todos los tipos JSON escalares. (runner.go: float32 + int..uint64 + bool→"1"/"0"; smoke: perfil creado por [U] + lanzamiento → comando con `-c 8192 -fa 1 -ngl 99 --port 8080 -t 8`, sin warnings, exit 0)
- [x] Quitar cursor de teclas del menú (pedido usuario 2026-11-17: `↑`/`↓` no funcionan en su terminal y ENTER lanzaba desde la posición del cursor). Revertir a por-línea: número → lanza directo, `u/U`, `0`, ENTER/vacío → repintar; eliminar `readKey`/`sel`. (main.go: `menuLoop` por `readLine`, `printMenu(entries)` sin cursor, `readKey` eliminado; smoke: `u`→añade, ENTER vacío NO lanza, `1`→lanza con flags completos, `0`→exit 0)
- [x] FIX: bool `true` se emitía como `--jinja 1`; llama.cpp exige flag SIN valor (`--jinja`). Cambiar en `buildArgs`: bool true → solo la clave. (runner.go: `case bool` en buildArgs → solo `flagPrefix(k)+k`; smoke: perfil con jinja:true+fa:true → `-fa --jinja` sin valores, exit 0)
- [x] Build + prueba de flujo: arranque, `[U]` (escaneo + JSON compañero) y lanzamiento con servidor fake (exit 0).
- [x] FIX menú `port: 0`: perfiles con `"port": "8080"` (string) no se mostraban; `profileFloat` ahora parsea strings numéricos (`strconv` añadido a config.go). Smoke: los 10 perfiles reales muestran `port: 8080`.
- [x] FIX BOM UTF-8: `config.json` guardado con BOM (Notepad/`Out-File`) fallaba el parseo, y un `0` pipeado con BOM daba "Opción no válida". `loadConfig` recorta `utf8BOM` antes de unmarshal; `readLine` recorta `\uFEFF` de la línea. Smoke: config con BOM carga el perfil; stdin con BOM → "Adiós." exit 0.
- [x] Menú con pantalla limpia + cabecera (pedido usuario 2026-11-17): `menuLoop` hace `clearScreen()` + `printHeader()` en cada repintado; cabecera `llama-launcher v1.0.0` + línea `=`. `ui.go` (`AppName`/`AppVersion`/`isTerminal`/`printHeader`), `term_windows.go` (ANSI VT vía `SetConsoleMode` con `syscall.NewLazyDLL` + fallback `cmd /c cls`), `term_unix.go` (ANSI). Sin escapes si stdout no es consola (pipes/tests limpios). Smoke: pipe → cabecera sin ANSI; consola → limpia y menú arriba.
- [x] FIX UX derivado de la limpieza: "Opción no válida." se borraba en el repintado inmediato → ahora `pause("Pulsa ENTER para continuar...")` antes de volver al menú.
- [x] Rellenar `readme.md` con el funcionamiento completo (uso, menú, config.json, traducción de flags, JSON compañero, escaneo, detalles Windows, estructura y limitaciones). Validado: los 4 ejemplos JSON parsean; el comando de ejemplo coincide con la salida real (`-m ... --alias ... -c 81920 -fa on --jinja --port 8080`).
- [x] REORG (2026-11-18, pedido usuario): estructura Go estándar. `cmd/llama-launcher/main.go` (package main) + `internal/config` (config.json) + `internal/runner` (flags/ejecución) + `internal/ui` (cabecera, consola, limpieza de pantalla). Exportar solo la API usada entre paquetes; eliminar los .go de la raíz; actualizar `.vscode/tasks.json` (build `./cmd/llama-launcher`), `.vscode/launch.json` (program `cmd/llama-launcher`) y README (estructura y comandos). Validación: gofmt limpio, `go vet ./...` exit 0, `go build -o llama-launcher.exe ./cmd/llama-launcher` exit 0; smoke en dir temporal: S1 arranque sin config → `profiles: []` + `0` exit 0; S2 `u` → escaneo con JSON compañero (`custom-alpha` port 9999/c 4096, `beta` defaults, `mmproj-*` excluido); S3 lanzamiento con server fake → `-m ... --alias custom-alpha -c 4096 --port 9999`, vuelta al menú y exit 0.
- [x] NUEVO `[R] Refrescar config` (pedido usuario 2026-11-18): `config.Load(path)` de nuevo (SIN escaneo) y sustituye el contenido en memoria (`*cfg = *reloaded`); error o archivo ausente → aviso + ENTER conservando la config actual. (main.go: `refreshConfig(path, cfg)`; smoke S4: perfil deshabilitado en disco + `r` → menú pasa de 2 a 1 perfiles; S6: JSON inválido → mensaje + pause + exit 0)
- [x] NUEVO `defaults` en config.json (v3, pedido usuario): `Config.Defaults Profile` con tag `json:"defaults"`; `builtinDefaults()` = `host/port/c/ngl/t/fa` + `api-key` = UUID v4 (`newUUID`, crypto/rand); `ensureDefaults(cfg) bool` crea el objeto y completa claves que falten → `Load` guarda si hubo cambios; `defaultProfile(modelFile, defaults)` = copia de defaults + alias/file/disabled:false. (smoke S1: config nueva → `DEFAULT_KEYS=api-key,c,fa,host,ngl,port,t` + UUID4 OK; S2: perfil nuevo hereda defaults y el UUID no se regenera)
- [x] NUEVO `[U]` marca `disabled:true` en perfiles cuyo `file` ya NO está en el escaneo (`markMissing`; comparación `filepath.ToSlash` + minúsculas); solo activa, nunca reactiva (respeta deshabilitados manuales); `Update` guarda. También `referenced` usa la misma normalización (evita duplicados `sub/x` vs `sub\x`). (smoke S3/S5: modelo borrado → `disabled=True` y ausente del menú; el resto intacto)
- [x] NUEVO menú: fichero del modelo en cian (`ui.ColorCyan`) y `(host:puerto)` en amarillo (`ui.ColorYellow`, host por defecto `127.0.0.1`); `ui.Color(s, code)` + `colorEnabled()` por plataforma (Windows: VT una sola vez con `sync.Once`; sin consola/VT → sin escapes). Pie con `[U]`, `[R]`, `[0]`. (smoke S4: `(127.0.0.1:8080)` en el menú, 0 escapes ANSI con stdout pipeado)
- [x] NUEVO prompt antes de leer la opción (pedido usuario 2026-11-18): `menuLoop` imprime `Introduce un código: ` (sin salto de línea) justo antes de `ui.ReadLine`; se borra con el repintado (ClearScreen) tras cualquier acción. (main.go: `fmt.Print("Introduce un código: ")`; smoke: prompt tras `[0] Salir` antes de leer la línea, UTF-8 correcto)
- [x] NUEVO pie del menú (pedido usuario 2026-11-18): `[R] Refrescar memoria con datos de config` (antes `[R] Refrescar config`); comportamiento sin cambios. (main.go ya lo tenía editado a mano por el usuario; smoke: etiqueta exacta en la salida + `[U]`/prompt intactos; README + CEREBRO sincronizados)
- [x] NUEVO `[S]` modo servidor router para la Pi (pedido usuario 2026-11-21): genera/sustituye `<modelsDir>/preset.ini` desde los perfiles habilitados y arranca `<serverBin> --models-preset <preset.ini> --port 8080 --host 0.0.0.0` con cwd = `config.Dir()`. Diseño: paquete nuevo `internal/preset` (Build/Save/Path), `runner.ResolveServerBin` exportado y `runner.RunServerIn(dir, args)`; main: `startServerMode` + `runArgs`. (smoke 2026-11-21: gofmt limpio + `go vet ./...` + build exit 0; `config.json` real en dir temporal → 18 secciones, 0 claves prohibidas y 0 duplicadas, `gemma-4-E4B-it` colisión `n-gpu-layers`+`ngl` → 99, `Qwen3.8` `n-gpu-layers = all` conservado; `--models-preset <abs> --port 8080 --host 0.0.0.0` recibidos por server fake; preset previo SUSTITUIDO; sin perfiles → aviso + ENTER y NO crea preset)
- [x] FIX lanzamiento directo `[n]`: la clave `mmproj` se enviaba a llama-server SIN ruta (nombre relativo al CWD del launcher) → no encontraba el proyector; solo el preset `[S]` la resolvía. Unificar: `config.ResolveModelPath` compartido + resolución de claves de RUTA en `runner.BuildArgs` (string y elementos string de arrays). (2026-11-24: gofmt limpio, `go vet ./...` + `go test ./...` + build exit 0; test `TestResolvePathValue`/`TestBuildArgsResuelveRutas`; smoke con server fake: `--mmproj C:\...\models\mmproj-model.gguf` al lanzar `[1]` y preset `[S]` con la misma ruta; README actualizado)
- [x] FIX preset.ini (pedido usuario 2026-11-21): la clave `mmproj` debe llevar la RUTA COMPLETA del fichero (relativa a `modelsDir` → absoluta, igual que `model`), no solo el nombre. Diseño: `resolveModelPath(path, modelsDirAbs)` + `options(p, modelsDirAbs)`; absolutos/URLs tal cual (`ToSlash`). (smoke 2026-11-21: gofmt/vet/build exit 0; relativo `mmproj-vision.gguf` → `<modelsDir>/mmproj-vision.gguf`, subcarpeta `vision/mmproj-other.gguf` → `<modelsDir>/vision/...`, absoluto `C:/otro/sitio/...` intacto, `mmproj-url` intacto, clave `--mmproj` normalizada a `mmproj` con ruta completa)
- [x] `shortFlags` COMPLETO desde la doc oficial (pedido usuario 2026-11-25): añadidos todos los alias con UN solo guion de `tools/server/README.md` de llama.cpp (68 claves de ≥2 letras: `cl`, `tb`, `Cr`, `Cb`, `Crb`, `ub`, `kvo`, `nkvo`, `nr`, `ctk`, `ctv`, `lm`, `lzm`, `dev`, `ot`, `cmoe`, `ncmoe`, `ncffn`, `sm`, `ts`, `mg`, `fit`, `fitt`, `fitc`, `mu`, `dr`, `hf`, `hfr`, `hff`, `hft`, `lv`, `jf`, `bs`, `lcs`, `lcd`, `ctxcp`, `cms`, `cram`, `kvu`, `no-kvu`, `sp`, `np`, `cb`, `nocb`, `mm`, `mmu`, `mmdev`, `ag`, `no-ag`, `to`, `rea`, `sps`, `ctkd`, `ctvd`, `hfd`, `hfrd`, `td`, `tbd`, `Cd`, `Crd`, `Cbd`, `otd`, `cmoed`, `ncmoed`, `devd`, `ngld`, `md`; `ngl`/`fa` ya estaban). Las de 1 letra (`h t c n b e v s m l j r a C`) siguen emitiéndose con `-` por longitud. Test `TestFlagPrefixShortFlags` + `TestBuildArgsUsaAliasCortos`; README con la tabla de traducción actualizada. (gofmt limpio, `go vet ./...` + `go test ./...` + build exit 0; cobertura verificada por script: 68 alias en el README oficial = 68 en el mapa, 0 diferencias; smoke con server falso: `-c 2048 -ctk q8_0 -no-kvu -np 2 --port 8080 -sm row -ts 0.5,0.5`, exit 0)






## 2. Arquitectura y Contratos (Firmas aprobadas)

> Nota (2026-11-18): las secciones siguientes describen los archivos de la v1 (todos en la raíz). Tras la REORG el layout vigente es *Estructura de paquetes (v2)*, al final de esta sección, con las firmas definitivas.

### go.mod
- `module llama-launcher` · `go 1.24` · sin require (solo stdlib).

### Constantes (definidas en main.go)
- `const ConfigFile = "config.json"` — NOMBRE del archivo; la ruta se resuelve junto al ejecutable vía `configPath()` (config.go).
- Nota: los valores reales de la máquina del usuario son `modelsDir: "C:/AI/llama.cpp/models"` y `serverBin: "C:/AI/llama.cpp/motor"` → van en el config.json del usuario, no en código.

### config.go (v2 — config.json como objeto)
- `type Profile = map[string]any` — perfil (esquema abierto: cualquier clave extra se conserva al leer/guardar).
- `type Config struct { ModelsDir string; ServerBin string; Profiles []Profile }` con tags json `modelsDir`/`serverBin`/`profiles` — raíz de config.json v2.
- `func configDir() string` — `filepath.Dir(os.Executable())`; si falla → `"."`.
- `func configPath() string` — `filepath.Join(configDir(), ConfigFile)`; siempre junto al ejecutable.
- `func sanitizeJSONBackslashes(data []byte) []byte` — convierte `\X` dentro de strings (X no escape JSON válido) en `\\X` (rutas Windows `C:\AI` → `C:\\AI`); deja intactas las escapes válidas (`\"`, `\\`, `\/`, `\bfnrt`, `\uXXXX`); idempotente (un JSON válido no se modifica).
- `var utf8BOM = []byte{0xEF, 0xBB, 0xBF}` — BOM UTF-8 que `loadConfig` recorta con `bytes.TrimPrefix` antes de parsear (tolerancia a Notepad/`Out-File`).
- `func loadConfig(path string) (*Config, error)` — si no existe → `nil, nil`; parsea objeto a `*Config`; si el unmarshal falla → sanitizar backslashes y reintentar (rutas Windows malformadas); **si se usó la versión saneada → reescribir el archivo en disco con los bytes saneados** (config corregido persistido; un JSON válido nunca se toca); aplica defaults `modelsDir:"models"`, `serverBin:"llama-server"` si vacíos; error wrappeado si sigue inválido.
- `func saveConfig(path string, cfg *Config) error` — `json.MarshalIndent` (2 espacios) + salto de línea final.
- `func scanModels(dir string) ([]string, error)` — `filepath.WalkDir` recursivo en `dir`; rutas relativas `.gguf` (insensible), ordenadas; salta base que empieza por `mmproj-` (case-insensitive): son el parámetro `--mmproj` de un modelo superior, no modelos IA.
- `func defaultProfile(modelFile string) Profile` — `alias` = nombre base sin `.gguf`; `file`, `disabled:false`, `host:"127.0.0.1"`, `port:8080`, `c:8192`, `ngl:99`, `t:8`, `fa:"on"` (llama.cpp nuevo: `-fa on|off|auto`; con bool `true` se emitiría `-fa` sin valor).
- `func companionJSONPath(modelsDirAbs, modelFile string) string` — resuelve la ruta del JSON compañero (exacto o case-insensitive en el mismo dir); `""` si no existe. (helper de `profileFromCompanionJSON` y del warning en `updateConfig`)
- `func profileFromCompanionJSON(modelsDirAbs, modelFile string) ([]Profile, bool)` — lee el JSON compañero vía `companionJSONPath`; admite un ARRAY de configuraciones (una por alias) **o un objeto único** (compatibilidad: se envuelve en un slice); **siempre sobreescribe `file` por `modelFile` (ruta relativa)**; omite elementos nulos; `ok=false` si no existe, es inválido o queda vacío.
- `func updateConfig(path string, cfg *Config) error` — escanea `modelsDirAbs(cfg.ModelsDir)`; añade solo los `.gguf` cuyo nombre no aparece en el campo `file` de ninguna entrada (ni en `disabled:true`); **para cada nuevo modelo: si existe JSON compañero válido → `profileFromCompanionJSON` (append de TODOS los perfiles del array); si no o inválido (warning) → `defaultProfile`**; nunca modifica entradas existentes; guarda.
- `func modelsDirAbs(modelsDir string) string` — absoluto → tal cual; relativo → `filepath.Join(configDir(), modelsDir)`.
- `func profileString(p Profile, key string) (string, bool)` — getter seguro a string.
- `func profileBool(p Profile, key string) bool` — `false` si ausente/no bool (ausente ≠ deshabilitado).
- `func profileFloat(p Profile, key string) float64` — getter a float64 (para mostrar `port`); acepta `float64/int/int64/float32` y `string` numérico (`strconv.ParseFloat`; si no parsea → 0). Fix 2026-11-17: perfiles con `"port": "8080"` (string) mostraban `port: 0`.
### runner.go (implementado — firmas definitivas)
- `var shortFlags = map[string]bool{...}` — alias CORTO con un solo guion documentado en `tools/server/README.md` de llama.cpp (consulta 2026-11-25): `cl`, `tb`, `Cr`/`Cb`/`Crb`, `ub`, `fa`, `kvo`/`nkvo`/`nr`, `ctk`/`ctv`, `lm`/`lzm`, `dev`, `ot`, `cmoe`/`ncmoe`/`ncffn`, `ngl`, `sm`/`ts`/`mg`, `fit`/`fitt`/`fitc`, `mu`/`dr`/`hf`/`hfr`/`hff`/`hft`, `lv`, `ctkd`/`ctvd`, `jf`, `bs`, `lcs`/`lcd`, `ctxcp`/`cms`/`cram`/`kvu`/`no-kvu`, `sp`, `np`, `cb`/`nocb`, `mm`/`mmu`/`mmdev`, `ag`/`no-ag`, `to`, `rea`, `sps`, `hfd`/`hfrd`, `td`/`tbd`, `Cd`/`Crd`/`Cbd`, `otd`, `cmoed`/`ncmoed`, `devd`, `ngld`, `md` → prefijo `-`; 1 letra también `-`; 2+ letras sin alias `--`; clave ya con `-` se usa tal cual. Los alias son sensibles a mayúsculas (`Cr` ≠ `cr`). Test: `TestFlagPrefixShortFlags`.
- `func resolveServerBin(bin string) (string, error)` — resuelve `bin` (absoluto tal cual; relativo → `configDir()`); `exec.LookPath`; fallback Windows `llama-server.exe`.
- `func flagPrefix(key string) string` — prefijo de flag según longitud de clave / lista `shortFlags`.
- `func formatJSONValue(v any) (string, bool)` — string → literal; float64 entero exacto → decimal (`FormatInt`); otro float64/float32 → `FormatFloat`; int/int8/int16/int32/int64/uint/uint8/uint16/uint32/uint64 → decimal; bool → `"1"`/`"0"`; `ok=false` solo si el tipo no soportado (mapa anidado u otro).
- `func buildArgs(p Profile, cfg *Config) ([]string, error)` — `[bin, -m, filepath.Join(modelsDirAbs(cfg.ModelsDir),file), --alias, alias]` + claves restantes ordenadas alfabéticamente: omite `alias|file|disabled` y bools `false`; **bools `true` → SOLO la clave (flag sin valor: llama.cpp `--jinja`, no `--jinja 1`)**; arrays (`extra_args`) se expanden elemento a elemento (elemento no soportado → warning + omisión del elemento); mapas anidados → warning + omisión; `file`/`alias` ausente o no string → error.
- `func runServer(args []string) (exitCode int, interrupted bool, err error)` — `exec.CommandContext` + `signal.Notify(os.Interrupt, syscall.SIGTERM)`; `Stdout`/`Stderr` heredados del hijo (fallback `io.Discard` si son nil); señal → `cancel()`; bloquea en `cmd.Wait()`; salida por señal ⇒ `interrupted=true`.

### ui.go / term_windows.go / term_unix.go (UI de consola)
- `const AppName = "llama-launcher"` — nombre mostrado en la cabecera.
- `const AppVersion = "1.0.0"` — versión mostrada en la cabecera (SemVer).
- `func isTerminal(f *os.File) bool` — true si `f` es consola (`info.Mode()&os.ModeCharDevice != 0`); false en pipe/redirección.
- `func printHeader()` — `llama-launcher v1.0.0` + línea `=` (len(AppName)+len(AppVersion)+2) + línea en blanco.
- `func clearScreen()` — LIMPIA la pantalla: Windows (`term_windows.go`) activa `ENABLE_VIRTUAL_TERMINAL_PROCESSING` (0x0004) con `kernel32!SetConsoleMode` vía `syscall.NewLazyDLL` y emite `ESC[2J ESC[H`; si no hay VT → fallback `cmd /c cls`. Unix (`term_unix.go`, `//go:build !windows`) → ANSI. **No escribe nada si `!isTerminal(os.Stdout)`** (tests/pipes limpios).

### readme.md (documentación de usuario)
- Secciones: descripción/requisitos, compilar y ejecutar, flujo interno, menú, `config.json` (campos + traducción de claves a flags), escaneo `[U]` + JSON compañero + perfil por defecto, detalles Windows (backslash/BOM/sin shell/Ctrl+C), estructura del proyecto, limitaciones y problemas típicos.
- `AppVersion` (en `ui.go`) es el único sitio a tocar al publicar una versión; la cabecera del readme debe coincidir.

### main.go
- `const ConfigFile = "config.json"` — NOMBRE del config; la ruta siempre se resuelve junto al ejecutable vía `configPath()` (config.go).
- `var stdinReader = bufio.NewReader(os.Stdin)` — lector único global para `readLine` (pausas y menú); reemplaza al antiguo `stdinScanner`.
- `func main()` — `path := configPath()`; `cfg, err := loadConfig(path)`: error → mensaje + exit 1; si `nil` (no existe) → `cfg = &Config{}` con defaults (`modelsDir:"models"`, `serverBin:"llama-server"`) + `profiles` VACÍOS (NO escaneo al arrancar) + `saveConfig(path, cfg)`; `menuLoop(path, cfg)`; exit 0.
- `func menuLoop(path string, cfg *Config)` — bucle por LÍNEAS: `clearScreen()` + `printHeader()` + `printMenu(enabledProfiles(cfg.Profiles))` + leer línea; manejo (pedido usuario 2026-11-17: SIN cursor de teclas, ENTER NO lanza):
  - Número 1..N (multi-dígito OK): lanza el ítem n directamente vía `runProfile`.
  - `"u"`/`"U"`: `updateConfig(path, cfg)` — EL ÚNICO LUGAR donde corre `scanModels` (error → warning + pause).
  - `"0"`: salir (return).
  - Línea vacía / ENTER: repintar (NO lanza nada).
  - Otra entrada: "Opción no válida." + repintar.
- `func runProfile(path string, cfg *Config, p Profile)` — lanza un perfil: `stat` de `filepath.Join(modelsDirAbs(cfg.ModelsDir), file)` (ausente → error + `pause` sin lanzar) → `buildArgs(p, cfg)` → mostrar comando → `runServer` → `pause` si `exitCode != 0 && !interrupted`.
- `func enabledProfiles(entries []Profile) []Profile` — copia de entradas con `profileBool(p,"disabled") != true`, en orden original.
- `func printMenu(entries []Profile)` — lista numerada de habilitados: `N) alias — file (port: X)`; pies `[U] Actualizar config · [0] Salir · número para lanzar`; nota si no hay habilitados.
- `func readLine() (string, bool)` — `stdinReader.ReadString('\n')` (absorbe \r\n); ok=false en EOF.
- `func pause(prompt string)` — imprime `prompt` y espera ENTER (`readLine`); no bloquea si `os.Stdin == nil`.

### .vscode (depuración)
- `.vscode/tasks.json` — tarea shell `build`: `go build -o llama-launcher.exe .` (group: build, reveal silent, problemMatcher `$goCompile`).
- `.vscode/launch.json` — config `"(debug) llama-launcher"`: type go, request launch, mode executable, program `${workspaceFolder}/llama-launcher.exe`, cwd workspaceFolder, console integratedTerminal (stdin del menú va por esa terminal), preLaunchTask build.

### Estructura de paquetes (v2, 2026-11-18)

Layout estándar de un programa Go (binario en `cmd/`, lógica no reutilizable en `internal/`):

```
cmd/llama-launcher/main.go      package main
internal/config/config.go       package config
internal/preset/preset.go       package preset
internal/runner/runner.go       package runner
internal/ui/ui.go               package ui
internal/ui/term_windows.go     package ui (//go:build windows)
internal/ui/term_unix.go        package ui (//go:build !windows)
go.mod · config.json · README.md · CEREBRO.md · .vscode/
```

Import paths con el module actual: `llama-launcher/internal/config`, `llama-launcher/internal/runner`, `llama-launcher/internal/ui`.

### internal/config — API exportada
- `const FileName = "config.json"` — NOMBRE del archivo (sustituye a `ConfigFile` de main).
- `type Profile = map[string]any` — perfil de esquema abierto (alias del map, sin cambio semántico).
- `type Config struct { ModelsDir, ServerBin string; Profiles []Profile }` — raíz de config.json v2.
- `func Default() *Config` — defaults sin escaneo: `models`, `llama-server`, `profiles` vacío (usado por main al no existir config.json).
- `func Dir() string` — directorio del ejecutable (`os.Executable`), `"."` si falla.
- `func Path() string` — `filepath.Join(Dir(), FileName)`; única forma de resolver la ruta del config.
- `func Load(path string) (*Config, error)` — nil,nil si no existe; tolera BOM y sanea backslashes persistiendo la corrección.
- `func Save(path string, cfg *Config) error` — MarshalIndent 2 espacios + `\n`.
- `func ModelsDirAbs(modelsDir string) string` — absoluto tal cual; relativo → `filepath.Join(Dir(), modelsDir)`.
- `func ResolveModelPath(path, modelsDirAbs string) string` — ruta de fichero del modelo (`model`, `mmproj`, `lora`…): vacía/absoluta/URL (`://`) tal cual; relativa → `filepath.Join(modelsDirAbs, path)`. Compartida por runner (flags) y preset (INI).
- `func Update(path string, cfg *Config) error` — escaneo + JSON compañero + save (único escaneo).
- `func String(p Profile, key string) (string, bool)` / `func Bool(p Profile, key string) bool` / `func Float(p Profile, key string) float64` — getters tolerantes (string numérico incluido).
- `func Enabled(entries []Profile) []Profile` — filtra `disabled: true` conservando orden (antes `enabledProfiles` en main).
- Internas: `sanitizeJSONBackslashes`, `scanModels`, `defaultProfile`, `companionJSONPath`, `profileFromCompanionJSON`, `utf8BOM`.

### internal/config — v3 ampliación (2026-11-18, diseño)
- `type Config struct { ModelsDir, ServerBin string; Defaults Profile; Profiles []Profile }` — `defaults` se serializa ANTES de `profiles` (orden de campos del struct).
- `func Default() *Config` — añade `Defaults: builtinDefaults()`.
- `func newUUID() string` — UUID v4 (`crypto/rand` + version/variant); fallback al reloj si `rand` falla.
- `func builtinDefaults() Profile` — `host:"127.0.0.1"`, `port:8080`, `c:8192`, `ngl:99`, `t:8`, `fa:"on"`, `api-key:<UUID v4>` (flag `--api-key`).
- `func ensureDefaults(cfg *Config) bool` — crea `Defaults` si nil y añade cada clave ausente de `builtinDefaults()`; true = hubo cambios (guardar).
- `func defaultProfile(modelFile string, defaults Profile) Profile` — copia de `defaults` + `alias` (base sin `.gguf`) + `file` + `disabled:false`.
- `func markMissing(cfg *Config, models []string) bool` — `disabled:true` en perfiles con `file` no presente en el escaneo (no toca los ya `true`); true = hubo cambios.
- `func normalizeModelPath(rel string) string` — clave de comparación `strings.ToLower(filepath.ToSlash(rel))`.
- `func Update(path string, cfg *Config) error` — escaneo → `ensureDefaults` (red de seguridad) → `markMissing` → altas (compañero/`defaultProfile(m, cfg.Defaults)`) → `Save`.
- `func Load(path string) (*Config, error)` — además: `ensureDefaults` y, si devuelve true, `Save` (warning por stderr si no se pudo guardar).

### internal/runner — API exportada
- `func BuildArgs(p config.Profile, cfg *config.Config) ([]string, error)` — binario + `-m` + `--alias` + flags ordenados (bool true sin valor, false omitido).
- `func RunServer(args []string) (exitCode int, interrupted bool, err error)` — primer plano, Ctrl+C reenviado al hijo (delega en `RunServerIn("", args)`).
- `func RunServerIn(dir string, args []string) (exitCode int, interrupted bool, err error)` — como `RunServer` con `cmd.Dir = dir` ("" = actual); usado por el modo servidor con cwd = `config.Dir()`. Error si `args` está vacío.
- `func ResolveServerBin(bin string) (string, error)` — antes `resolveServerBin`: absoluto tal cual, relativo → `config.Dir()`, `exec.LookPath`, fallback Windows `llama-server.exe`; lo usa también el modo servidor.
- `var pathFlags = map[string]bool{...}` — claves cuyo valor es una RUTA de fichero del modelo: `mmproj`, `lora`, `control-vector`, `grammar-file`, `chat-template-file`, `model-draft`.
- `func resolvePathValue(key, val, modelsDirAbs string) string` — si `key` (sin guiones iniciales) está en `pathFlags` → `config.ResolveModelPath(val, modelsDirAbs)`; si no, `val` intacto.
- `BuildArgs` aplica `resolvePathValue` a los valores string y a cada elemento string de arrays: `mmproj` relativo se envía como ruta completa bajo `modelsDir` (igual que `[S]`), para que llama-server lo encuentre sea cual sea su CWD.
- Internas: `shortFlags`, `flagPrefix`, `formatJSONValue`, `pathFlags`, `resolvePathValue`.
- Test: `internal/runner/runner_test.go` — `TestResolvePathValue` (relativo/absoluto/URL/subcarpeta/clave con guiones/clave no-ruta) y `TestBuildArgsResuelveRutas` (mmproj y arrays con rutas).

### internal/preset — API exportada (modo servidor, 2026-11-21)
- `const FileName = "preset.ini"` — nombre del fichero de presets (dentro de `modelsDir`).
- `func Path(modelsDirAbs string) string` — `filepath.Join(modelsDirAbs, FileName)`.
- `func Build(cfg *config.Config) string` — INI: comentario de cabecera + una sección por perfil HABILITADO (`[alias]`; duplicados → `-2`, `-3`…), `model = <ruta absoluta del .gguf>` + el resto de claves traducidas a forma larga y ordenadas; `mmproj` también con ruta absoluta.
- `func Save(path string, cfg *config.Config) error` — `os.WriteFile(path, []byte(Build(cfg)), 0o644)` (sustituye/trunca el fichero).
- Internas: `sectionName` (alias saneado), `uniqueSectionName`, `options(p, modelsDirAbs)` (mapa clave INI → valor), `sortedProfileKeys`, `iniKey` (c→ctx-size, ngl→n-gpu-layers, fa→flash-attn, t→threads, tb→threads-batch, b→batch-size, ub→ubatch-size, n→n-predict, np→parallel, ctk→cache-type-k, ctv→cache-type-v, s→seed; resto sin guiones iniciales), `formatValue` (string/bool `true|false`/números; arrays y mapas → no soportado), `resolveModelPath` (wrapper de `config.ResolveModelPath` que aplica `ToSlash`: absoluta/URL tal cual; relativa → modelsDir; separadores `/`).

### internal/ui — API exportada
- `const AppName = "llama-launcher"` · `const AppVersion = "1.1.1"`.
- `const ColorCyan = "36"` (fichero del modelo) · `const ColorYellow = "33"` (`host:puerto`) — códigos ANSI para `Color`.
- `func Color(s, code string) string` — `\x1b[<code>m` + s + `\x1b[0m` SOLO si `colorEnabled()`; en pipes/tests o consolas sin VT devuelve s intacto.
- `func PrintHeader()` — cabecera nombre/versión + separador + línea en blanco.
- `func ClearScreen()` — por plataforma (`term_windows.go` ANSI VT/`cmd /c cls`, `term_unix.go` ANSI); no-op si stdout no es consola.
- `func ReadLine() (string, bool)` — lector único `bufio.Reader` (absorbe `\r\n` y BOM UTF-8); ok=false en EOF.
- `func Pause(prompt string)` — imprime prompt y espera ENTER.
- Internas por plataforma: `isTerminal(*os.File) bool` (ui.go), `colorEnabled() bool`, y en Windows `enableVT() bool` + `vtOnce sync.Once`/`vtOK` (SetConsoleMode idempotente).

### cmd/llama-launcher/main.go
- `func main()` — `path := config.Path()`; `config.Load` (error → exit 1); nil → `config.Default()` + `config.Save`; `menuLoop(path, cfg)`.
- `func menuLoop(path string, cfg *config.Config)` — limpia + cabecera + `printMenu(config.Enabled(cfg.Profiles))` + `fmt.Print("Introduce un código: ")` + `ui.ReadLine`; número → `runProfile(cfg, enabled[n-1])`; `u/U` → `config.Update`; `r/R` → `refreshConfig` (error → mensaje + `ui.Pause`); `0` → salir; vacío → repintar.
- `func refreshConfig(path string, cfg *config.Config) error` — `config.Load` y `*cfg = *reloaded`; error si es inválido o no existe (`config.json no existe`), dejando intacta la config en memoria.
- `func runProfile(cfg *config.Config, p config.Profile)` — ya NO recibe `path` (no lo usaba): stat del `.gguf`, `runner.BuildArgs`, comando `$ ...`, `runner.RunServer`, pausa si fallo real.
- `func printMenu(entries []config.Profile)` — recibe la lista YA filtrada: `N) alias — <file en cian> (<host:puerto> en amarillo)`, host por defecto `127.0.0.1`; pies `[U] Buscar nuevos modelos y actualizar config` / `[R] Refrescar memoria con datos de config` / `[S] Modo servidor (Pi): genera preset.ini y arranca llama-server` / `[0] Salir`.
- `func startServerMode(cfg *config.Config)` — `[S]`: exige perfiles habilitados; `preset.Save(preset.Path(config.ModelsDirAbs(cfg.ModelsDir)), cfg)`; `runner.ResolveServerBin(cfg.ServerBin)`; args `[bin, --models-preset, <preset>, --port, 8080, --host, 0.0.0.0]`; `runArgs(config.Dir(), args)`.
- `func runArgs(dir string, args []string)` — imprime `$ comando`, ejecuta `runner.RunServerIn(dir, args)` y pausa solo si `exitCode != 0 && !interrupted` (compartido con `runProfile`, que usa dir "").
- `const serverModeHost = "0.0.0.0"` · `const serverModePort = "8080"` — parámetros fijos del router (pedido usuario).

## 3. Banco de Ideas / Funciones Pendientes de Implementar

- `func checkDuplicatePort(entries []Profile) []string` — advertir puertos duplicados en el menú (sin bloquear).
- Log a `llama-launcher.log` (`os.Create` + timestamps) — opcional.
- `cmd.Env` con `CHCP 65001` si se detecta conflicto cp1252 ↔ UTF-8 en la consola.
- Ampliar `shortFlags` si llama.cpp expone más flags cortos (regenerar la lista desde `tools/server/README.md`).
- `preset.shortKeys`: ampliarlo con los alias cortos nuevos (`ts`, `sm`, `mg`, `ctk`, `np`, `cb`, `mm`, `md`…) para normalizarlos a forma larga en `[S]`; el ejemplo oficial de `docs/preset.md` usa claves cortas (`hf = ...`), verificar qué acepta el parser antes de cambiar.
- `-tags` de build para rutas por plataforma (fuera de alcance actual).
- Validar en pruebas manuales: `alias` repetido y `--alias` duplicado → comportamiento de llama.cpp.
- `extra_args` con elementos numéricos: reusar `formatJSONValue` (cubierto por diseño; verificar).
- Prueba de flujo con binario fake: `fakeserver.go` (fuera del módulo, `go run` directo) — variante A: imprime `FAKE-SERVER-RUNNING` + duerme 90s (arranque/Ctrl+C); variante B: `os.Exit(2)` (error + ENTER). PATH temporal `.smoketest/bin` delante; Ctrl+C real vía P/Invoke `GenerateConsoleCtrlEvent(CTRL_BREAK=1, 0)` sobre el grupo de consola compartido.

## 4. Decisiones Técnicas y Restricciones

- SOLO stdlib (cero dependencias externas): os, exec, context, signal, bufio, encoding/json, filepath, sort, strconv, strings, math, errors, fmt, io, syscall.
- `map[string]any`: ninguna pérdida de claves; `encoding/json` ordena alfabéticamente las claves de mapa en la salida (no se preserva el orden de inserción) — aceptado ante el requisito de esquema abierto.
- float64 desde JSON: `v == math.Trunc(v,1)` → `strconv.FormatInt(int64(v),10)`; si no → `strconv.FormatFloat(v,'f',-1,64)` (Go ≥ 1.24; máquina con 1.27.4).
- `file` se guarda como string plano relativo a `modelsDir` (campo de config.json); `filepath.Join` solo al lanzar.
- Sin shell jamás: `exec.CommandContext` con argumentos exactos → los espacios en rutas (OneDrive `work AI`) funcionan correctamente.
- Señales: `signal.Notify(os.Interrupt, syscall.SIGTERM)`. Windows: el runtime entrega CTRL+C/CTRL_BREAK (consola) como `os.Interrupt`; Linux: SIGINT/SIGTERM. `Process.Cancel` del `CommandContext` envía CTRL_BREAK al hijo (llama.cpp lo gestiona) → fin limpio. Salida por señal ≠ error (no se pausa).
- Ctrl+C sin servidor en ejecución: termina la app (comportamiento estándar de consola); solo se gestiona durante `runServer`.
- Salidas del hijo heredadas directo a `os.Stdout`/`os.Stderr` (tiempo real, sin buffers); si son nil (arranque sin consola) → `io.Discard`.
- `pause(ENTER)` solo tras fallo real (`exitCode != 0 && !interrupted`) o error de arranque (binario no resuelto / modelo ausente); nunca tras Ctrl+C limpio.
- Creación de `config.json`: solo si no existe al arrancar; SIEMPRE junto al ejecutable (`configPath()`); creación con defaults + `profiles` vacíos (SIN escaneo). `scanModels` SOLO en `[U]`/`updateConfig`.
- Pantalla: en CADA repintado del menú se limpia (`clearScreen`) y se imprime la cabecera (`AppName` + `AppVersion`), para que el menú quede arriba (pedido usuario 2026-11-17). Windows: ANSI solo si se logra activar VT (`SetConsoleMode` vía `syscall.NewLazyDLL`); si no, `cmd /c cls`. Si `stdout` no es consola (pipe, tests, `.smoketest`) **no se limpia ni se emiten escapes**. Consecuencia aceptada: al volver al menú se borra lo impreso por `runProfile` (comando y log del servidor).
- "Opción no válida." (entrada no numérica o fuera de rango) imprime el aviso y hace `pause(ENTER)` antes de repintar; sin la pausa el aviso desaparecería al limpiar la pantalla.
- Entrada: `bufio.NewReader(os.Stdin)` único (reutilizado entre menú/pausas); menú por LÍNEAS sin cursor (pedido usuario 2026-11-17): número → lanza directo (multi-dígito OK), `0` y `u/U` inmediatos, ENTER/vacío → repintar. Tolerancia a BOM UTF-8: `loadConfig` recorta el BOM del archivo y `readLine` recorta `\uFEFF` de la línea (stdin pipeado por PowerShell/Notepad).
- Rutas Windows en config.json: el usuario escribe naturalmente `C:\AI\llama.cpp` (backslash simple = JSON inválido). `loadConfig` sanitiza `\X`→`\\X` SOLO si el unmarshal falla (reintentar); **si se saneó → reescribir el archivo en disco con los bytes saneados** (persistir la corrección; un JSON válido nunca se toca → idempotente).
- Tipos de perfiles: en memoria (`defaultProfile`) los números son `int` y los bools `bool`; al cargar desde JSON son `float64`/`bool`. `buildArgs`/`formatJSONValue` deben aceptar AMBOS (fix 2026-11-17: warnings + flags omitidos al lanzar). Bool `false` se omite; `true` → SOLO la clave (flag sin valor: llama.cpp `--jinja`; los bools tipo `--flash-attn` también aceptan la forma sin valor).
- JSON compañero de modelo: `<modelo>.json` junto al `.gguf` puede contener un ARRAY de configuraciones (cada una con su propio `alias`) **o un objeto único** (se envuelve en un slice de 1). Cada elemento define un perfil; `file` SIEMPRE sobreescrito por la ruta relativa (para el menú/stat); solo se usa si el modelo aún no está referenciado en config.json. **inválido → warning + `defaultProfile`; ausente → `defaultProfile` silencioso** (sin spam).
- Entradas inválidas (`alias`/`file` ausentes o no string): omitidas del menú con warning; conservadas sin cambios al guardar (esquema abierto).
- Rutas de trabajo: `modelsDir` y `serverBin` son CAMPOS de config.json (defaults `"models"` / `"llama-server"`); la ruta del config.json SIEMPRE esta junto al ejecutable (`configPath()` = `filepath.Dir(os.Executable())`). Los valores reales del usuario (C:/AI/llama.cpp) van en su propio config.json, nunca en el codigo.
- Targets: Windows (consola) y Linux; Go ≥ 1.24; sin CGO, sin dependencias.
- Layout (2026-11-18): `cmd/llama-launcher` (package main: menú y lanzamiento) + `internal/{config,runner,ui}`; imports `llama-launcher/internal/...` con el module actual (sin dominio: válido para builds locales, `go vet/build` OK). `config.json` sigue junto al ejecutable → al compilar en la raíz queda en la raíz. Único estado global: `ui.stdinReader`. README y `.vscode` apuntan a `./cmd/llama-launcher`.
- config.json v3 (2026-11-18): raíz `{modelsDir, serverBin, defaults, profiles}`. `defaults` (`Profile`) es la PLANTILLA de los modelos NUEVOS; se serializa antes de `profiles` (orden de campos del struct). `Load` ejecuta `ensureDefaults` y solo reescribe el archivo si faltaba el bloque o alguna clave (idempotente una vez completo) → un `defaults` ya existente NUNCA se toca ni se regenera.
- `defaults.api-key` = UUID v4 generado con `crypto/rand` (16 bytes + version 4 + variante RFC 4122; fallback al reloj si `rand` falla). Se envía como `--api-key <uuid>` y los modelos nuevos lo HEREDAN (mismo valor) hasta que el usuario lo cambie a mano.
- `markMissing` (solo en `[U]`, tras el escaneo): `disabled:true` a los perfiles cuyo `file` no aparece; **nunca** reactiva (`disabled` manual se respeta) ni borra. Comparación `strings.ToLower(filepath.ToSlash(rel))` (tolera `/` vs `\` y caja). `referenced` usa la misma normalización → no se duplican perfiles por diferencias de separador.
- Consecuencia aceptada: `[U]` con un `modelsDir` equivocado/vacío deshabilita todos los perfiles no encontrados (recuperable a mano); documentado en README.
- `[R]` = solo relee `config.json` (SIN escaneo, SIN escribir): `*cfg = *reloaded`; si es inválido o no existe → mensaje + ENTER y se conserva la config en memoria. Para altas/ocultar modelos ausentes hay que usar `[U]`.
- Colores: `ui.Color(s, code)` emite `\x1b[<code>m…\x1b[0m` SOLO si `colorEnabled()` (Windows: stdout consola + VT activado una vez con `sync.Once`; Unix: stdout consola). En pipes/tests/consolas sin VT, texto plano sin escapes (la limpieza de pantalla usa el mismo predicado y cae a `cmd /c cls`). Fichero del modelo en cian (`36`), `(host:puerto)` en amarillo (`33`); `host` ausente → `127.0.0.1` (default de llama.cpp).
- `AppVersion` 1.1.1 (features `[R]`, `defaults`, ocultar ausentes, colores; bump manual del usuario).
- Prompt del menú: `fmt.Print("Introduce un código: ")` se imprime sin salto de línea justo después del pie y antes de `ui.ReadLine` (la opción se teclea en esa misma línea). Si `stdout` es consola y no hay repintado inmediato, el texto queda visible; tras cualquier acción el `ClearScreen` lo borra junto con el menú.
- Modo servidor `[S]` (2026-11-21): router de llama-server para la Pi. Genera `<modelsDir>/preset.ini` (SUSTITUYE el anterior) desde los perfiles habilitados y lanza `[serverBin, --models-preset, <preset>, --port, 8080, --host, 0.0.0.0]` con `cmd.Dir = config.Dir()`. Formato INI verificado en llama.cpp (`common/preset.cpp` + `docs/preset.md`): secciones `[nombre]`, claves = argumentos SIN guiones, UN valor por clave (parser a mapa: claves repetidas se sobrescriben), claves desconocidas abortan la carga.
- En el preset NO se escriben `host`/`port`/`api-key`/`alias` (el router los elimina/sobrescribe; van en la línea de comandos) ni `disabled` (no es un argumento de llama.cpp → error). `file` → `model` con ruta ABSOLUTA (recomendado por llama.cpp; las relativas se resuelven contra el CWD del servidor).
- Claves cortas normalizadas a forma larga (`ngl`→`n-gpu-layers`, `fa`→`flash-attn`, `c`→`ctx-size`…). Si dos claves del mismo perfil apuntan a la misma clave INI (p.ej. `n-gpu-layers` + `ngl`), gana la última en orden alfabético de clave de config (mismo criterio que `BuildArgs`). Bools `false` se omiten y `true` → `true`.
- Arrays/mapas no representables en el INI (un valor por clave) → se omiten con warning; perfiles sin `alias`/`file` → omitidos con warning. `[S]` NO escanea: usa los perfiles de `config.json` en memoria (`[U]` primero si faltan modelos).
- `mmproj` es una RUTA en el preset: si el valor es relativo se prefija con `modelsDir` (absoluta), si ya es absoluto o una URL (`://`) se deja tal cual, siempre con separadores `/`. Sin esto llama-server resolvería el nombre contra SU CWD y no encontraría el fichero. `model` usa la misma resolución (`resolveModelPath`).
- `[n]` (lanzamiento directo) usa la MISMA resolución para claves de ruta (`mmproj`, `lora`, `control-vector`, `grammar-file`, `chat-template-file`, `model-draft`) vía `config.ResolveModelPath` + `runner.resolvePathValue`: un `mmproj` relativo en el perfil se envía como `<modelsDir>/mmproj-*.gguf`. URLs (`://`) y rutas absolutas se dejan intactas; el resto de claves string no se tocan. Sin esto llama-server (cwd = el del launcher) no encontraba el proyector al lanzar desde el menú.
