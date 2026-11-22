# Patch catalog

AnyGame reads catalogs in the format documented by AnyPS5:

- `index.json` with `format: 1` and an `entries` array.
- Each entry has `titleId`, `contentVersion`, a relative `patch` path and
  `targets` mapping every patched file to its SHA-256.
- Patch files are AnyPS5 TOML (`format = 1`, `[[patch]]` entries).

Point **Settings → Patch catalog** at:

- an `index.json` URL or a directory URL ending in `/`,
- a GitHub repository URL (`https://github.com/owner/repo`) or a tree URL,
- a local directory containing `index.json`.

This bundled catalog is empty. AnyGame never applies a patch whose target
hashes do not match the dump, and the relinker validates every byte range
again during conversion (`--patches`). Review community patches before use.
