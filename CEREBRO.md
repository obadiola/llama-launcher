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
- [x] **BUILD ROTO (fix 2026-11-17):** `config.go` quedó corrupto al pegar `updateConfig`/`profileFromCompanionJSON` dentro de otra función (en el commit `pre gemma`): `return &cfgfgfgfg`, dos `updateConfig`, dos `profileFromCompanionJSON` y código muerto tras un `return nil, false` → `go build` fallaba en `config.go:228`. Reconstruido: `return &cfg`; UNA `profileFromCompanionJSON` (`([]Profile, bool)`, admite array u objeto) + UNA `updateConfig` (append de todos los perfiles). gofmt/vet/build exit 0; smoke: JSON array → 2 perfiles con `file` sobrescrito, JSON objeto → 1 perfil, modelo sin JSON → defaults, `mmproj-*` excluido, lanzamiento OK (`-c 4096 -fa on --jinja --port 8081`), arranque sin config → `profiles: []`.
- [x] Build + prueba de flujo: arranque, `[U]` (escaneo + JSON compañero) y lanzamiento con servidor fake (exit 0).
- [x] FIX menú `port: 0`: perfiles con `"port": "8080"` (string) no se mostraban; `profileFloat` ahora parsea strings numéricos (`strconv` añadido a config.go). Smoke: los 10 perfiles reales muestran `port: 8080`.
- [x] FIX BOM UTF-8: `config.json` guardado con BOM (Notepad/`Out-File`) fallaba el parseo, y un `0` pipeado con BOM daba "Opción no válida". `loadConfig` recorta `utf8BOM` antes de unmarshal; `readLine` recorta `\uFEFF` de la línea. Smoke: config con BOM carga el perfil; stdin con BOM → "Adiós." exit 0.
- [x] Menú con pantalla limpia + cabecera (pedido usuario 2026-11-17): `menuLoop` hace `clearScreen()` + `printHeader()` en cada repintado; cabecera `llama-launcher v1.0.0` + línea `=`. `ui.go` (`AppName`/`AppVersion`/`isTerminal`/`printHeader`), `term_windows.go` (ANSI VT vía `SetConsoleMode` con `syscall.NewLazyDLL` + fallback `cmd /c cls`), `term_unix.go` (ANSI). Sin escapes si stdout no es consola (pipes/tests limpios). Smoke: pipe → cabecera sin ANSI; consola → limpia y menú arriba.
- [x] FIX UX derivado de la limpieza: "Opción no válida." se borraba en el repintado inmediato → ahora `pause("Pulsa ENTER para continuar...")` antes de volver al menú.
- [x] Rellenar `readme.md` con el funcionamiento completo (uso, menú, config.json, traducción de flags, JSON compañero, escaneo, detalles Windows, estructura y limitaciones). Validado: los 4 ejemplos JSON parsean; el comando de ejemplo coincide con la salida real (`-m ... --alias ... -c 81920 -fa on --jinja --port 8080`).


## 2. Arquitectura y Contratos (Firmas aprobadas)

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
- `var shortFlags = map[string]bool{"ngl": true, "fa": true}` — flags cortos (prefijo `-`); 1 letra también `-`; 2+ letras `--`; clave ya con `-` se usa tal cual.
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

## 3. Banco de Ideas / Funciones Pendientes de Implementar

- `func checkDuplicatePort(entries []Profile) []string` — advertir puertos duplicados en el menú (sin bloquear).
- `func detectMissingModels(entries []Profile) map[string]bool` — marcar en el menú los `file` que ya no existen en disco (renombre/OneDrive); sin tocar el JSON.
- Opción `[R] Recargar config` del menú (solo lectura desde disco, sin escaneo).
- Log a `llama-launcher.log` (`os.Create` + timestamps) — opcional.
- `cmd.Env` con `CHCP 65001` si se detecta conflicto cp1252 ↔ UTF-8 en la consola.
- Ampliar `shortFlags` si llama.cpp expone más flags cortos.
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