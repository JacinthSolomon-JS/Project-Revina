# User Guide

`devsec` is a cross-platform (macOS, Windows, Linux) developer-environment
provisioner and system-hardening tool. You describe what you want in a profile
(YAML), and devsec builds a plan and applies it idempotently using your OS's
native tools.

This guide covers building the binary, using the CLI and TUI, working with
profiles, and what the `audit` and hardening modes do.

---

## 1. Building

Requires Go (see `go.mod` for the version).

```bash
go build ./cmd/devsec          # produces ./devsec on the host OS
```

Cross-compiling for other platforms:

```bash
GOOS=linux   GOARCH=amd64 go build ./cmd/devsec
GOOS=darwin  GOARCH=arm64 go build ./cmd/devsec
GOOS=windows GOARCH=amd64 go build ./cmd/devsec
```

The binary is self-contained: profiles ship embedded inside it, so it works
from any directory with no files on disk.

---

## 2. Quick start

### See what a profile would do (no changes)

```bash
./devsec --dry-run --profile dev.yaml
```

`--dry-run` prints the exact commands each step would run (including `sudo`
wrapping and hardening drop-in file writes). Nothing is changed, checked, or
contacted.

### Audit the current machine (read-only)

```bash
./devsec audit --profile hardening.yaml
```

`audit` reports each step's state — `satisfied`, `missing`, or `drifted` — by
probing the real system, and a summary line. It never applies anything.

### Run interactively

```bash
./devsec --profile dev.yaml
```

On a real terminal this opens the TUI:

1. Review the plan (step ID, `sudo` badge, exact command/change per step).
2. Press **Enter/Space** to start, **q/Esc** to quit before anything runs.
3. Watch the progress bar and streaming per-step log.
4. On completion you see a summary; if any step failed, devsec exits non-zero.

If any step needs elevation, devsec runs `sudo -v` first so the TUI itself is
never run as root; the password prompt appears before the TUI starts.

### Run non-interactively

```bash
./devsec --profile dev.yaml --non-interactive   # never prompts; errors instead of assuming yes
./devsec --profile dev.yaml --yes               # skip confirmation and apply
./devsec --profile hardening.yaml --yes         # apply hardening (opt-in)
```

Out of a terminal, devsec refuses to run without one of these two flags.

---

## 3. Commands and flags

| Command            | Purpose                                                    |
| ------------------ | ---------------------------------------------------------- |
| `devsec` (default) | Build and run a plan against the machine.                  |
| `devsec audit`     | Report current state of every step, apply nothing.         |

Common flags:

| Flag                | Default     | Meaning                                                                              |
| ------------------- | ---------- | ------------------------------------------------------------------------------------- |
| `--profile`         | `dev.yaml` | Embedded profile name, or a path to a YAML file (e.g. `--profile ./my-profile.yaml`). |
| `--dry-run`         | `false`    | Print the plan and stop.                                                              |
| `--yes`             | `false`    | Apply without prompting.                                                              |
| `--non-interactive` | `false`    | Never prompt; refuse rather than assume yes.                                          |
| `--state`           | (default)  | Audit state file path; default `~/.config/devsec/state.json`.                         |
| `--log-level`       | `info`     | `debug`, `info`, `warn`, or `error`.                                                  |

---

## 4. Profiles

Profiles are declarative YAML. Four ship embedded:

| Profile           | What it does                                                                           |
| ----------------  | -------------------------------------------------------------------------------------- |
| `dev.yaml`        | Core developer CLI utilities (ripgrep, fzf, bat, zoxide, go, rustup).                  |
| `rev-eng.yaml`    | Reverse-engineering & binary analysis (Ghidra, Rizin, radare2, gdb, binwalk, ...).     |
| `pentest.yaml`    | Pentesting & red-team toolkit (nmap, wireshark, burp, ffuf, hashcat, metasploit, ...). |
| `hardening.yaml`  | System hardening; see section 5.                                                       |

Minimal custom profile:

