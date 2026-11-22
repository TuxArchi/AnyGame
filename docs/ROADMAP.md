# Hoja de ruta

## Fase 1 — Base (hecho)

- [x] CLI con subcomandos y ayuda.
- [x] Actualizador de AnyPS5: GitHub API, `--check`, `--list`, versiones,
      objetivos Linux/Windows, SHA-256, instalación atómica, estado activo.
- [x] Descubrimiento de título (`eboot`, `app0`, módulos, `param.json`/`param.sfo`).
- [x] Desenvoltura SELF/fSELF (port validado byte a byte contra el original).
- [x] Pipeline `build` con relinker oficial y layout `libs/` + `app0/`.
- [x] Copia selectiva de librerías por `needed.json` + cierre `DT_NEEDED`.
- [x] Lanzadores `run.sh` / `run.bat`, entrada `.desktop`, icono, manifiesto.
- [x] `doctor` con CPU, Vulkan y udmabuf.
- [x] Tests unitarios de SELF, SFO, descubrimiento, args y empaquetado.

## Fase 2 — Producto utilizable

- [ ] `build --batch <dir>`: convertir varios títulos con un informe resumen.
- [ ] Perfiles por título (`profiles/<titleId>.toml`): opciones de build,
      variables `ANYPS5_*`, parches y argumentos extra.
- [x] Catálogo de parches AnyPS5: filtro por título/versión, hashes SHA-256,
      selección en la ficha y `--patches` al convertir.
- [ ] `anygame verify <paquete>`: comprobar layout, librerías, `needed.json`,
      manifiesto y enlaces antes de lanzar.
- [ ] `anygame export <paquete> --zip`: paquete comprimido reproducible.
- [ ] Detección de GPU/Vulkan y recomendación de variables (`ANYPS5_GPU`,
      `APS5_OUTPUT_RESOLUTION`, `ANYPS5_UPSCALE`).
- [ ] Logs estructurados y `--verbose` en todos los comandos.

## Fase 3 — Interfaz (hecho)

- [x] App de escritorio (biblioteca, ficha, conversión con log, runtimes,
      ajustes y diagnóstico).
- [x] AppImage Linux y ZIP Windows con `.exe` y sidecar.
- [x] Empaquetado reproducible con Electron y `appimagetool`.
- [ ] Integración con Steam: añadir acceso directo no-Steam y portada.
- [ ] Bandeja del sistema y arranque minimizado.

## Fase 4 — Compatibilidad y calidad

- [ ] Matriz de pruebas con títulos homebrew y demos legales.
- [ ] Integración CI (Linux + Windows) con tests de release simulado.
- [ ] Firma de los binarios de AnyGame en Windows.
- [ ] Autoactualización de la app de escritorio.
- [ ] Empaquetado (AppImage/zip) y autoactualización de AnyGame.
- [ ] i18n (es/en) de la CLI.

## Riesgos conocidos

- El formato de los assets de AnyPS5 puede cambiar; el actualizador falla con un
  mensaje claro si no encuentra `relinker-*` o `prx-<objetivo>-*`.
- Los juegos requieren una GPU Vulkan 1.1 con extensiones concretas; cualquier
  promesa de compatibilidad debe venir de las pruebas de AnyPS5, no de AnyGame.
- El espacio en disco: `--resources copy` duplica el juego; el modo `link`
  depende del sistema de archivos.
