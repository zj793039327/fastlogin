# fastlogin

An interactive terminal command launcher. Pick a server or command from a grouped, foldable, searchable list and hand the terminal over to a Runner (SSH or arbitrary command). The session ends → the program exits.

Built with Go + [bubbletea](https://github.com/charmbracelet/bubbletea). SSH connections use `golang.org/x/crypto/ssh` (no external `ssh`/`sshpass` dependency; passwords never appear in the process list).

## Features

- **Interactive TUI** — grouped list with fold/expand, fuzzy search, cursor selection
- **Vim keybindings** — `j`/`k`/`h`/`l` alongside arrow keys
- **SSH** — password login, PEM private key, or `pem: default` (uses local SSH agent + `~/.ssh/config` IdentityFile + default keys)
- **Arbitrary commands** — `type: command` entries exec any interactive command under a PTY (e.g. `mysql -h ... -u ... -p`)
- **Config sources** — a YAML config file, optionally merged with `~/.ssh/config`
- **Pluggable Runners** — add new session types (mysql/postgres/etc.) by implementing one interface

## Install

### macOS — Homebrew (recommended)

```bash
brew install zj793039327/tap/fastlogin
```

Upgrade later:

```bash
brew update && brew upgrade fastlogin
```

### Windows — Scoop (recommended)

```powershell
scoop bucket add zj https://github.com/zj793039327/scoop-bucket
scoop install fastlogin
```

Upgrade later:

```powershell
scoop update fastlogin
```

### All platforms — prebuilt binary

Download the archive for your OS/arch from [GitHub Releases](https://github.com/zj793039327/fastlogin/releases), extract it, and put the binary on your `PATH`.

### Build from source

Requires Go 1.21+.

```bash
git clone https://github.com/zj793039327/fastlogin.git
cd fastlogin
go build -o fastlogin .
```

## Configuration

fastlogin reads `~/.config/fastlogin/config.yaml` by default. See [`config.example.yaml`](config.example.yaml) for a full example.

```yaml
groups:
  - name: Production
    description: Production servers — handle with care
    entries:
      - name: web-01
        type: ssh
        host: 10.0.1.10
        user: root
        port: 22
        auth:
          password: "secret"
        tags: [web, prod]

      - name: db-01
        type: ssh
        host: 10.0.1.20
        user: admin
        auth:
          pem: ~/.ssh/db01.pem

  - name: Dev
    entries:
      - name: local-dev
        type: ssh
        host: 127.0.0.1
        user: dev
        auth:
          password: "dev123"

      - name: mysql-repl
        type: command
        command: mysql -h 10.0.1.20 -u admin -p

# Loose top-level entries (rendered without a group header)
entries:
  - name: quick-ssh
    type: ssh
    host: 1.2.3.4
    user: root
    auth:
      pem: default   # use local SSH agent + ~/.ssh/config IdentityFile + default keys

# Merge non-wildcard Hosts from ~/.ssh/config into a virtual "ssh-config" group
include_ssh_config: true
```

### Auth options

| `auth:` field | Description |
|---------------|-------------|
| `password` | Plaintext password (MVP; keyring encryption is future work) |
| `pem: <path>` | Path to a PEM private key |
| `pem: default` | Use the local machine's SSH config: SSH agent → `~/.ssh/config` IdentityFile → default keys (`~/.ssh/id_*`) |
| `passphrase` | Optional passphrase for an encrypted PEM |

### `.ssh/config` merge

When `include_ssh_config: true`, each non-wildcard `Host` block (with a `User`) is imported as an SSH entry under a virtual `ssh-config` group, deduplicated by `user@host:port` against YAML-defined entries.

## Usage

```bash
fastlogin
```

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Move cursor |
| `→` / `←` / `Tab` or `h` / `l` | Fold/expand group |
| `⏎` Enter | Connect to the selected entry |
| `/` | Enter search mode |
| `Esc` | Exit search / clear filter |
| `q` / `Ctrl+C` | Quit |

After selecting an entry, the TUI exits and the terminal is handed to the matching Runner. The program exits when the session ends.

## License

MIT
