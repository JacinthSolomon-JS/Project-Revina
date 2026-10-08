# AGENTS.md

Guidance for AI coding agents working on **devsec-provisioner** (`devsec`), a cross-platform (macOS, Windows, Linux) environment provisioner and system hardening tool written in Go with a Bubbletea TUI.

## Project Overview

`devsec` acts as a meta-package manager plus a hardening tool. It reads declarative YAML/TOML profiles (dev, reverse engineering, pentesting, hardening), builds an ordered **plan** of steps, and executes it idempotently using each OS's native package manager and config mechanisms.

Core principles:

1. **Plan, then apply.** The engine builds a plan first. `--dry-run` prints the plan; the TUI shows it for confirmation. Nothing mutates the system during planning.
2. **Probe real state.** Idempotency comes from `Step.Check()` inspecting the actual system, never from trusting `state.json`. The state file is an audit log/cache only.
3. **Engine before UI.** The engine must work fully headless. The TUI is a thin layer on top and must never contain system-mutating logic.
4. **Safety first on hardening.** A bug here can lock a user out of their machine. Audit is the default; apply is opt-in.

## Tech Stack

- Go (see `go.mod` for the version), single static binary, cross-compiled for darwin/linux/windows
- TUI: `charmbracelet/bubbletea`, `bubbles`, `lipgloss`
- CLI flags: standard `flag` or `cobra` (keep to one; don't mix)
- Config: YAML (primary), TOML (optional), embedded defaults via `embed`

## Repository Layout

```
cmd/devsec/main.go          Entrypoint: flags, privilege detection, wiring only
internal/
  config/                   Profile/Task schema (schema.go) and loader/validator (loader.go)
  engine/
    plan.go                 Plan builder (profiles -> ordered/DAG steps)
    executor.go             Runs a plan: Check -> Apply, logging, error policy
    runner.go               Runner interface (all command execution goes through here)
    system.go               OS/distro detection, elevation helpers
    translators/            One file per package manager (brew, winget, scoop, apt, dnf, pacman, ...)
    mapping/                Logical package name -> per-manager package name tables (YAML)
  hardening/
    linux_linux.go          sysctl, sshd drop-ins, ufw/nftables, fail2ban, Lynis
    macos_darwin.go         FileVault, Gatekeeper/spctl, SSH, remote Apple events, OpenBSM audit
    windows_windows.go      BitLocker check, SMBv1, execution policy, Defender, LSA Protection
ui/
  app.go                    Bubbletea Model/Update/View
  components/               header, profile_selector, progress, log_viewer
profiles/                   Embedded profiles: dev.yaml, rev-eng.yaml, pentest.yaml, hardening.yaml
```

Notes:

- Use `internal/`, not `pkg/`. This is an application, not a library.
- Not every directory exists yet. Check the tree before assuming a file is present, and follow the build order below.

## Commands

```bash
go build ./cmd/devsec                          # build for host OS
go test ./...                                  # unit tests (must pass without root and without touching the host)
go vet ./...
gofmt -l .                                     # must output nothing
golangci-lint run                              # if installed

# Cross-compile checks (run before finishing any change touching OS-specific code)
GOOS=linux   GOARCH=amd64 go build ./...
GOOS=darwin  GOARCH=arm64 go build ./...
GOOS=windows GOARCH=amd64 go build ./...

# Safe manual runs
./devsec --dry-run --profile profiles/dev.yaml
./devsec audit --profile profiles/hardening.yaml    # report only
```

## Core Contracts

### Step

Every unit of work implements this interface. Do not bypass it.

```go
type Step interface {
    ID() string
    Check(ctx context.Context) (State, error) // Satisfied | Missing | Drifted. Read-only.
    Apply(ctx context.Context) error          // Mutates. Must be safe to re-run.
    Describe() string                         // Human-readable; used by dry-run and UI
    NeedsPrivilege() bool
}
```

Rules:

- `Check` must never mutate the system.
- `Apply` is only called when `Check` returns Missing or Drifted.
- `Describe` must show the exact commands or file changes that `Apply` would perform.

### Runner

All command execution goes through a `Runner` interface (`Run(ctx, name, args...)`). Never call `os/exec` directly outside the real Runner implementation. This is what makes translators unit-testable against recorded commands, and what lets dry-run work.

### Translator

Package manager backends implement a common interface (`Installed()`, `Install()`, `Version()`). Logical package names are resolved through the mapping tables (e.g. `fd` vs `fd-find`, `bat` vs `batcat` on Debian). Add new mappings to the YAML tables, not to Go code.

## Coding Conventions

- OS-specific code uses `_linux.go`, `_darwin.go`, `_windows.go` suffixes or `//go:build` tags. Windows registry code must never break macOS/Linux builds.
- Wrap errors with context (`fmt.Errorf("apt install %s: %w", pkg, err)`). No panics in library code.
- Pass `context.Context` as the first argument to anything that does I/O or runs commands.
- Never use `curl | sh` or equivalent. Binary/GitHub-release fallbacks must pin a version and verify a checksum or signature.
- Prefer small, focused files and table-driven tests.
- Keep Bubbletea models pure: side effects go in `tea.Cmd`s that call into the engine, and results return as messages.
- Interactive sudo inside the TUI must use `tea.ExecProcess` or a pre-auth step (`sudo -v`) before the TUI starts. Never run the whole TUI as root.

## Privilege Handling

- Linux/macOS: detect non-root, prompt for sudo only for steps where `NeedsPrivilege()` is true.
- Windows: detect admin at launch; self-elevate via `ShellExecute` with `runas` when needed. Elevated processes lose stdout, so results must come back over a named pipe or the tool must relaunch with the same profile and flags.
- Agents must not add code that silently escalates privileges or caches credentials.

## Hardening Module Rules

These are strict because mistakes can lock users out.

1. **Audit and apply are separate.** Audit reports only. Apply requires an explicit flag/confirmation.
2. **Use drop-in files** (`/etc/ssh/sshd_config.d/`, `/etc/sysctl.d/`) instead of editing main config files.
3. **Validate before applying** (`sshd -t`, `sysctl --system` dry checks, etc.) and abort on failure.
4. **Back up** anything changed and provide a rollback path.
5. **Be careful with remote sessions.** Detect SSH sessions and warn or refuse before enabling a firewall or disabling password login.
6. Map checks to CIS/STIG IDs where applicable and include the ID in the audit output.
7. **Validate profile combinations.** Warn on conflicts (for example, pentest tooling that Defender will quarantine alongside strict Defender policy).

Terminology to keep correct in code and docs:

- Reverse engineering: **Rizin** and **Cutter**; **iaito** is the GUI for radare2.
- macOS auditing is **OpenBSM** (`audit_control`), not auditd.
- Gatekeeper is managed with `spctl`; treat them as one feature.
- Windows credential protection means **LSA Protection (RunAsPPL)** and Credential Guard where supported.

## Testing

- Unit tests must run without root, without network, and without modifying the host. Use a fake `Runner` and temp directories.
- Integration tests run only in disposable environments: Linux in containers (Debian, Ubuntu, Arch, Fedora), macOS in Tart/UTM VMs, Windows in a snapshotted VM.
- **Never run `Apply` for hardening steps against the developer's own machine.** Agents must use dry-run, audit mode, or containers/VMs.
- Every new Step needs tests for: Check returns Satisfied, Missing, and Drifted; Apply is idempotent; Describe matches actual commands.

## Profiles

- Profiles live in `profiles/` and are embedded in the binary. Users can supply their own via `--profile path.yaml`.
- Schema changes require: updating `internal/config/schema.go`, the validator, all embedded profiles, and a note in the changelog.
- Keep profiles declarative. No shell snippets embedded in YAML unless they go through a defined step type.

## Scope and Safety Guardrails for Agents

- This project **installs and configures** publicly available security tooling (nmap, Ghidra, Burp, etc.) and **hardens** systems. It does not contain exploit code, payloads, or offensive tooling of its own. Do not add any.
- Don't add telemetry, network calls at startup, or auto-update behavior without an explicit request.
- Don't add dependencies casually. Justify new ones, prefer the standard library, and check licenses.
- Don't modify `go.mod`/`go.sum` beyond what a change requires.
- Destructive operations (deleting files, disabling services, changing auth) require a clear `Describe()` and must be skipped in dry-run.

## Suggested Build Order

1. Config schema, plan builder, and dry-run (no UI)
2. One translator (apt or brew) plus the Check/Apply loop
3. Headless execution (`--non-interactive`, `--yes`) with logging
4. Bubbletea UI on top of the working engine
5. Remaining OSes/translators, then hardening in audit-only mode, then hardening apply

## Definition of Done

- [ ] `gofmt`, `go vet`, and `go test ./...` pass
- [ ] Cross-compile succeeds for linux, darwin, and windows
- [ ] New Steps implement the full interface, with tests
- [ ] Dry-run output accurately describes the change
- [ ] No direct `os/exec` use outside the Runner
- [ ] Docs/profiles updated if behavior or schema changed
