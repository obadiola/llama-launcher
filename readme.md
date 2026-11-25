# llama-launcher

Lanzador de consola para modelos **GGUF** ejecutados con `llama-server` (llama.cpp).
Lee un `config.json` con perfiles de modelos, muestra un menú numerado y arranca el
servidor con los flags de cada perfil.

- **Sin dependencias externas**: solo Go stdlib (`go.mod` sin `require`).
- **Windows y Linux** (consola). Sin CGO.
- Esquema de configuración **abierto**: cualquier clave de un perfil se traduce a un flag de `llama.cpp`; no hay lista cerrada de opciones.

## Requisitos

- **Go 1.24+** para compilar (probado con go1.27.4). El ejecutable ya compilado no necesita Go.
- **`llama-server`** de llama.cpp, indicado en `serverBin` (ruta absoluta o relativa al ejecutable) o disponible en el `PATH`.
- Los modelos `.gguf` en la carpeta indicada por `modelsDir`.

## Compilar y ejecutar

Windows (PowerShell):

```powershell
go build -o llama-launcher.exe ./cmd/llama-launcher
.\llama-launcher.exe
```

(o directamente `go run ./cmd/llama-launcher`)

Linux:

```bash
go build -o llama-launcher ./cmd/llama-launcher
./llama-launcher
```

En VS Code hay una tarea `build` y una configuración de depuración `(debug) llama-launcher`
(F5) en `.vscode/`.

> `config.json` se lee y se guarda **siempre junto al ejecutable** (`filepath.Dir(os.Executable())`),
> no en el directorio actual: si copias el `.exe` a otra carpeta, usará (o creará) el `config.json` de esa carpeta.

## Cómo funciona

