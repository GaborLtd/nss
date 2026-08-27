# Changelog

Important changes are recorded here. Versions follow Semantic Versioning.

## [Unreleased]

## [0.2.1] - 2026-08-27

- Added `nss update` and `nssd update`, which verify checksums and update both binaries.
- Added `nssd service install|status|restart|uninstall` for macOS LaunchAgents and Linux systemd user services.
- Rebuilt the release tag after stale queued runs from a GitHub Actions incident.

## [0.1.4] - 2026-08-26

- `nssd serve` prints a ready message after initialization so the Unix socket and settings can be verified.
- Added operational documentation for verifying the daemon with `nssd list`.

- Bootstrapped the Native Session Shell (`nss`) project.
- Added architecture, session lifecycle, protocol, and release documentation.
- Added the Go scaffold, GitHub Actions CI, GoReleaser, and checksum-verifying installer.
- Completed the first MVP vertical slice: PTY sessions, an SSH attach proxy, reconnecting client, bounded spool, and terminal resize.
