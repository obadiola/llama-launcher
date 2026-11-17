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
go build -o llama-launcher.exe .
.\llama-launcher.exe
```

Linux:

```bash
go build -o llama-launcher .
./llama-launcher
```

En VS Code hay una tarea `build` y una configuración de depuración `(debug) llama-launcher`
(F5) en `.vscode/`.

> `config.json` se lee y se guarda **siempre junto al ejecutable** (`filepath.Dir(os.Executable())`),
> no en el directorio actual: si copias el `.exe` a otra carpeta, usará (o creará) el `config.json` de esa carpeta.

## Cómo funciona

1. **Arranque**: busca `config.json` junto al ejecutable.
   - Si **no existe**, lo crea con valores por defecto (`modelsDir: "models"`, `serverBin: "llama-server"`)
     y `profiles: []`. **No escanea modelos al arrancar.**
   - Si existe, lo carga (tolerando `\` sin escapar y BOM UTF-8).
2. **Bucle del menú**: en cada vuelta limpia la pantalla, imprime la cabecera
   (`llama-launcher v1.0.0`) y la lista de perfiles habilitados, y espera una línea.
3. **Al elegir un perfil**: comprueba que el `.gguf` exista, construye los argumentos,
   muestra el comando (`$ ...`) y ejecuta `llama-server` **en primer plano** (su salida va
   directo a la consola). Al terminar vuelve al menú.
4. **Ctrl+C durante el servidor**: se envía al proceso hijo (CTRL_BREAK en Windows,
   SIGINT/SIGTERM en Linux), el servidor termina y se vuelve al menú **sin pausa**.
   Si el servidor falla (código de salida ≠ 0), se muestra `Pulsa ENTER para continuar...`.

## El menú

```
llama-launcher v1.0.0
=====================

