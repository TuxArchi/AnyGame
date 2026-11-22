# Privacy

AnyGame is local-first. It has **no telemetry, analytics, accounts or uploads**
of game content.

## Data kept on your machine

- `ANYGAME_HOME` (default `~/.local/share/AnyGame` on Linux,
  `%LOCALAPPDATA%\AnyGame` on Windows): library, settings, downloaded AnyPS5
  runtimes, patch-catalog cache and diagnostic logs.
- `application settings` under the Electron user-data directory.
- `github-auth.json`: a GitHub token, encrypted with the OS keyring
  (`safeStorage`) when available, otherwise a file with mode `0600`.
- Converted packages in the folder you choose, including `app0/`, `libs/`,
  `anygame.json`, `anygame-report.json`/`anygame-run.log` and a shader cache.

## Network requests

AnyGame contacts:

- **GitHub** to check and download AnyPS5 releases, to read a patch catalog and,
  only after you sign in, to create a fork/branch/Pull Request.
- The **patch catalog** URL you configure.

No game files, saves, paths or diagnostic logs are uploaded. The GitHub token is
sent only to `github.com`/`api.github.com` as the authorization header.

## Ko-fi

The support link opens `https://ko-fi.com/tuxarchi` in your browser. Ko-fi's own
privacy policy applies there; AnyGame sends nothing to it.

## Removing data

Delete the `ANYGAME_HOME` directory and `github-auth.json` to remove local data.
A converted package is self-contained; delete its folder to remove it.
