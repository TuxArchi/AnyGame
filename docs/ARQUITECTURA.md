# Arquitectura de AnyGame

## Idea

AnyPS5 ya resuelve el problema difícil: convierte un ejecutable de PS5 en un
programa nativo y proporciona las librerías del sistema. AnyGame es la capa que
convierte eso en un producto para juegos completos:

1. Encuentra y valida el título.
2. Prepara el ejecutable (SELF → ELF).
3. Ejecuta el relinker oficial con las opciones correctas.
4. Ensambla el layout de ejecución que AnyPS5 documenta.
5. Deja un paquete reproducible, con manifiesto y lanzador.

No hay emulación ni intérprete: el código del juego se ejecuta de forma nativa.

## Capas

```text
┌──────────────────────────────────────────────────────────────┐
│ CLI (internal/cli)                                           │
│  update · runtimes · info · build · run · unwrap · doctor    │
└───────────────┬───────────────────────────┬──────────────────┘
                │                           │
┌───────────────▼─────────────┐ ┌───────────▼──────────────────┐
│ anyps5 (releases)           │ │ game (descubrimiento)        │
│ GitHub API · SHA-256        │ │ eboot · módulos · param.sfo  │
│ instalación atómica         │ │ param.json · icono           │
└───────────────┬─────────────┘ └───────────┬──────────────────┘
                │                           │
┌───────────────▼─────────────┐ ┌───────────▼──────────────────┐
│ self (SELF → ELF)           │ │ relink (proceso relinker)    │
│ port de unwrap_self.py      │ │ args · log · needed.json     │
└───────────────┬─────────────┘ └───────────┬──────────────────┘
                │                           │
                └───────────────┬───────────┘
┌───────────────────────────────▼──────────────────────────────┐
│ packaging                                                    │
│ libs/ selectivas · app0/ · run.sh/run.bat · .desktop        │
│ anygame.json                                                 │
└──────────────────────────────────────────────────────────────┘
```

## Flujo de `anygame build`

```text
juego/
  eboot.bin (SELF o ELF)
  sce_module/…
  sce_sys/…
        │
        ▼
game.Discover ──► título, módulos, metadatos
        │
        ▼
self.UnwrapFile (solo si es SELF) ──► .anygame-work/eboot.elf
        │
        ▼
stageModules
  ├── desenvuelve sce_module/*.prx  (SELF → ELF)
  ├── desenvuelve Media/Modules|Plugins/*.prx
  └── deja todo en .anygame-work/modules
        │
        ▼
relinker --registry --rpath '$ORIGIN/libs' …
  --sce-module-path .anygame-work/modules
  [--module-dir Media/Modules --module-dir Media/Plugins]
  [--executable …] [--patches …]
        │
        ├──► <salida>/<id>.elf | .exe
        ├──► <id>.registry.json
        └──► <id>.needed.json
        │
        ▼
packaging.Package
  ├── libs/  (cierre transitivo desde needed.json)
  ├── app0/  (recursos enlazados o copiados + eboot.bin original)
  ├── run.sh / run.bat
  ├── <Título>.desktop + icon.png
  └── anygame.json
```

### Módulos SELF y plugins

Los volcados suelen traer los módulos (`sce_module/*.prx`, `Media/Plugins/*.prx`)
como contenedores SELF. El relinker los omite con un aviso, así que AnyGame los
desenvuelve antes a un árbol de trabajo y le pasa ese árbol como
`--sce-module-path`. Los directorios `Modules`/`Plugins` bajo el título se
detectan automáticamente (hasta 4 niveles) y se pasan como `--module-dir`; sus
originales se excluyen de `app0/` para no pisar los módulos convertidos.

### Por qué `app0/`

AnyPS5 documenta que los títulos buscan sus recursos en `app0/` y comprueban
`/app0/eboot.bin` para localizar su propio directorio. Los módulos convertidos
también se escriben ahí (`app0/sce_module/`). AnyGame respeta ese contrato y no
inventa una disposición distinta.

### Copia selectiva de librerías

El relinker escribe `<salida>.needed.json` con las librerías del sistema que el
título necesita. AnyGame parte de esa lista y añade el cierre transitivo leyendo
`DT_NEEDED` de cada `.prx` con `debug/elf`.

- Lista conocida y vacía → no se copia ninguna librería.
- Lista desconocida (sin `--registry`) → se copian todas, como red de seguridad.
- Windows → se añaden siempre `libgcc_s_seh-1.dll`, `libstdc++-6.dll` y
  `libwinpthread-1.dll`, que el runtime carga sin buscar en `PATH`.

### Recursos: enlaces por defecto

Copiar un juego de 100 GB duplicaría el espacio. `--resources link` crea enlaces
duros (con caída a copia si el sistema de archivos no lo permite); `copy` fuerza
una copia y `none` deja solo ejecutable y librerías. Los enlaces simbólicos se
preservan y, si no se pueden recrear, se copia su destino.