1. **Arranque**: busca `config.json` junto al ejecutable.
   - Si **no existe**, lo crea con valores por defecto (`modelsDir: "models"`, `serverBin: "llama-server"`,
     el bloque `defaults` y `profiles: []`). **No escanea modelos al arrancar.**
   - Si existe, lo carga (tolerando `\` sin escapar y BOM UTF-8) y **completa `defaults`** con las claves
     que falten (por ejemplo `api-key` con un UUID nuevo), guardándolo.
2. **Bucle del menú**: en cada vuelta limpia la pantalla, imprime la cabecera
   (`llama-launcher v1.1.1`) y la lista de perfiles habilitados, muestra `Introduce un código: `
   y espera una línea.
3. **Al elegir un perfil**: comprueba que el `.gguf` exista, construye los argumentos,
   muestra el comando (`$ ...`) y ejecuta `llama-server` **en primer plano** (su salida va
   directo a la consola). Al terminar vuelve al menú.
4. **Ctrl+C durante el servidor**: se envía al proceso hijo (CTRL_BREAK en Windows,
   SIGINT/SIGTERM en Linux), el servidor termina y se vuelve al menú **sin pausa**.
   Si el servidor falla (código de salida ≠ 0), se muestra `Pulsa ENTER para continuar...`.
5. **Modo servidor `[S]`**: genera `<modelsDir>/preset.ini` con los perfiles habilitados y lanza
   `llama-server --models-preset <preset.ini> --port 8080 --host 0.0.0.0` (router de llama.cpp;
   ver [Modo servidor](#modo-servidor-pi-s-y-presetini)).

## El menú

```
llama-launcher v1.1.1
=====================

1) gemma-4-E2B-it-q8 — gemma/gemma-4-E2B-it-Q8_0.gguf (127.0.0.1:8080)
2) qwen3.8-27b — Qwen3.8-27B-UD-IQ2_XS.gguf (127.0.0.1:8080)
[U] Buscar nuevos modelos y actualizar config
[R] Refrescar memoria con datos de config
[S] Modo servidor (Pi): genera preset.ini y arranca llama-server
[0] Salir
Introduce un código:
```

| Entrada | Acción |
|---|---|
| `1`, `2`, … (también multi-dígito) | Lanza ese perfil directamente |
| `u` / `U` | **Buscar nuevos modelos y actualizar config**: escanea `modelsDir`, añade perfiles nuevos a `config.json` y oculta los modelos ausentes |
| `r` / `R` | Relee `config.json` desde disco **sin escanear** (aplica cambios hechos con la app abierta) |
| `s` / `S` | **Modo servidor (Pi)**: genera `preset.ini` y arranca `llama-server` en modo router (ver [Modo servidor](#modo-servidor-pi-s-y-presetini)) |
| `0` | Sale del programa |
| ENTER / línea vacía | Repinta el menú (no lanza nada) |
| Cualquier otra cosa | `Opción no válida.` + ENTER para volver al menú |

- Solo se numeran los perfiles **habilitados** (`disabled` distinto de `true`); el número se refiere a esa lista.
- El pie del menú indica las acciones (`[U]` escanear, `[R]` refrescar, `[S]` modo servidor, `[0]` salir) y, justo después,
  se muestra el prompt `Introduce un código: ` en la misma línea donde se escribe la opción.
- Cada perfil se muestra como `alias — file (host:puerto)`: el **fichero va en cian** y el
  **`host:puerto` en amarillo**. Si el perfil no tiene `host` se muestra `127.0.0.1` (el host por defecto de llama.cpp).
- Los colores y la limpieza de pantalla solo se emiten si `stdout` es una consola que acepta secuencias ANSI
  (en pipes, scripts y tests la salida queda limpia; en consolas Windows sin VT se usan sin color).
- Si no hay perfiles habilitados: `No hay perfiles habilitados. Usa [U] para escanear modelos.`

## Modo servidor (Pi): `[S]` y `preset.ini`

La opción `[S]` arranca `llama-server` en **modo router**: sirve todos los modelos a la vez y elige
cuál cargar según el campo `model` de cada petición (pensado para una Raspberry Pi).

1. Genera (o **sustituye**) `<modelsDir>/preset.ini` con los perfiles habilitados de `config.json`.
2. Ejecuta, con el directorio del ejecutable como directorio de trabajo:

   ```
   <serverBin> --models-preset <modelsDir>/preset.ini --port 8080 --host 0.0.0.0
   ```

   `serverBin` y `modelsDir` salen de `config.json`; `--port 8080` y `--host 0.0.0.0` son fijos.

> En la Pi, con `"modelsDir": "models"` y `"serverBin": "motor/llama-server"` (ambos relativos al
> ejecutable), el comando resultante es exactamente
> `./motor/llama-server --models-preset ./models/preset.ini --port 8080 --host 0.0.0.0`.

`preset.ini` es el formato INI de presets de llama.cpp: cada modelo es una sección con el alias como
nombre y las opciones como `clave = valor` (los argumentos de `llama-server` **sin** los guiones iniciales):

```ini
; generado por llama-launcher [S]; se sustituye en cada arranque

