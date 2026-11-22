# Uso

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
`prx` junto al ejecutable y en la raíz del título.

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