## Actualizador

`internal/anyps5` habla con la API de GitHub y trabaja con los assets oficiales:

- `relinker-v<versión>` / `relinker-v<versión>.exe`
- `prx-linux-v<versión>.tar.gz`
- `prx-windows-v<versión>.tar.gz`

Para cada objetivo se instala:

```text
$ANYGAME_HOME/
├── runtimes/<versión>/<objetivo>/
│   ├── bin/relinker[.exe]     # binario del host actual
│   ├── libs/                  # librerías del objetivo
│   ├── docs/                  # documentación del release
│   └── runtime.json           # manifiesto con SHA-256
├── cache/downloads/           # assets verificados, reutilizables
└── state.json                 # versión activa por objetivo
```

- El binario del relinker es el del **host**; las librerías son las del
  **objetivo**. Así un AnyGame en Linux puede producir paquetes Windows con
  `--windows`, y viceversa.
- La descarga se verifica contra el campo `digest` (`sha256:…`) del release.
- La instalación se hace en un directorio temporal y se renombra al final; un
  fallo no deja una versión a medias.
- `state.json` recuerda la versión activa por objetivo; `build --version` permite
  fijar otra sin cambiar la activa.

## Desenvoltura SELF

`internal/self` es un port fiel de `tools/unwrap_self.py` de AnyPS5. Cubre:

- Tabla de registros SELF (`0x40`, entradas de `0x20` bytes).
- Recolocación de segmentos con `p_offset % p_align == p_vaddr % p_align`.
- `PT_PHDR`, `PT_DYNAMIC`, `PT_INTERP`, `PT_TLS`, notas y cola sin dirección.
- Relleno de ceros para TLS que excede la parte respaldada por fichero.
- Límite de padding (64 MiB) y de salida.
- Rechazo de contenedores cifrados por entropía (7,5 bits/byte).

La suite incluye un contenedor SELF sintético y una comprobación manual contra el
script original: la salida es **idéntica byte a byte**.

## Aplicación de escritorio

```text
┌─────────────────────────────────────────────────────────────┐
│ Renderer (HTML/CSS/JS, sin framework)                       │
│  biblioteca · ficha · conversión · runtimes · ajustes       │
└───────────────┬─────────────────────────────────────────────┘
                │ contextBridge (IPC)
┌───────────────▼─────────────────────────────────────────────┐
│ Electron main (ui/main.js)                                  │
│  ventana · diálogos · library.json · procesos · ajustes     │
└───────────────┬─────────────────────────────────────────────┘
                │ spawn (sin shell)
┌───────────────▼─────────────────────────────────────────────┐
│ bin/anygame[.exe]  (el mismo CLI del proyecto)              │
│  info · build · run · update · runtimes · doctor           │
└─────────────────────────────────────────────────────────────┘
```

- **Sin bundler ni npm**: el renderer es HTML/CSS/JS servido desde `file://` y
  `preload.js` expone una API mínima con `contextIsolation` y `sandbox`.
- **Sin `shell: true`**: cada acción construye un array de argumentos para el
  CLI; los datos del usuario nunca se interpolan en una línea de comandos.
- **Estado compartido**: `library.json` en `ANYGAME_HOME`; cada entrada guarda
  ruta del título, ruta del paquete, target, icono y fechas. La CLI no necesita
  conocer la biblioteca.
- **Progreso**: `build` y `update` se lanzan como procesos hijos y su
  stdout/stderr se reenvía al modal de log; se puede cancelar (SIGTERM y, tras
  4 s, SIGKILL).
- **Empaquetado**: `tools/package` descarga Electron, compila el sidecar para
  Linux y Windows, copia `ui/` en `resources/app` y genera el AppDir + AppImage
  o el ZIP Windows. El AppImage usa `--no-sandbox` (sandbox de Chromium no
  disponible tras el montaje) y CSP sin conexiones de red.

## Decisiones

- **Go y cero dependencias externas**: un único binario, compilación cruzada
  trivial y `debug/elf`/`net/http`/`archive/tar` en la biblioteca estándar.
- **El relinker se ejecuta como proceso**, no se enlaza: AnyPS5 sigue siendo la
  fuente de verdad y se puede actualizar sin recompilar AnyGame.
- **Todo el estado es inspeccionable**: JSON legible y un log de conversión por
  paquete.
- **Nada de `/tmp`**: los temporales de conversión viven dentro del directorio de
  salida (`.anygame-work`) y se eliminan al terminar, salvo `--keep-work`.

## No objetivos (por ahora)

- Descifrar juegos o extraer PKG.
- Reimplementar librerías de PS5.
- Prometer compatibilidad: el estado por juego lo mantiene AnyPS5 en sus
  discusiones de *Game bring-up*.
