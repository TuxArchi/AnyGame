# Contributing

AnyGame is licensed under **GPL-2.0-only**. By contributing code you agree to
license it under the same terms.

## Rules

- No game code, game data, artwork, saves, keys, firmware or decrypted binaries.
- No personal paths, tokens or diagnostic logs in commits.
- Keep compatibility work general; do not special-case a single title.
- Add tests for behavior changes.
- Use Conventional Commits.

## Build and test

```sh
make check
make build
./scripts/smoke.sh
```

The smoke test downloads AnyPS5 runtimes and needs `gcc`. It writes only under
`.work-tmp/`.

## Patches

The community patch catalog is a separate repository and uses AnyPS5's TOML
format. Patch data is contributed under **CC0-1.0**; never include game content.
AnyGame validates SHA-256 target hashes, but a maintainer must test each patch.
