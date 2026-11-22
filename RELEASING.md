# Releasing

## Checklist

- [ ] `make check` passes.
- [ ] `git status` is clean; no game dumps, images, saves, logs, tokens or
      personal paths are tracked.
- [ ] `DISCLAIMER.md`, `PRIVACY.md`, `THIRD_PARTY_NOTICES.md` and `LICENSE`
      are present.
- [ ] The release notes name only personal, legally owned copies.
- [ ] `go run ./tools/package -version <version>` builds both packages.
- [ ] `dist/SHA256SUMS` matches the uploaded files.
- [ ] The AppImage does not contain AnyPS5 binaries or game content.

## Automatic release

```sh
git tag -a v0.1.0 -m "AnyGame v0.1.0"
git push origin main
git push origin v0.1.0
```

`Release` builds the AppImage and Windows ZIP on Ubuntu, writes
`SHA256SUMS` and publishes them with `docs/releases/v0.1.0.md`.

## Manual release

```sh
make check
go run ./tools/package -version 0.1.0
(cd dist && sha256sum AnyGame-*.AppImage AnyGame-*.zip > SHA256SUMS)
gh release create v0.1.0 \
  --title "AnyGame v0.1.0" \
  --notes-file docs/releases/v0.1.0.md \
  dist/AnyGame-0.1.0-x86_64.AppImage \
  dist/AnyGame-0.1.0-windows-x64.zip \
  dist/SHA256SUMS
```

Never upload game files, screenshots containing game art, saves, keys or
firmware. The release assets are the two packages and the checksum file.
