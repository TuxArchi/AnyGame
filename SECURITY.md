# Security policy

## Supported versions

Only the latest AnyGame release receives fixes.

## Reporting a vulnerability

Open a private security advisory in the repository or contact the maintainer
through the address listed in the repository profile. Do not attach game
files, saves, keys, firmware or personal tokens.

Include:

- AnyGame and AnyPS5 versions, OS and architecture;
- the command or UI action that triggers the issue;
- the smallest synthetic reproduction you can provide.

## Scope

AnyGame is a launcher and conversion front end. Vulnerabilities in AnyPS5
itself should be reported to the AnyPS5 project. Community patches are
third-party content: report malicious patches to the catalog maintainers and
include the entry ID and target hashes.

## Tokens

The GitHub token stays in the main process, encrypted with the OS keyring when
available. Sign out from **Community** to delete it. Never paste a token into an
issue, log or patch submission.
