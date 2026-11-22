# AnyGame

Capa de producto sobre [AnyPS5](https://github.com/boykopovar/AnyPS5): descubre un
juego de PS5 extraído, desenvuelve su `eboot.bin` si hace falta, lo convierte con
el relinker oficial de AnyPS5 y ensambla un paquete listo para ejecutar en Linux o
Windows. Incluye **CLI** y **aplicación de escritorio** con biblioteca, iconos,
estado de conversión, runtimes y ajustes.

> **Pensada para copias de juegos personales y legales.** AnyGame no incluye
> juegos, datos, imágenes, firmware ni claves, y no descifra ni elude
> protecciones. Usa solo copias que poseas legalmente.
>
> ¿Te resulta útil? Puedes apoyar el proyecto en
> **[ko-fi.com/tuxarchi](https://ko-fi.com/tuxarchi)**.

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
| Login de GitHub en la app (Device Flow + PAT) | ✅ |
| Ajustes de rendimiento por juego (resolución, FPS, FSR, present mode…) | ✅ |
| Empaquetado: AppImage (Linux) y ZIP con `.exe` (Windows) | ✅ |
| Lotes, sumisión de parches in-app y paridad Windows completa | ⏳ |

## Descargas

Los paquetes están en [Releases](https://github.com/TuxArchi/AnyGame/releases):

- `AnyGame-0.1.0-x86_64.AppImage` (Linux)
- `AnyGame-0.1.0-windows-x64.zip` (Windows)
- `SHA256SUMS`

## Apoya AnyGame

AnyGame se mantiene en mi tiempo libre. Si te resulta útil y quieres apoyar el
desarrollo, puedes invitarme a un café:

**https://ko-fi.com/tuxarchi**

No es obligatorio: los informes reproducibles y las contribuciones de parches
también ayudan mucho.

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

## Ajustes por juego

La ficha del juego (clic derecho) incluye **Game settings**:

- límite de FPS (`ANYPS5_FPS_LIMIT`);
- resolución reportada (`APS5_OUTPUT_RESOLUTION`);
- upscaling bilineal/FSR y nitidez (`ANYPS5_UPSCALE`, `ANYPS5_FSR_SHARPNESS`);
- modo de presentación (`ANYPS5_PRESENT_MODE`);
- caché de texturas (`APS5_TEXTURE_CACHE_MIB`);
- filtro de GPU (`ANYPS5_GPU`);
- fullscreen (`ANYPS5_FULLSCREEN`).

Se guardan por juego en `library.json` (`launchProfile`) y se aplican al
**Play** y al **Diagnose** desde la app, sin recompilar. No modifican el juego:
la resolución interna y los FPS reales siguen dependiendo de un **parche** y de
reconvertir. Los campos vacíos heredan el ajuste global.

## Login de GitHub

**Community → Sign in with GitHub** abre el Device Flow: la app muestra un
código, lo pegas en `github.com/login/device` y autorizas. El token se guarda
cifrado con el llavero del sistema y nunca llega al renderer.

Para habilitar el Device Flow crea una OAuth App:

1. `https://github.com/settings/applications/new`
2. Nombre `AnyGame`; Homepage la URL del repo de parches.
3. Callback `https://github.com` (Device Flow no la usa).
4. Activa **Enable Device Flow**.
5. Copia el **Client ID** y pégalo en **Settings → GitHub OAuth client ID**
   o expórtalo como `ANYGAME_GITHUB_CLIENT_ID`.

Scope solicitado: `public_repo`, el mínimo de OAuth para abrir PRs en un repo
público. Alternativa sin OAuth App: un **PAT fine-grained** con
*Contents: read/write* y *Pull requests: read/write* sobre el repo del catálogo,
pegado en Community. El token se guarda en `github-auth.json` (0600) cifrado
cuando el sistema lo permite.

La publicación automática del PR se implementará sobre esta sesión; hoy el
login ya está operativo y el token queda disponible en el proceso principal.

## Variables de entorno

| Variable | Efecto |
|---|---|
| `ANYGAME_HOME` | Directorio de datos (runtimes, caché, estado). |
| `GITHUB_TOKEN` | Sube el límite de la API de GitHub. |
| `ANYGAME_PATCH_CATALOG` | Fuerza la fuente del catálogo de parches. |
| `ANYGAME_GITHUB_CLIENT_ID` | Client ID público de la OAuth App para Device Flow. |

## Licencia y aviso legal

AnyGame se distribuye bajo **GPL-2.0-only**. No está afiliado, autorizado ni
respaldado por Sony Interactive Entertainment. No incluye ni distribuye código o
datos de juegos, imágenes, saves, firmware, claves ni librerías propietarias.
AnyPS5 se descarga de sus releases oficiales en tiempo de ejecución.

- [Aviso legal completo](DISCLAIMER.md)
- [Privacidad](PRIVACY.md)
- [Terceros](THIRD_PARTY_NOTICES.md)

Usa AnyGame solo con copias de juegos que **poseas legalmente** y donde tu ley
permita copias personales, interoperabilidad y preservación. Eres responsable
del contenido que conviertas y de los parches que apliques.
