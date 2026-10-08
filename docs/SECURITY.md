# SECURITY.md

Security model, trust boundaries, and hardening guarantees for **devsec**.

## 1. What devsec is

`devsec` is a cross-platform (macOS, Windows, Linux) environment provisioner and
system-hardening tool. It reads declarative YAML/TOML profiles, builds an ordered
plan of steps, and executes it idempotently using each OS's native package
manager and configuration mechanisms. It also applies OS hardening (audit by
default, apply opt-in).

It is a legitimate administration tool: it installs and configures publicly
available security tooling and hardens systems. It contains no exploit code,
payloads, or offensive tooling of its own.

## 2. Trust model

- **Profiles are a trust boundary.** A profile is executable intent: it causes
  packages to be installed (sometimes via `sudo`) and hardening to be applied.
  Only run profiles you author or have reviewed, the same way you would treat a
  `Makefile` that runs `sudo`.
- **Package and rule names are validated at parse time.** Names must match
  `^[A-Za-z0-9][A-Za-z0-9+_.:=@-]*$`. Option-like names (leading `-`), whitespace,
  path separators, and shell metacharacters are rejected before any command is
  built. This blocks argument injection into
  `apt`/`dnf`/`pacman`/`winget`/`scoop`/`choco`/`brew`.
- **No shell is involved.** Every command runs through a single `Runner`
  (`internal/engine/runner.go`) using `exec.CommandContext(ctx, name, args...)`
  with argument slices. There is no `sh -c` interpolation of user or profile
  data, so shell injection is structurally impossible in the current code.
- **Command output is sanitized.** The real `Runner` strips ANSI escape
  sequences and control characters from command output before it reaches logs,
  the audit store, or the TUI, preventing terminal/log injection from a hostile
  package repository.
- **The user is always part of the boundary.** `devsec` must not be copied into
  `/usr/bin` or run with elevated privileges passively; see section 4.

## 3. Privilege handling

- **Never run the whole TUI or the process as root.** When interactive, devsec
  pre-authenticates with `sudo -v` before the TUI starts, and each privileged
  step is wrapped with `sudo` individually at the command level.
- **Elevation is explicit, not silent.** A step reports `NeedsPrivilege()`;
  the plan header shows how many steps require elevation; the executor logs a
  warning when a plan that needs elevation is run non-elevated. No credentials
  are cached or stored beyond the OS-level `sudo` timestamp.
- **Windows elevation is detection-only for now.** `IsElevated()` on Windows is
  conservative and never assumes admin rights. `runas` self-elevation with a
  named pipe for results (as described in `docs/AGENTS.md`) is planned but not
  implemented; until then, run elevated Windows shells explicitly.
- **`sudo`/`sshd`/`ufw` are resolved from `PATH`.** This is appropriate for a
  CLI but means a hostile `PATH` could substitute binaries. Do not deploy
  devsec in environments where `PATH` is not trusted (e.g. as a setuid helper).

## 4. Hardening rules safety model

Hardening steps can lock a user out of their machine, so they follow strict
rules (mirrored in `docs/AGENTS.md`):

1. **Audit is the default; apply is opt-in.** `devsec audit` only reports.
   Applying hardening requires an explicit run plus confirmation (or `--yes`).
2. **Drop-in files, not edits.** Changes go to `/etc/ssh/sshd_config.d/`,
   `/etc/sysctl.d/`, etc. devsec refuses to rewrite a file it did not create.
3. **Symlink-safe writes.** Drop-in rules refuse symlink targets before and
   after writing, and rollback also refuses symlinks. A root-run rule can never
   write through a symlink.
4. **Validate before apply, roll back on failure.** e.g. `sshd -t` runs after
   writing the drop-in; on failure the change is rolled back from the backup.
5. **Remote-session guard.** Rules that can cut off remote access (firewall,
   sshd password-auth, remote login) refuse to apply when an SSH session is
   detected. This detection is env-var best-effort; treat it as a guardrail,
   not a guarantee. The local-console firewall rule additionally runs
   `ufw allow OpenSSH` before `ufw --force enable` so enabling the firewall
   never severs remote SSH for other users.
6. **Never harden the developer's own machine during development.** Tests must
   not modify the host; integration runs happen in containers/VMs or in
   dry-run/audit mode only.

## 5. Data the tool keeps

- The only persistent data is an **append-only audit log** at
  `~/.config/devsec/state.json` (overridable with `--state`). It records step
  IDs, states, apply outcomes, durations, and error strings. It is created
  unreadable by other users (dir `0700`, file `0600`).
- The state file is **not a source of truth**. Idempotency comes from
  `Step.Check()` probing the real system; the state file exists for audit and
  reporting only.
- No telemetry, no startup network calls, no auto-update.

## 6. Dependency integrity

- All runtime dependencies are pinned in `go.mod`/`go.sum`, and `go.sum` has
  been fully cross-checked against the official checksum database
  (`GOSUMDB=sum.golang.org go mod tidy` + `go mod verify`). The earlier caveat
  about `charmbracelet/harmonica` being hashed from a locally built zip no
  longer applies: the official module hash now supersedes it.
- Before a release, re-run `go mod tidy` and `go mod verify` on a stable
  network and review any hash changes:

  ```bash
  GOSUMDB=sum.golang.org go mod tidy
  go mod verify
  ```

  and confirm the recorded hashes match the official checksum database.
- Review new dependencies before adding them; prefer the standard library.

## 7. Reporting vulnerabilities

- **Do not open a public issue for a vulnerability.**
- Report privately to the maintainers by email/private channel until a fix and
  advisory are ready.
- Include: affected version(s), platform, a minimal profile or command that
  reproduces the issue, and expected vs. observed behavior.
- This project has no bug bounty; reports are handled best-effort.

## 8. Scope of this document

This is a living document covering the current codebase. Changes that affect
the trust model (new ways to run commands, elevation changes, new persistent
data, new hardening rules) must update this file and `docs/AGENTS.md`, and add a
security note to `docs/CHANGELOG.md`.

## Definition of "done" security checks

- `gofmt -l .` outputs nothing
- `go vet ./...` is clean
- `go test ./...` and `go test -race ./...` pass without root and without
  touching the host
- Cross-compiles pass for linux, darwin, and windows
- No direct `os/exec` usage outside the Runner