```yaml
name: my-setup
description: Personal environment
tasks:
  - id: cli
    kind: package
    description: CLIs
    packages:
      - ripgrep
      - zoxide
  - id: security
    kind: hardening
    description: Core hardening
    rules:
      - sshd-password-auth
```

Run it with `./devsec --profile ./my-setup.yaml`.

### Package names and mapping

Profile package names are *logical* names. Each OS maps them to its native
names through per-manager tables (bulk of the mapping is in
`internal/engine/mapping/tables/*.yaml`), so the same profile works on each OS —
e.g. `rg` → `ripgrep` (brew), `fd` → `fd-find` (Debian), `bat` →
`sharkdp.bat` (winget).

Names must match `^[A-Za-z0-9][A-Za-z0-9+_.:=@-]*$`. Anything option-like is
rejected at load time.

### Which package managers are used

| OS      | Manager(s) detected, in order |
| ------- | ----------------------------- |
| macOS   | `brew`                        |
| Linux   | `apt` → `dnf` → `pacman`      |
| Windows | `winget` → `scoop` → `choco`  |

Linux installs that need root are wrapped with `sudo`. Windows has no `sudo`;
steps that require elevation report it, and the run logs a warning if the shell
is not elevated.

---

## 5. Hardening mode

`hardening` is the safety-sensitive part. Read `SECURITY.md` §4 for the model.
The key points for users:

- `devsec audit --profile hardening.yaml` is always safe — it only looks.
- Applying requires an explicit interactive confirm, or `--yes`.
- Changes use **drop-in files** with backups, are **validated** (e.g. `sshd -t`)
  and **rolled back** on failure.
- SSH-locking rules (firewall enable, disable SSH password auth, disable remote
  login) **refuse to apply over an SSH session**.

Available rules per OS:

| Rule                  | Linux | macOS | Windows |
| --------------------- | ----- | ----- | ------- |
| `sysctl-secure`       | ✅    | —     | —       |
| `sshd-password-auth`  | ✅    | ✅    | —       |
| `ufw-enable`          | ✅    | —     | —       |
| `gatekeeper`          | —     | ✅    | —       |
| `remote-login`        | —     | ✅    | —       |
| `disable-smbv1`       | —     | —     | ✅      |
| `powershell-policy`   | —     | —     | ✅      |

Rules not applicable to your OS are skipped silently (so you can share one
`hardening.yaml` everywhere); typos are errors.

---

## 6. State and audit log

Every applied run appends one NDJSON line per step to
`~/.config/devsec/state.json` (override with `--state`). It records step ID,
state, whether it applied, error, and duration. Both the directory and the file
are created private to your user. The file is **not** your source of truth —
devsec always re-checks the real system; the state file is an audit trail.

---

## 7. Troubleshooting

- **`refusing to run non-interactively without --non-interactive or --yes`**
  — you ran it from a script/pipe. Pass `--non-interactive` (fail loudly) or
  `--yes` (apply without prompting).
- **`invalid log level`** — use `--log-level debug|info|warn|error`.
- **`hardening requires a rule factory`** — internal wiring error; report it.
- **`unknown hardening rule "foo"`** — typo in a `rules:` entry.
- **`refusing to modify ... file was not created by devsec`** — a drop-in file
  already exists that devsec didn't write; devsec will not touch it. Remove or
  back it up yourself, then re-run.
- **`refusing ... over an SSH session`** — expected: the rule would risk
  locking remote users out. Apply from the console, or pre-add an allow rule.
- **Package install fails with an empty result** — confirm the manager is
  installed (`brew`, `apt-get`, `winget`, ...). devsec distinguishes "manager
  missing" (error) from "package not installed" (normal `missing` state).

---

## 8. Development notes

- All command execution goes through the `Runner` interface — tests swap in a
  `FakeRunner`, so translator and rule behavior is tested without touching a
  real OS.
- Unit tests run without root, network, or host modification:
  `go test ./...` and `go test -race ./internal/...`.
- Safe local testing: `./devsec --dry-run --profile dev.yaml` and
  `./devsec audit --profile hardening.yaml`.
- For contributing details, see `docs/AGENTS.md`.