[qwen2.5-coder-14b]
model = C:/modelos/qwen2.5-coder-14b-instruct-q4_k_m.gguf
cache-type-k = q8_0
cache-type-v = q8_0
ctx-size = 32768
flash-attn = on
jinja = true
n-gpu-layers = 99
```

Traducción de claves al INI:

| En el perfil | En `preset.ini` |
|---|---|
| `alias` | nombre de la sección (`[alias]`; repetido → `alias-2`, `alias-3`…) |
| `file` | `model` con la ruta **absoluta** del `.gguf` |
| `mmproj` | `mmproj` con la ruta **absoluta** del fichero (si es relativa se resuelve contra `modelsDir`) |
| `c` | `ctx-size` |
| `ngl` / `n-gpu-layers` | `n-gpu-layers` |
| `fa` / `flash-attn` | `flash-attn` |
| `t` / `tb` | `threads` / `threads-batch` |
| `b` / `ub` | `batch-size` / `ubatch-size` |
| `n` | `n-predict` |
| `np` | `parallel` |
| `ctk` / `ctv` | `cache-type-k` / `cache-type-v` |
| `s` | `seed` |
| Resto (`temp`, `top-k`, `jinja`, `reasoning-budget`…) | se usa la misma clave |

- `host`, `port`, `api-key` y `alias` **no** se escriben en el preset: los controla el router de
  llama.cpp y los sobrescribe/elimina al cargar el modelo (por eso van en la línea de comandos).
- `disabled` tampoco: no es un argumento de `llama.cpp` y haría fallar el archivo entero; los perfiles
  deshabilitados simplemente no aparecen.
- `mmproj` se trata como `model`: si el valor es relativo se prefija con `modelsDir` (ruta completa);
  si ya es absoluto o una URL se deja tal cual. Así llama-server encuentra el proyector sin depender
  de su directorio de trabajo.
- Lo mismo se aplica al **lanzamiento directo** (`[n]` del menú): las claves de ruta `mmproj`, `lora`,
  `control-vector`, `grammar-file`, `chat-template-file` y `model-draft` se resuelven contra
  `modelsDir` cuando su valor es relativo (los absolutos y las URLs se dejan intactos).
- `true` → `true`; `false` y claves propias de la app se omiten. Los arrays y mapas anidados no se
  pueden representar (el INI admite un valor por clave): warning por `stderr` y se omiten.
- Si un perfil define dos claves que apuntan al mismo nombre INI (por ejemplo `n-gpu-layers` y `ngl`),
  gana la última en orden alfabético, igual que al lanzar con flags.
- `[S]` **no escanea** modelos: usa los perfiles que hay en memoria. Si has añadido modelos, pulsa `[U]` antes.
- Si no hay ningún perfil habilitado, muestra el aviso y no genera el preset.
- El fichero se reescribe **completo** en cada `[S]` (no se conserva nada editado a mano en él).

## config.json

Es la única configuración de la app y vive junto al ejecutable.

```json
{
  "modelsDir": "C:/AI/llama.cpp/models",
  "serverBin": "C:/AI/llama.cpp/motor/llama-server.exe",
  "defaults": {
    "api-key": "43938026-dceb-4e0d-948d-e332a8a72892",
    "c": 8192,
    "fa": "on",
    "host": "127.0.0.1",
    "ngl": 99,
    "port": 8080,
    "t": 8
  },
  "profiles": []
}
```

| Campo | Tipo | Valor por defecto | Descripción |
|---|---|---|---|
| `modelsDir` | string | `"models"` | Carpeta de modelos. Si es relativa, se resuelve respecto al ejecutable. |
| `serverBin` | string | `"llama-server"` | Ejecutable de llama.cpp: ruta absoluta, relativa al ejecutable o nombre en el `PATH`. En Windows, fallback automático a `llama-server.exe`. |
| `defaults` | objeto | ver abajo | Valores que hereda cada modelo **nuevo** al escanear. |
| `profiles` | array | `[]` | Lista de perfiles (objetos de esquema abierto). |

### `defaults` — valores de los modelos nuevos

`defaults` es una **plantilla**: cuando `[U]` añade un modelo sin JSON compañero, el perfil se crea copiando
`defaults` y añadiendo `alias`, `file` y `disabled: false`.

- Se puede editar a mano: cualquier clave (`c`, `port`, `temp`, `jinja`, `n-gpu-layers`…) se traduce a flags
  igual que en un perfil.
- Si falta el bloque o alguna de sus claves, la app lo **crea/completa y guarda** `config.json` con
  `host`, `port`, `c`, `ngl`, `t`, `fa` y `api-key`.
- `api-key` se rellena con un **UUID v4** generado con `crypto/rand`; se envía como `--api-key <uuid>`.
  Se genera una sola vez y queda guardado, así que todos los modelos nuevos heredan ese mismo valor
  (cámbialo a mano si quieres otro por modelo).
- Los perfiles ya existentes **no** se tocan: `defaults` solo afecta a los modelos añadidos después.

### Perfiles

Cada perfil es un objeto JSON. La app reconoce estas claves:

| Clave | Uso |
|---|---|
| `alias` | Nombre mostrado en el menú y enviado como `--alias`. |
| `file` | Ruta del `.gguf` **relativa a `modelsDir`**. Se envía como `-m <modelsDir>/<file>`. |
| `disabled` | `true` → el perfil se oculta del menú (no se borra del JSON). |
| `host` | Se usa para mostrar `(host:puerto)` en el menú (por defecto `127.0.0.1`); además se envía como `--host`. |
| `port` | Se usa para mostrar `(host:puerto)` en el menú; además se envía como `--port`. Acepta número o string. |
| cualquier otra | Se traduce a un flag de `llama-server` (ver abajo). |

### Traducción de un perfil a argumentos

El comando se construye **sin shell** (argumentos exactos, los espacios en rutas funcionan):

1. `llama-server -m <modelsDir>/<file> --alias <alias>` (los dos campos son obligatorios).
2. Después, el resto de claves **en orden alfabético**, omitiendo `file`, `alias` y `disabled`:

| Clave en el perfil | Flag generado | Valor |
|---|---|---|
| 1 carácter (`c`, `t`, `b`…) | `-c`, `-t`, `-b` | `-c 8192` |
| Alias corto documentado (`ngl`, `fa`, `ts`, `sm`, `ctk`, `ctv`, `np`, `mm`, `md`, `to`…) | `-ngl`, `-fa`, `-ts`, `-sm`, `-ctk`… | `-ngl 99`, `-fa on`, `-ts 0.5,0.5` |
| Ya empieza por `-` (`"-ngl"`) | se usa tal cual | `-ngl 99` |
| Resto (`port`, `temp`, `jinja`…) | `--port`, `--temp`, `--jinja` | `--port 8080` |

La lista de alias cortos (`-cl`, `-tb`, `-ub`, `-kvo`, `-ctk`, `-ctv`, `-ot`, `-sm`, `-ts`, `-mg`,
`-mm`, `-mmdev`, `-np`, `-cb`, `-to`, `-sps`, `-md`, `-ngld`… incluidos los de borrador `-td`,
`-tbd`, `-devd`) está en `internal/runner/runner.go` (`shortFlags`), copiada de la ayuda oficial de
`llama-server`. Los alias distinguen mayúsculas (`Cr`/`Cb`/`Cd`…). Cualquier clave que no esté en esa
lista se envía en forma larga (`--clave`), que es la forma que recomienda llama.cpp.

Reglas de valor:

- **string / número** → `flag valor`. Los números enteros se envían sin decimales (`8192`, no `8192.0`).
- **booleano `true`** → **solo el flag**, sin valor (`"jinja": true` → `--jinja`).
- **booleano `false`** → se omite por completo.
- **array** → el flag se repite por cada elemento (`"lora": ["a.gguf","b.gguf"]` → `--lora a.gguf --lora b.gguf`).
- **objeto/mapa anidado** → warning por `stderr` y se omite (no hay forma de expresarlo en un flag).

### Ejemplo real

```json
{
  "alias": "gemma-4-E4B-it-Cline",
  "file": "gemma-4-E4B-it-UD-Q5_K_XL.gguf",
  "c": 81920,
  "fa": "on",
  "jinja": true,
  "port": "8080"
}
```

produce (las claves restantes van ordenadas alfabéticamente):

```
$ C:/AI/llama.cpp/motor/llama-server.exe -m C:/AI/llama.cpp/models/gemma-4-E4B-it-UD-Q5_K_XL.gguf --alias gemma-4-E4B-it-Cline -c 81920 -fa on --jinja --port 8080
```

> Nota: `fa` es un flag corto (`-fa`) y en llama.cpp reciente acepta `on`, `off` o `auto`.
> Con `"fa": true` se enviaría `-fa` sin valor.

## Escanear modelos (`[U]`)

`u`/`U` es el **único** momento en que se escanea la carpeta de modelos:

1. Recorre `modelsDir` **recursivamente**.
2. Considera archivos `.gguf` (extensión insensible a mayúsculas).
3. Ignora los `mmproj-*.gguf`: son el parámetro `--mmproj` de un modelo, no modelos IA.
4. **Marca `disabled: true`** en los perfiles cuyo `file` ya no está en el escaneo (modelo borrado o renombrado):
   desaparecen del menú pero **siguen en el JSON**. Nunca reactiva un perfil deshabilitado a mano.
5. Añade solo los modelos cuyo `file` no esté ya en ningún perfil (incluidos los `disabled`). La comparación
   tolera `/` frente a `\` y diferencias de mayúsculas.
6. Para cada modelo nuevo busca un **JSON compañero** (ver abajo):
   - válido → crea perfiles con sus parámetros;
   - ausente → perfil con los valores de `defaults`, en silencio;
   - inválido → warning por `stderr` + perfil con los valores de `defaults`.
7. Guarda `config.json` (indentación de 2 espacios; las claves de los mapas se escriben en orden alfabético).
   **Nunca borra perfiles.**

### Refrescar memoria con datos de config (`[R]`)

`r`/`R` vuelve a leer `config.json` **desde disco y sin escanear** modelos, y sustituye la configuración en memoria:

- Útil si editas a mano `alias`, `port`, `disabled`, `defaults`… con la app ya abierta.
- Si el fichero es inválido o ha desaparecido, muestra el error, **conserva** la configuración anterior y espera ENTER.
- Al repintar el menú se aplican los cambios (los `disabled` ya no aparecen).
- No añade modelos nuevos ni modifica el JSON: para eso está `[U]`.

### JSON compañero de un modelo

Junto al `.gguf`, con el mismo nombre y extensión `.json`
(`modelo.gguf` → `modelo.json`). Admite dos formas:

**Array** → un perfil por elemento (varios alias/configuración para el mismo modelo):

```json
[
  { "alias": "gemma-4-fast", "port": 8081, "c": 4096 },
  { "alias": "gemma-4-long", "port": 8082, "ctx-size": 81920, "jinja": true }
]
```

**Objeto** → un único perfil:

```json
{ "alias": "qwen3.8", "ctx-size": 81920, "jinja": true }
```

Reglas:

- `file` se **sobrescribe siempre** con la ruta relativa del `.gguf` (no hace falta ponerlo).
- El resto de claves es libre; se traducen a flags igual que en `config.json`.
- Solo se usa cuando ese `.gguf` todavía no está referenciado en `profiles`.

### Perfil nuevo (hereda `defaults`)

Es lo que se crea para un modelo sin JSON compañero (o con JSON inválido): copia de `defaults` + `alias`,
`file` y `disabled: false`:

```json
{
  "alias": "<nombre del .gguf sin extensión>",
  "file": "<ruta relativa al .gguf>",
  "disabled": false,
  "host": "127.0.0.1",
  "port": 8080,
  "c": 8192,
  "ngl": 99,
  "t": 8,
  "fa": "on",
  "api-key": "<UUID v4 de defaults>"
}
```

## Detalles de Windows y robustez

- **Rutas con `\`**: si escribes `"modelsDir": "C:\AI\llama.cpp\models"` (backslash simple, JSON inválido),
  la app lo corrige automáticamente (`\` → `\\`) y **reescribe** el `config.json` corregido. Un JSON válido no se toca.
- **BOM UTF-8**: se tolera en `config.json` y en la entrada de teclado (guardados con Notepad/PowerShell antiguo).
- **Sin shell**: nunca se invoca `cmd.exe` para lanzar el modelo, así que rutas con espacios
  (`...\OneDrive\work AI\...`) y caracteres especiales funcionan sin comillas manuales.
- **Ctrl+C**: con el servidor en marcha se reenvía al hijo (CTRL_BREAK / señal) y se vuelve al menú.
  Sin servidor en marcha, cierra la app (comportamiento estándar de consola).
- **Salida del servidor**: se hereda directamente a la consola, en tiempo real.
  Ojo: al volver al menú la pantalla se limpia, así que el log del servidor ya no se ve.
- **Colores**: el menú usa secuencias ANSI (fichero en cian, `host:puerto` en amarillo). En Windows se activa
  `ENABLE_VIRTUAL_TERMINAL_PROCESSING` una sola vez (`sync.Once`); si la consola no lo soporta o `stdout` no es
  consola, el texto sale **sin color y sin escapes**.

## Estructura del proyecto

```
llama-launcher/
├── cmd/llama-launcher/main.go   punto de entrada (menú, refresco, lanzamiento de perfiles)
├── internal/config/config.go    config.json: carga/guardado, defaults, escaneo, JSON compañero, getters
├── internal/preset/preset.go    preset.ini: perfiles → secciones INI del router de llama-server
├── internal/runner/runner.go    perfil → argumentos de llama-server y ejecución
├── internal/ui/ui.go            nombre/versión, cabecera, colores, lectura de línea y pausas
├── internal/ui/term_windows.go  limpieza de pantalla y VT/colores en Windows
├── internal/ui/term_unix.go     limpieza de pantalla y colores en Linux/macOS
├── go.mod
├── config.json                  configuración (se genera al primer arranque)
├── CEREBRO.md                   memoria de diseño del proyecto
└── .vscode/                     tarea `build` y depuración F5
```

| Paquete | Contenido |
|---|---|
| `main` (`cmd/llama-launcher`) | `main`, `menuLoop`, `refreshConfig`, `runProfile`, `startServerMode`, `runArgs`, `printMenu` |
| `config` (`internal/config`) | `Config`/`Profile`, `Load`, `Save`, `Update`, `ModelsDirAbs`, `String`/`Bool`/`Float`, `Enabled` (`defaults`/`markMissing` internos) |
| `preset` (`internal/preset`) | `FileName`, `Path`, `Build` y `Save`: perfiles habilitados → secciones `[alias]` con `model` y el resto de opciones |
| `runner` (`internal/runner`) | traducción del perfil a argumentos (`BuildArgs`), resolución del binario (`ResolveServerBin`) y ejecución (`RunServer`/`RunServerIn`) |
| `ui` (`internal/ui`) | `AppName`/`AppVersion`, `PrintHeader`, `Color`, `ClearScreen`, `ReadLine`, `Pause` |


La versión que muestra la cabecera (`llama-launcher v1.1.1`) se define en `internal/ui/ui.go`
(`AppName` y `AppVersion`).

## Limitaciones

- Un solo servidor a la vez: el menú queda bloqueado mientras `llama-server` corre.
- No valida los flags contra `llama.cpp`: una clave mal escrita se envía igual y el servidor fallará.
- `alias`, `file` y `disabled` son claves propias de la app (`alias` se envía como `--alias`; las otras no).
- Los mapas anidados dentro de un perfil se ignoran con un warning.
- Al volver al menú se borra el log del servidor (la pantalla se limpia).
- Al guardar, las claves de los mapas se ordenan alfabéticamente (no se conserva el orden original).
- `[U]` con un `modelsDir` equivocado (o vacío) marca `disabled: true` **todos** los perfiles cuyos modelos no
  encuentre: revisa `modelsDir` antes de escanear (no se borra nada; se pueden reactivar a mano).
- `defaults.api-key` se comparte: todos los modelos nuevos heredan el mismo UUID hasta que lo cambies.
- `[S]` requiere un `llama-server` con soporte de router (`--models-preset`); en el preset se ignoran
  `host`/`port`/`api-key` por modelo (los controla el router), y los arrays/mapas anidados se omiten.
  El `preset.ini` se reescribe completo en cada `[S]`: no edites a mano algo que quieras conservar.
- No hay tests automatizados; la validación es manual (menú, `[U]`, `[R]`, `[S]` y lanzamiento con un `llama-server` real o simulado).

## Problemas típicos

| Síntoma | Causa / solución |
|---|---|
| `error al cargar la configuración: parsear ...` | JSON irrecuperable (coma final, comillas sin cerrar). Revisa `config.json`. |
| `No se encontró ...gguf` | `file` no coincide con el modelo en `modelsDir`. Pulsa `[U]` y revisa `modelsDir`. |
| `no se encontró "..." en PATH` | `serverBin` mal escrito o `llama-server` no está en el `PATH`. Usa ruta absoluta. |
| `Opción no válida.` | Entrada no numérica o número fuera del rango de perfiles habilitados. |
| El menú muestra `host:puerto` con `:0` | El perfil no tiene `port` o no es numérico. |
| `No se pudo refrescar la configuración: ...` | `config.json` inválido o desaparecido al pulsar `[R]`; corrige el JSON y vuelve a pulsar `[R]`. |
| Un modelo no aparece en el menú | Está `disabled` (a mano o porque `[U]` no encontró su `.gguf`), su `file` ya existe en otro perfil, o es un `mmproj-*` (no se lista). |
| El modelo arranca y muere al instante | Flags inválidos para tu versión de `llama-server`: lee su error antes de que se repinte el menú. |
