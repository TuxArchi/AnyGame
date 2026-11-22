# Uso

## Aplicación de escritorio

Linux:

```sh
chmod +x AnyGame-0.1.0-x86_64.AppImage
ANYGAME_HOME="$HOME/.local/share/AnyGame" ./AnyGame-0.1.0-x86_64.AppImage
```

Windows: descomprime el ZIP y ejecuta `AnyGame.exe`.

Flujo recomendado:

1. **Runtimes → Install latest** descarga las librerías de Linux y Windows.
   *Check updates* consulta el último release y *Reinstall* repara una instalación.
2. **Games → Add game** abre el selector de carpetas y ejecuta
   `anygame info` para leer el título.
3. Pulsa una portada para abrir la ficha: fuente, paquete, target, runtime,
   avisos y acciones.
4. **Convert** abre el formulario (target, salida, recursos, parches). El log de
   AnyPS5 se muestra en vivo y la entrada se actualiza al terminar.
5. **Play** lanza el paquete en segundo plano. **Rebuild** vuelve a convertir.

Ajustes:

- *Default target* y *Resources* se aplican a nuevas conversiones.
- *Output root* es la carpeta padre por defecto de los paquetes.
- *Data directory* cambia `ANYGAME_HOME` para la app y la CLI.
- *GPU filter* se pasa como `ANYPS5_GPU` al lanzar; *Fullscreen* como
  `ANYPS5_FULLSCREEN=1`.

La biblioteca vive en `<data>/library.json`; los runtimes, en
`<data>/runtimes/`. La CLI ve los mismos datos.

## Instalación del runtime

```sh
anygame update                    # host actual, última versión
anygame update --target all       # Linux y Windows
anygame update --target windows   # solo librerías Windows
anygame update --check            # ¿hay versión nueva?
anygame update --list             # releases recientes de AnyPS5
anygame update --version 0.1.5 --force
```

`update --check` e `update --list` aceptan `--json`.

## Inspeccionar un título

```sh
anygame info /juegos/PPSA01234
anygame info /juegos/PPSA01234 --json
```

AnyGame busca, en este orden:

1. `<ruta>/eboot.bin`
2. `<ruta>/eboot.elf`
3. `<ruta>/app0/eboot.bin`
4. `<ruta>/app0/eboot.elf`
5. `<ruta>/*eboot*.bin` o `*.elf`

Los metadatos salen de `sce_sys/param.json`; si no existe, de
`sce_sys/param.sfo`. Los módulos se buscan en `sce_module`, `sce_modules` y
`prx` junto al ejecutable y en la raíz del título. Los directorios
`Media/Modules` y `Media/Plugins` se detectan como módulos adicionales y se
pasan al relinker automáticamente.

Los módulos SELF se desenvuelven antes de convertir; no hace falta hacerlo a
mano. El paquete incluye también las dependencias de los módulos convertidos y
descarta entradas `DT_NEEDED` sin imports (por ejemplo, `UnityNpToolkit2.prx`).

Un volcado incompleto (por ejemplo, sin `Media/Metadata`) puede arrancar y fallar
al cargar los datos de Unity: es un problema del volcado, no del pipeline.

## Convertir y empaquetar

```sh
anygame build /juegos/PPSA01234 -o ./juegos/PPSA01234
```

Opciones:

| Opción | Efecto |
|---|---|
| `-o`, `--output` | Directorio del paquete (por defecto `./<titleId>`). |
| `--target linux\|windows` | Plataforma de salida (por defecto la del host). |
| `--version <v>` | Versión concreta de AnyPS5; no cambia la activa. |
| `--name <nombre>` | Nombre base del ejecutable. |
| `--patches <archivo>` | Parche TOML de AnyPS5. |
| `--no-patches` | No aplicar ni descubrir `relinker-patches.toml`. |
| `--resources link\|copy\|none` | Cómo poblar `app0/` (por defecto `link`). |
| `--skip-modules` | Título sin `sce_module` (homebrew). |
| `--module-dir <dir>` | Directorio de módulos adicional (repetible). |
| `--executable <ruta>` | Ejecutable adicional lanzado con `sceSystemServiceLoadExec` (repetible). |
| `--windows-gui` | PE con subsistema GUI (sin consola). |
| `--to-intel` | Convierte instrucciones exclusivas de AMD. |
| `--dry-run` | Planifica sin escribir el paquete. |
| `--keep-work` | Conserva `.anygame-work` para depurar. |
| `--json` | Resultado legible por máquina. |

