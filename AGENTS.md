# Native Session Shell (nss) Project Guidelines

## Project scope

Native Session Shell (`nss`) is a remote development terminal tool built on standard OpenSSH.

- `nss`: the laptop/mobile CLI and reconnect orchestrator.
- `nssd`: the server-side session daemon responsible for PTYs, shell processes, and session lifecycle.
- SSH, VPN, LAN, Cloudflare Tunnel, and other connection methods are transport layers and must not become dependencies of the `nss` core.
- One native terminal tab maps to one independent remote PTY session.
- Do not add tmux-style multiple windows, panes, prefix hotkeys, or an embedded terminal UI.

## Language and documentation

- Primary user-facing documentation is written in English so that non-Chinese users can use the project.
- Keep Traditional Chinese translations under `README.zh-TW.md` and `docs/zh-TW/` when the corresponding English document changes.
- Technical terms may remain in English; explain them in plain language when useful.
- Code comments remain in Traditional Chinese unless the project explicitly changes that convention.
- Document every public behavior, CLI flag, configuration, protocol, and lifecycle change.
- Keep the README focused on onboarding and stable promises; put detailed design in `docs/`.

## Feature development rules

Every feature must include:

1. An implementation.
2. Automated tests.
3. User or developer documentation.
4. CI validation.

Do not consider a feature complete without tests and documentation. If unit testing is not suitable, provide an integration test, a reproducible manual test procedure, or a clear statement of the current limitation.

Priorities:

1. Define behavior and failure cases first.
2. Write tests before implementation; if that is not practical, add the tests in the same change.
3. Keep the protocol versionable and backward-compatible.
4. Test network loss, process crashes, duplicate attach, session cleanup, and disk quotas.

## Testing and quality gates

At minimum, run locally:

```bash
gofmt -w .
go vet ./...
go test -race -cover ./...
go build ./cmd/nss ./cmd/nssd
```

CI must run the same primary checks on pull requests and pushes to the main branch. Add new lint, static analysis, or integration checks to CI rather than relying only on developer machines.

Tests must not depend on a production server, real user credentials, or undeclared external services. Use test fixtures, fake transports, temporary directories, or a controllable local test server for SSH, PTY, and filesystem behavior.

## Security rules

- Never print SSH private keys, session secrets, or complete user input in logs, errors, test artifacts, or documentation.
- Session secrets must use unpredictable random values; a guessable session ID is not sufficient.
- Spool files must use least-privilege permissions and be limited by per-session and global disk quotas.
- Never automatically replay arbitrary raw keyboard input accumulated while disconnected.
- Do not disable host-key verification or authentication checks for testing convenience.
- GitHub Actions must use the minimum required `GITHUB_TOKEN` permissions; only the release job may use `contents: write`.

## Release rules

- Use Semantic Versioning with tags in the form `vMAJOR.MINOR.PATCH`.
- Create a GitHub Release only from a tag that has passed CI.
- Use GoReleaser to produce macOS/Linux `amd64` and `arm64` artifacts.
- Every release must contain `nss`, `nssd`, checksums, and reproducible version information.
- The installer may download only GitHub Release assets and must verify SHA-256 before installation.
- Document breaking protocol or configuration changes in release notes and migration documentation.

## Change checklist

Before a commit or pull request, verify:

- [ ] Feature behavior is documented in `README.md` or `docs/`.
- [ ] Unit tests or suitable integration/manual tests exist.
- [ ] `gofmt`, `go vet`, `go test -race`, and builds pass.
- [ ] Transport-specific logic is not mixed into the session core.
- [ ] Memory, disk spool, retry, and goroutine growth are bounded.
- [ ] CLI, protocol, session lifecycle, and release documentation is updated when applicable.
