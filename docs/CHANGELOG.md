# Changelog

All notable changes to **devsec** are documented here. Format follows Keep a
Changelog; versioning is 0.x until the public contract stabilizes.

## [Unreleased]

### Added
- Cobra CLI (`devsec run`, `devsec audit`) with `--profile`, `--dry-run`,
  `--yes`, `--non-interactive`, `--state`, and `--log-level` flags.
- Bubbletea TUI: plan review, streaming progress bar, and per-step log viewer
  (`ui/`). Interactive runs pre-authenticate with `sudo -v` before starting so
  the TUI is never run as root.
- Engine contracts in `internal/engine`: `Step`, `State`, `Runner`
  (`ExecRunner`/`FakeRunner`), `BuildPlan`, `Executor`, NDJSON audit store.
- Translators for `brew`, `apt`, `dnf`, `pacman`, `winget`, `scoop`, and
  `choco`, plus per-manager package-name mapping tables (`internal/engine/mapping`).
- Hardening rules (`internal/hardening`): sysctl drop-in, sshd password-auth
  drop-in with `sshd -t` validation and rollback, UFW firewall, macOS
  Gatekeeper and Remote Login, Windows SMBv1 and PowerShell execution policy.
  Rules are registered per-OS; unknown rules are rejected, known-but-unsupported
  rules are skipped so cross-OS profiles work everywhere.
- Embedded profiles: `dev`, `rev-eng`, `pentest`, `hardening`.

### Changed
- Repository layout moved the entrypoint to `cmd/devsec/` and removed the empty
  `pkg/` directory.

### Fixed
- Terminology corrections: reverse-engineering section lists Rizin/Cutter and
  iaito (not "Rizal"); macOS auditing is OpenBSM, not auditd.
- `--profile <name>` now prefers an existing file with that name over the
  embedded profile of the same name; previously a local `dev.yaml` was silently
  shadowed by the embedded one.
- Restored the missing `charmbracelet/harmonica` entry in `go.mod`/`go.sum`
  (a `go build`/`go vet` breaker); the checksum DB now governs all module hashes.
- TUI: the run no longer panics ("slice bounds out of range") on tiny or
  zero-height terminals — the log viewport height is clamped.
- TUI: the progress bar now animates to the real completion percentage
  (its frame command was being discarded and `FrameMsg` was not forwarded,
  leaving it frozen at 0%); views are sized so the bar stays inside the
  terminal's painted window even on the summary screen.
- TUI: the completion summary now stays on screen until dismissed instead of
  quitting before it could render.

### Security
- Package names in profiles are validated (`^[A-Za-z0-9][A-Za-z0-9+_.:=@-]*$`);
  option-like or metacharacter names that could be injected into
  apt/dnf/pacman/winget/scoop/choco/brew argv are rejected at parse time.
- Hardening drop-in writes refuse symlink targets (pre- and post-write) so a
  root-run rule can never write through a symlink; failed rollbacks also refuse
  symlinks.
- The ufw rule now runs `ufw allow OpenSSH` before `ufw --force enable` so
  enabling the firewall from a console never severs remote SSH access; it still
  refuses over an active SSH session.
- Command output from the real Runner is sanitized of ANSI escape sequences and
  control characters before reaching logs, the audit store, or the TUI.
- The audit store (`state.json`) is now created world-unreadable (dir 0700,
  file 0600).

## [0.0.1] - unreleased
- Project skeleton: Go module, agent docs, and `devsec run` dry-run with the
  brew translator and the dev profile.