### Ejemplos

Windows desde Linux:

```sh
anygame build /juegos/PPSA01234 --target windows -o ./win/PPSA01234
```

Título con plugins en `Media/Plugins` y un `picker`:

```sh
anygame build /juegos/PPSA01234 \
  -o ./juegos/PPSA01234 \
  --module-dir Media/Plugins \
  --executable picker/picker.bin
```

Sin duplicar 100 GB de recursos:

```sh
anygame build /juegos/PPSA01234 -o ./juegos/PPSA01234 --resources link
```

## Verificar y diagnosticar

```sh
anygame verify ./juegos/PPSA01234
anygame verify ./juegos/PPSA01234 --json

anygame diagnose ./juegos/PPSA01234 --seconds 15
anygame diagnose ./juegos/PPSA01234 --json
```

`verify` comprueba sin lanzar nada:

- `anygame.json`, formato ELF/PE, target y permisos.
- `libs/`, `run.sh`/`run.bat` y `app0/eboot.bin`.
- `DT_NEEDED` del ejecutable y de **cada módulo convertido**, resolviendo
  `$ORIGIN`, librerías de `libs/` y librerías del sistema.

`diagnose` ejecuta el juego durante N segundos (15 por defecto), lo detiene y
analiza `anygame-diagnose.log`: librerías que no cargan, ficheros que el juego
abre y no existen, funciones no implementadas, fallos fatales y red. El
resultado se guarda en `anygame.json` (`diagnosis`) y la ficha del juego lo
muestra.

`build --diagnose --diagnose-seconds 15` hace la prueba al terminar de empaquetar.

## Ejecutar

```sh
anygame run ./juegos/PPSA01234
anygame run ./juegos/PPSA01234 -- --alguna-opcion
```

`run` lee `anygame.json`, entra en el directorio del paquete y lanza el
ejecutable. También puedes usar `./run.sh` (Linux) o `run.bat` (Windows).

Variables de AnyPS5 útiles al ejecutar:

```sh
ANYPS5_GPU="RTX" anygame run ./juegos/PPSA01234
ANYPS5_FULLSCREEN=1 anygame run ./juegos/PPSA01234
```

## Desenvolver un SELF

```sh
anygame unwrap eboot.bin eboot.elf --verbose
```

El comando rechaza contenedores cifrados, igual que el script oficial.

## Diagnóstico

```sh
anygame doctor
anygame doctor --json
```

Comprueba plataforma, arquitectura, extensión de CPU (AVX2, FMA, F16C, BMI1/2,
LZCNT, MOVBE), runtime instalado, cargador Vulkan y `/dev/udmabuf`.

## Solución de problemas

**`no eboot found`** — la ruta debe contener el ejecutable o `app0/eboot.bin`.

**`every load segment looks encrypted`** — el `eboot.bin` está cifrado; AnyGame
no descifra. Necesitas un volcado descifrado.

**`has no sce_module/… directory`** — el relinker exige un directorio de módulos
salvo que uses `--skip-modules` (homebrew) o `--module-dir`.

**`some needed library is not in the AnyPS5 runtime`** — actualiza el runtime
(`anygame update --force`) o revisa que el objetivo sea el correcto.

**Falta `libvulkan` / el juego no arranca** — instala el driver Vulkan del
fabricante; `anygame doctor` lo detecta.

**`/dev/udmabuf` sin permisos** — añade el usuario al grupo `kvm` (o usa una ACL)
para que la memoria directa compartida se importe en vez de copiarse.

**Windows: error 126 al cargar un `.prx`** — faltan las DLL del toolchain en
`libs/`; AnyGame las copia automáticamente. Si usas un paquete antiguo,
reconviértelo.
