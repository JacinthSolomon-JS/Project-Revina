# Project Revina

A simple, cross-platform tool that sets up your developer environment and
hardens your machine — one command, on macOS, Windows, or Linux.

You write a small YAML profile describing what you want, and devsec installs the
packages and applies the hardening using your OS's native tools (brew, apt,
dnf, pacman, winget, scoop, choco). You can see exactly what it will do before
it does anything.

## Quick start

```bash
go build ./cmd/devsec        # builds the ./devsec binary
```

See what a profile will do (changes nothing):

```bash
./devsec --dry-run --profile dev.yaml
```

Check your machine's current state (changes nothing):

```bash
./devsec audit --profile hardening.yaml
```

Run interactively (shows a plan in a terminal UI, press Enter to proceed):

```bash
./devsec --profile dev.yaml
```

Run without prompts:

```bash
./devsec --profile dev.yaml --yes
```

## Profiles

Profiles describe what to install and harden. Four come built in:

| Profile | What it does |
| ------- | ------------ |
| `dev.yaml` | Core dev tools: ripgrep, fzf, bat, zoxide, go, rustup |
| `rev-eng.yaml` | Reverse engineering: Ghidra, Rizin, radare2, gdb, ... |
| `pentest.yaml` | Pentesting: nmap, wireshark, burp, ffuf, hashcat, ... |
| `hardening.yaml` | System hardening (audit by default, apply is opt-in) |

You can also write your own profile file and pass it with `--profile`:

```yaml
name: my-setup
tasks:
  - id: cli
    kind: package
    packages: [ripgrep, zoxide]
  - id: secure
    kind: hardening
    rules: [sshd-password-auth]
```

## Safety

- **`--dry-run`** and **`audit`** never change anything.
- Hardening is **audit by default**; applying it always asks first (or needs
  `--yes`).
- Changes use drop-in config files with backups, are validated, and roll back
  on failure.
- Rules that could lock you out (firewall, SSH) refuse to run over an SSH
  session.

## Docs

- **[USER-GUIDE.md](docs/USER-GUIDE.md)** — full guide: flags, profiles, the TUI,
  troubleshooting.
- **[SECURITY.md](docs/SECURITY.md)** — security model and hardening guarantees.

## Build status

[gofmt, go vet, all unit tests, and linux/darwin/windows cross-compiles pass]
