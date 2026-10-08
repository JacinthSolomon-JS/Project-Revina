# Project Revina
Building a multi-OS environment provisioner and hardening tool across macOS, Windows, and Linux.

### 1. System Architecture & Tech Stack
##### Recommended Engine: Rust or Go
- **Go**: Fast cross-platform builds, rich native support for TUI libraries (`bubbletea`), lightweight concurrency for parallel installations, and single-binary distribution.
- **Rust**: Excellent memory safety, cross-compilation target tooling (`cross`), and top-tier TUI interfaces (`ratatui`).

##### Architecture Blueprint

``` Plaintext
                   +-----------------------+
                   |  CLI / TUI Interface  |
                   |  (Bubbletea / Clap)   |
                   +-----------+-----------+
                               |
                   +-----------v---------------------+
                   | Execution Engine / Orchestrator |
                   +-----+-----------+---------------+
                         |           |
            +------------+           +------------+
            |                                     |
+-----------v-------------+             +-----------v-----------+
| OS Package Translators  |             |  Hardening Modules    |
| (Brew/Winget/Apt/Pacman)|             | (Registry/PAM/Sysctl) |
+-------------------------+             +-----------------------+
```

### 2. Cross-Platform Package Abstraction Engine
Your CLI must act as a meta-package manager. It maps logical package names to native OS package managers and fallback binary releases.

| Domain | Generic Package | macOS Installer | Linux Installer | Windows Installer |
| --- | --- | --- | --- | --- |
| CLI Essentials | ripgrep, fzf, bat, zoxide | brew install ... | apt install / pacman -S | winget install ... / choco |
| Dev / Language | go, rustup, pyenv, docker | brew / Cask | Official .sh installers / APT | winget / Official MSI |
| Reves Eng / Binary | ghidra, gdb, cutter, radare2 | brew install --cask ghidra | GitHub Releases / APT / AUR | Direct GitHub API asset extraction |
| Pentesting | nmap, metasploit, ffuf, burp | brew / Direct binaries | Native APT (Kali/Parrot) / Go install | WSL2 automated environment setup |
### 3. Module Categorization & Tooling Blueprint
Organize setup tasks into modular profiles so users can toggle specific suites:

##### A. Development & Programming
**Languages:** 
- [ ] Python (via uv or pyenv)
- [ ] Rust (rustup)
- [ ] Go
- [ ] Node (fnm or nvm)
- [ ] C/C++ toolchains (build-essential / Xcode tools / MSVC).

**CLI/TUI Utilities:**
- [ ] eza
- [ ] bat
- [ ] fzf
- [ ] ripgrep
- [ ] zoxide
- [ ] lazygit
- [ ] lazydocker
- [ ] neovim

##### B. Reverse Engineering & Binary Analysis
**Static/Dynamic:** 
- [ ] Ghidra
- [ ] Cutter/Rizal
- [ ] Radare2/IAITO
- [ ] GDB + pwndbg / gef
- [ ] x64dbg (Windows)

**Utils:** 
- [ ] binwalk
- [ ] checksec
- [ ] LIEF
- [ ] strace/dtrace
- [ ] decompiler-explorer CLI

##### C. Pentesting, Red Teaming & OSINT
**Network & Recon:** 
- [ ] nmap
- [ ] masscan
- [ ] wireshark
- [ ] tshark
- [ ] bettercap

**Web Exploitation:** 
- [ ] burpsuite
- [ ] ffuf
- [ ] gobuster
- [ ] sqlmap
- [ ] httpx
- [ ] subfinder.

**Active Directory & Creds:** 
- [ ] netexec
- [ ] bloodhound
- [ ] hashcat
- [ ] john

##### D. System Hardening & Security Audit
**macOS:** 
- [ ] Enable FileVault
- [ ] disable SSH root/password login
- [ ] enforce Gatekeeper
- [ ] disable remote Apple events
- [ ] adjust spctl.

**Linux:** 
- [ ] Audit with Lynis
- [ ] enforce SSH hardening (sshd_config)
- [ ] set secure sysctl kernel parameters (ASLR, SYN flood protection)
- [ ] set up ufw / nftables
- [ ] fail2ban.

**Windows:** 
- [ ] Enforce BitLocker check
- [ ] disable SMBv1
- [ ] configure PowerShell execution policy
- [ ] adjust Windows Defender policies
- [ ] block LSA secrets dumping

### 4. Key Engineering Challenges to Handle
**Privilege Escalation Routing:**
- Linux/macOS: Detect non-root execution and selectively prompt for sudo only when executing root-required actions.
- Windows: Detect administrative rights at launch (IsUserAnAdmin API) and self-elevate via ShellExecute with runas if required.

**Idempotency:** Running the tool multiple times should modify state only when missing or outdated. Store configuration state in ~/.config/yourtool/state.json.

**Dry-Run Mode:** Provide a --dry-run flag showing all shell execution steps before changes are applied to the OS.

**Declarative Config Files:** Support YAML or TOML profiles (e.g., profile-malware-analysis.yaml) so setups can be shared across teams via Git.
---
### Part 1: Go + Bubbletea Architecture & Project Structure

The project follows the Elm Architecture pattern native to Bubbletea, separating pure terminal UI state from system execution side effects.

`Project Directory Layout

```Plaintext

devsec-provisioner/
├── main.go                 # Entrypoint, flag parsing, privilege detection
├── pkg/
│   ├── config/
│   │   ├── loader.go               # Parses YAML/TOML profiles & validates schema
│   │   └── schema.go               # Profile & Task Go struct definitions
│   ├── engine/
│   │   ├── executor.go             # Idempotent execution coordinator
│   │   ├── system.go               # OS detector, elevation helper (Sudo/RunAs)
│   │   └── translators/
│   │       ├── brew.go             # macOS Homebrew translator
│   │       ├── winget.go           # Windows WinGet / Choco translator
│   │       ├── apt.go              # Debian/Ubuntu APT translator
│   │       └── pacman.go           # Arch Linux Pacman translator
│   └── hardening/
│       ├── linux.go                # Sysctl, SSH, UFW hardening
│       ├── macos.go                # Gatekeeper, FileVault, Auditd settings
│       └── windows.go              # Registry tweaks, Defender, Service disabling
├── ui/
│   ├── components/
│   │   ├── header.go               # Styled banner & platform info
│   │   ├── profile_selector.go     # Interactive multi-select tree
│   │   ├── progress.go             # Live execution progress bar
│   │   └── log_viewer.go           # Scrollable viewport for stdout/stderr
│   └── app.go                      # Core Bubbletea Model, Update, & View handlers
├── profiles/                       # Standard embedded profile definitions
│   ├── dev.yaml
│   ├── rev-eng.yaml
│   ├── pentest.yaml
│   └── hardening.yaml
├── go.mod
└── go.sum
```
