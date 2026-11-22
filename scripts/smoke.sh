#!/usr/bin/env bash
# End-to-end smoke test: builds AnyGame, prepares a synthetic title, converts it
# for Linux and Windows and checks the generated layout.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export ANYGAME_HOME="${ANYGAME_HOME:-$ROOT/.work-tmp/home}"
WORK="$ROOT/.work-tmp/smoke"
BIN="$WORK/anygame"
TITLE="$WORK/title"

rm -rf "$WORK"
mkdir -p "$TITLE/sce_sys" "$TITLE/data"

printf 'int main(void) { return 7; }\n' > "$WORK/title.c"
gcc -shared -nostdlib -fPIC -Wl,-e,main -o "$TITLE/eboot.elf" "$WORK/title.c"
printf 'smoke asset\n' > "$TITLE/data/asset.txt"
cat > "$TITLE/sce_sys/param.json" <<'JSON'
{"titleId":"PPSA00001","contentVersion":"01.000.000","defaultLanguage":"en-US","localizedParameters":{"en-US":{"titleName":"AnyGame Smoke"}}}
JSON

cd "$ROOT"
go build -trimpath -o "$BIN" ./cmd/anygame

"$BIN" version
"$BIN" doctor || true

HOST="$(go env GOOS)"
"$BIN" update --target "$HOST" >/dev/null
"$BIN" update --target windows >/dev/null

"$BIN" info "$TITLE"

"$BIN" build "$TITLE" --skip-modules -o "$WORK/out-linux" >/dev/null
for file in PPSA00001.elf anygame.json run.sh app0/eboot.bin app0/data/asset.txt; do
  test -e "$WORK/out-linux/$file" || { echo "missing $file"; exit 1; }
done
test -x "$WORK/out-linux/PPSA00001.elf"

"$BIN" build "$TITLE" --skip-modules --target windows -o "$WORK/out-windows" >/dev/null
for file in PPSA00001.exe anygame.json run.bat app0/eboot.bin; do
  test -e "$WORK/out-windows/$file" || { echo "missing $file"; exit 1; }
done

echo "smoke test passed"