1) gemma-4-E2B-it-q8 — gemma/gemma-4-E2B-it-Q8_0.gguf (port: 8080)
2) qwen3.8-27b — Qwen3.8-27B-UD-IQ2_XS.gguf (port: 8080)
[U] Actualizar config · [0] Salir · número para lanzar
```

| Entrada | Acción |
|---|---|
| `1`, `2`, … (también multi-dígito) | Lanza ese perfil directamente |
| `u` / `U` | Escanea `modelsDir` y añade perfiles nuevos a `config.json` |
| `0` | Sale del programa |
| ENTER / línea vacía | Repinta el menú (no lanza nada) |
| Cualquier otra cosa | `Opción no válida.` + ENTER para volver al menú |

- Solo se numeran los perfiles **habilitados** (`disabled` distinto de `true`); el número se refiere a esa lista.
- Cada perfil se muestra como `alias — file (port: X)`.
- Si no hay perfiles habilitados: `No hay perfiles habilitados. Usa [U] para escanear modelos.`
- La pantalla se limpia en cada repintado (solo si `stdout` es una consola; en pipes/scripts no se emiten secuencias ANSI).

## config.json

Es la única configuración de la app y vive junto al ejecutable.

```json
{
  "modelsDir": "C:/AI/llama.cpp/models",
  "serverBin": "C:/AI/llama.cpp/motor/llama-server.exe",
  "profiles": []
}
```

| Campo | Tipo | Valor por defecto | Descripción |
|---|---|---|---|
| `modelsDir` | string | `"models"` | Carpeta de modelos. Si es relativa, se resuelve respecto al ejecutable. |
| `serverBin` | string | `"llama-server"` | Ejecutable de llama.cpp: ruta absoluta, relativa al ejecutable o nombre en el `PATH`. En Windows, fallback automático a `llama-server.exe`. |
| `profiles` | array | `[]` | Lista de perfiles (objetos de esquema abierto). |

### Perfiles

Cada perfil es un objeto JSON. La app reconoce estas claves:

| Clave | Uso |
|---|---|
| `alias` | Nombre mostrado en el menú y enviado como `--alias`. |
| `file` | Ruta del `.gguf` **relativa a `modelsDir`**. Se envía como `-m <modelsDir>/<file>`. |
| `disabled` | `true` → el perfil se oculta del menú (no se borra del JSON). |
| `port` | Se usa para mostrar `(port: X)` en el menú; además se envía como `--port`. Acepta número o string. |
| cualquier otra | Se traduce a un flag de `llama-server` (ver abajo). |

### Traducción de un perfil a argumentos

El comando se construye **sin shell** (argumentos exactos, los espacios en rutas funcionan):

1. `llama-server -m <modelsDir>/<file> --alias <alias>` (los dos campos son obligatorios).
2. Después, el resto de claves **en orden alfabético**, omitiendo `file`, `alias` y `disabled`:

| Clave en el perfil | Flag generado | Valor |
|---|---|---|
| 1 carácter (`c`, `t`, `b`…) | `-c`, `-t`, `-b` | `-c 8192` |
| `ngl`, `fa` | `-ngl`, `-fa` (lista corta) | `-ngl 99`, `-fa on` |
| Ya empieza por `-` (`"-ngl"`) | se usa tal cual | `-ngl 99` |
| Resto (`port`, `temp`, `jinja`…) | `--port`, `--temp`, `--jinja` | `--port 8080` |

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
4. Añade solo los modelos cuyo `file` no esté ya en ningún perfil (incluidos los `disabled`).
5. Para cada modelo nuevo busca un **JSON compañero** (ver abajo):
   - válido → crea perfiles con sus parámetros;
   - ausente → perfil por defecto, en silencio;
   - inválido → warning por `stderr` + perfil por defecto.
6. Guarda `config.json` (indentación de 2 espacios; las claves de los mapas se escriben en orden alfabético).
   **Nunca modifica ni borra perfiles existentes.**

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

### Perfil por defecto

Es lo que se crea para un modelo sin JSON compañero (o con JSON inválido):

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
  "fa": "on"
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

## Estructura del proyecto

| Archivo | Contenido |
|---|---|
| `main.go` | `main`, `menuLoop`, `runProfile`, `enabledProfiles`, `printMenu`, `readLine`, `pause` |
| `config.go` | `Config`/`Profile`, carga y guardado de `config.json`, escaneo, JSON compañero, getters |
| `runner.go` | traducción del perfil a argumentos (`buildArgs`) y ejecución (`runServer`) |
| `ui.go` | nombre/versión, cabecera y detección de consola |
| `term_windows.go` / `term_unix.go` | limpieza de pantalla por plataforma |
| `config.json` | configuración (se genera al primer arranque) |
| `.vscode/` | tarea `build` y depuración F5 |


La versión que muestra la cabecera (`llama-launcher v1.0.0`) se define en `ui.go`
(`AppName` y `AppVersion`).

## Limitaciones

- Un solo servidor a la vez: el menú queda bloqueado mientras `llama-server` corre.
- No valida los flags contra `llama.cpp`: una clave mal escrita se envía igual y el servidor fallará.
- `alias`, `file` y `disabled` son claves propias de la app (`alias` se envía como `--alias`; las otras no).
- Los mapas anidados dentro de un perfil se ignoran con un warning.
- Al volver al menú se borra el log del servidor (la pantalla se limpia).
- Al guardar, las claves de los mapas se ordenan alfabéticamente (no se conserva el orden original).
- No hay tests automatizados; la validación es manual (menú, `[U]` y lanzamiento con un `llama-server` real o simulado).

## Problemas típicos

| Síntoma | Causa / solución |
|---|---|
| `error al cargar la configuración: parsear ...` | JSON irrecuperable (coma final, comillas sin cerrar). Revisa `config.json`. |
| `No se encontró ...gguf` | `file` no coincide con el modelo en `modelsDir`. Pulsa `[U]` y revisa `modelsDir`. |
| `no se encontró "..." en PATH` | `serverBin` mal escrito o `llama-server` no está en el `PATH`. Usa ruta absoluta. |
| `Opción no válida.` | Entrada no numérica o número fuera del rango de perfiles habilitados. |
| El menú muestra `port: 0` | El perfil no tiene `port` o no es numérico. |
| Un modelo no aparece en el menú | Está `disabled`, su `file` ya existe en otro perfil, o es un `mmproj-*` (no se lista). |
| El modelo arranca y muere al instante | Flags inválidos para tu versión de `llama-server`: lee su error antes de que se repinte el menú. |
