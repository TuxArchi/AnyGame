# AnyGame

Capa de producto sobre [AnyPS5](https://github.com/boykopovar/AnyPS5): descubre un
juego de PS5 extraído, desenvuelve su `eboot.bin` si hace falta, lo convierte con
el relinker oficial de AnyPS5 y ensambla un paquete listo para ejecutar en Linux o
Windows. Incluye **CLI** y **aplicación de escritorio** con biblioteca, portadas,
estado de conversión, runtimes y ajustes.

AnyGame **no reimplementa** el runtime: usa los binarios y las librerías de los
releases oficiales de AnyPS5, y añade lo que falta para trabajar con juegos
completos: actualizador con verificación SHA-256, detección de títulos,
desenvoltura de SELF, disposición del layout de ejecución, lanzadores y metadatos.

## Estado

| Área | Estado |
|---|---|
| Actualizador de AnyPS5 (Linux/Windows, SHA-256, versiones) | ✅ |
| Descubrimiento de título (`eboot.bin`, `app0/`, módulos, `param.json`/`param.sfo`) | ✅ |
| Desenvoltura automática de módulos SELF y plugins (`Media/Modules`, `Media/Plugins`) | ✅ |
| Desenvoltura SELF/fSELF (port en Go, idéntico byte a byte a `unwrap_self.py`) | ✅ |
| Conversión con el relinker oficial | ✅ |
| Paquete Linux (ELF, `libs/`, `app0/`, `run.sh`, `.desktop`) | ✅ |
| Paquete Windows (PE, `libs/`, `app0/`, `run.bat`) | ✅ |
| Copia selectiva de librerías según `.needed.json` + dependencias | ✅ |
| Cierre de dependencias de módulos convertidos y poda de `DT_NEEDED` sin imports | ✅ |
| App de escritorio (biblioteca, conversión, runtimes, ajustes) | ✅ |
| Verificación estática del paquete (`verify`) y prueba de arranque (`diagnose`) | ✅ |
| Catálogo de parches AnyPS5 (TOML) en la ficha del juego | ✅ |
| Empaquetado: AppImage (Linux) y ZIP con `.exe` (Windows) | ✅ |
| Lotes, catálogo de parches, perfiles por juego | ⏳ |

## Requisitos

- Linux o Windows **x86-64** (es la arquitectura a la que se recompilan los
  títulos; un host ARM no sirve).
- Go 1.22 o superior para compilar AnyGame.
- GPU con Vulkan 1.1 y driver actualizado para **ejecutar** los juegos.
- Un volcado **descifrado** del juego: AnyPS5 no descifra contenedores.

## Compilar

```sh
make build          # bin/anygame (CLI)
make check          # formato, vet y tests
make dist           # CLI Linux y Windows en dist/
make ui             # escritorio: AppImage + ZIP Windows en dist/
make smoke          # prueba de extremo a extremo (requiere gcc y red)
```

`make ui` descarga Electron y `appimagetool` la primera vez (unos 300 MB) en
`.work-tmp/ui-downloads` y genera:

```text
dist/AnyGame-<versión>-x86_64.AppImage
dist/AnyGame-<versión>-windows-x64.zip
```

El AppImage se ejecuta con `--no-sandbox` desde su `AppRun` porque el sandbox de
Chromium no funciona dentro de una imagen montada sin privilegios; la app solo
carga contenido local y bloquea conexiones de red desde la interfaz.

## Uso rápido (CLI)

```sh
# 1. Comprobar el host
anygame doctor

# 2. Instalar el runtime de AnyPS5 (última versión; Linux y Windows)
anygame update --target all

# 3. Inspeccionar el juego
anygame info /ruta/JuegoPS5

# 4. Convertir y empaquetar
anygame build /ruta/JuegoPS5 -o ./juegos/PPSA01234

# 5. Ejecutar
anygame run ./juegos/PPSA01234
```

`anygame update --check` avisa de versiones nuevas sin descargar nada.
`anygame update --version 0.1.5 --target linux` fija una versión concreta.

## Aplicación de escritorio

Abre el AppImage o `AnyGame.exe` y trabaja desde la biblioteca:

1. **Runtimes** → *Install latest* (Linux + Windows) con progreso en vivo.
2. **Games** → *Add game* y elige la carpeta extraída del título.
3. Pulsa una portada → *Convert* (plataforma, carpeta de salida, recursos,
   parches) y sigue el log de conversión dentro de la app.
4. *Play* lanza el juego; *Rebuild* lo reconvierte; *Test* ejecuta un arranque
   de 15 s y analiza el log; *Verify* comprueba el paquete.
5. La conversión puede lanzar la prueba automáticamente (casilla *Test after
   conversion*).

La app y la CLI comparten `library.json`, runtimes y ajustes bajo
`~/.local/share/AnyGame` (Linux) o `%LOCALAPPDATA%\AnyGame` (Windows).

## Layout generado

```text
PPSA01234/
├── PPSA01234.elf          # o PPSA01234.exe en Windows
├── libs/                  # librerías del sistema AnyPS5 necesarias
├── app0/                  # recursos del juego + eboot.bin original
│   ├── eboot.bin
│   ├── sce_sys/
│   └── sce_module/        # módulos ya convertidos por el relinker
├── run.sh                 # o run.bat
├── <Título>.desktop       # Linux
└── anygame.json           # manifiesto de la conversión
```

## Parches de la comunidad

La ficha del juego (clic izquierdo o derecho) incluye **Community patches**:

1. Configura **Settings → Patch catalog** con una URL a `index.json`, una URL de
   directorio, un repositorio de GitHub (`https://github.com/owner/repo`) o una
   carpeta local con `index.json`.
2. La app filtra por `titleId` + `contentVersion`, comprueba el SHA-256 de cada
   fichero objetivo y marca el parche como **compatible** o incompatible.
3. Selecciona **un** parche (AnyPS5 no combina parches) y aparecerá en
   *Convert*. El relinker lo aplica con `--patches` y vuelve a validar cada byte.

El catálogo incluido en `ui/patches/` está vacío: no se distribuyen parches de
terceros. `ANYGAME_PATCH_CATALOG` permite fijar la fuente sin tocar Ajustes.

Formato: `index.json` con `format: 1`, entradas con `titleId`,
`contentVersion`, `patch` y `targets` (SHA-256), y parches TOML de AnyPS5. Las
descargas se guardan en `ANYGAME_HOME/cache/patch-catalog`.

## Variables de entorno

| Variable | Efecto |
|---|---|
| `ANYGAME_HOME` | Directorio de datos (runtimes, caché, estado). |
| `GITHUB_TOKEN` | Sube el límite de la API de GitHub. |
| `ANYGAME_PATCH_CATALOG` | Fuerza la fuente del catálogo de parches. |

## Licencia y aviso legal

AnyGame se distribuye bajo GPL-2.0, la misma licencia que AnyPS5. No incluye ni
distribuye software de Sony, firmware, claves ni juegos. Los binarios de AnyPS5
se descargan de sus releases oficiales. Usa el proyecto solo con material que
poseas legalmente.
