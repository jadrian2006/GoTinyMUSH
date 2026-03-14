# Building & Deployment Guide

## Prerequisites

- **Go 1.24.0** or later (for building from source)
- **Node.js 22+** (only if building the admin panel frontend)
- **Docker** and **Docker Compose** (for container deployment)

No C compiler, GDBM, or other native dependencies are required. GoTinyMUSH is pure Go with `CGO_ENABLED=0`.

## Building from Source

Clone the repository and build:

```bash
git clone https://github.com/jadrian2006/GoTinyMUSH.git
cd GoTinyMUSH
go build -o gotinymush ./cmd/server
```

On Windows, this produces `gotinymush.exe` automatically.

### Build with Version String

```bash
go build -ldflags "-X github.com/crystal-mush/gotinymush/pkg/server.Version=0.7.0" -o gotinymush ./cmd/server
```

### Build with Embedded Admin Panel

The admin panel is a Preact/Vite frontend that gets embedded into the Go binary via `go:embed`. To include it:

```bash
cd web/admin
npm ci
npm run build        # outputs to ../../pkg/admin/dist
cd ../..
go build -o gotinymush ./cmd/server
```

### Running Tests

```bash
# All Go unit tests
go test ./...

# Batch eval tests (softcode function verification)
go run ./cmd/evaltest -batch tests/eval_basic.txt

# Interactive eval testing against a database
go run ./cmd/evaltest -db data/minimal.FLAT -player 1
```

For Docker-based testing (no local Go install required):

```bash
docker run --rm -v "$(pwd):/src" -w /src golang:latest go test -buildvcs=false ./...
```

## Release Binaries

Pre-built binaries are published via [GoReleaser](https://goreleaser.com/) to GitHub Releases at `https://github.com/jadrian2006/GoTinyMUSH/releases`.

Release archives are built for:

| OS | Architecture | Format |
|---|---|---|
| Linux | amd64, arm64 | `.tar.gz` |
| macOS | amd64, arm64 | `.tar.gz` |
| Windows | amd64, arm64 | `.zip` |

Each archive includes:
- `gotinymush` binary (or `gotinymush.exe`)
- `README.md`, `LICENSE`, `CREDITS`
- `data/game.yaml` (example config)
- `data/goTinyAlias.conf` (alias definitions)
- `data/minimal.FLAT` (seed database)
- `data/text/*` (help files, connect screens)
- `data/dict/base.txt` (spellcheck dictionary)
- `docs/*` (documentation)

## Docker Deployment

### Quick Start

```bash
docker compose up -d --build
```

This builds the image and starts the server. On first run with no database configured, the server starts in **setup mode**: only the admin panel is available at `http://localhost:8443/admin/`. Upload a flatfile or archive through the admin panel to initialize the game.

### Docker Image Structure

The Dockerfile uses a multi-stage build:

1. **Stage 1** (`node:22-alpine`): Builds the admin panel frontend
2. **Stage 2** (`golang:latest`): Compiles the Go binary with `CGO_ENABLED=0`, embedding the admin panel
3. **Stage 3** (`alpine:latest`): Minimal runtime image with `su-exec`, `tzdata`, and `libcap`

The final image runs as a non-root `mush` user. The entrypoint script adjusts UID/GID to match `PUID`/`PGID` environment variables and uses `setcap` to allow binding to privileged ports (80/443).

### Exposed Ports

| Port | Purpose |
|---|---|
| 6250 | Telnet game port (cleartext) |
| 8443 | Web server (admin panel, REST API, WebSocket) |
| 80 | Let's Encrypt ACME challenge (when `web_domain` is set) |
| 443 | HTTPS (when `web_domain` is set) |

### Docker Compose Configuration

```yaml
services:
  gotinymush:
    build:
      context: .
      args:
        VERSION: "0.7.0"
    ports:
      - "6250:6250"
      - "8443:8443"
    volumes:
      - gamedata:/game/data
    environment:
      PUID: 1000
      PGID: 1000
      TZ: America/New_York
      MUSH_CONF: /game/data/game.yaml
      MUSH_TEXTDIR: /game/data/text
      MUSH_DICTDIR: /game/data/dict
      MUSH_GODPASS: "changeme"
      MUSH_ADMIN_PASS: "changeme"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8443/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s

volumes:
  gamedata:
```

### Setup Mode vs Normal Mode

When `MUSH_BOLT` and `MUSH_DB` are both unset, the server starts in **setup mode**:
- Only the admin panel web server runs (on the web port, default 8443)
- No telnet listener, no game engine
- Navigate to `http://localhost:8443/admin/` to upload a flatfile or configure the game
- Once a database is imported through the admin panel, restart with `MUSH_BOLT` set to enter normal mode

To skip setup mode and go straight to a running game:

```yaml
environment:
  MUSH_BOLT: /game/data/game.bolt
  MUSH_DB: /game/data/minimal.FLAT   # only needed on first run
```

### Importing Your Own Database

Mount your flatfile into the container:

```yaml
volumes:
  - gamedata:/game/data
  - ./mygame.FLAT:/game/data/mygame.FLAT
environment:
  MUSH_DB: /game/data/mygame.FLAT
  MUSH_BOLT: /game/data/game.bolt
```

After the first successful import, remove `MUSH_DB` (or leave it; it is only used when the bolt database does not exist or `-import` is set).

### Importing Comsys (Channel Data)

If migrating from C TinyMUSH with a `mod_comsys.db` file:

```yaml
volumes:
  - ./mod_comsys.db:/game/data/mod_comsys.db
environment:
  MUSH_COMSYSDB: /game/data/mod_comsys.db
```

Channels are imported into bbolt on first load and served from bbolt on subsequent starts.

### Setting the God Password

```bash
# Via environment variable (recommended)
docker compose exec gotinymush sh -c \
  'MUSH_GODPASS=mynewpassword gotinymush -bolt /game/data/game.bolt -conf /game/data/game.yaml'

# Or set it in docker-compose.yml
environment:
  MUSH_GODPASS: "mynewpassword"
```

When `MUSH_GODPASS` is set, the password is applied at startup and the server continues booting normally.

## Directory Structure

After installation (release binary or Docker), the layout is:

```
gotinymush              # Server binary
data/
  game.yaml             # Game configuration (YAML)
  game.bolt             # bbolt database (created on first run)
  goTinyAlias.conf      # Command/flag/function/attribute aliases
  minimal.FLAT          # Seed database (Room Zero + Wizard)
  text/
    connect.txt         # Login screen shown to new connections
    motd.txt            # Message of the day (shown after login)
    newuser.txt         # Shown to newly created characters
    register.txt        # Registration info screen
    help.txt            # Player help database
    wizhelp.txt         # Wizard help database
    qhelp.txt           # Quick help database
    news.txt            # news command content
  dict/
    base.txt            # Base spellcheck dictionary
backups/                # Archive output directory (created as needed)
```

In the Docker image, the layout under `/game/` is:

```
/game/
  seed/                 # Read-only seed files (copied to data/ on first boot)
    game.yaml
    goTinyAlias.conf
    minimal.FLAT
    text/
    dict/
  data/                 # Persistent volume mount point
    game.yaml           # Working config (copied from seed/)
    game.bolt           # bbolt database
    text/               # Working text files
    dict/               # Working dictionary
  certs/                # Generated TLS certificates
```

## Data Files

### Text Files

Text files in `data/text/` are **hot-reloaded** via filesystem watching (`fsnotify`). Edit them while the server is running and changes take effect immediately without a restart.

| File | In-Game Command | Purpose |
|---|---|---|
| `help.txt` | `help <topic>` | Player help database |
| `wizhelp.txt` | `wizhelp <topic>` | Wizard-only help |
| `qhelp.txt` | `qhelp <topic>` | Quick help |
| `news.txt` | `news <topic>` | Game news |
| `connect.txt` | (login screen) | Shown to new connections |
| `motd.txt` | `@motd` | Message of the day |
| `newuser.txt` | (on create) | Shown to newly created characters |
| `register.txt` | (on register) | Registration information |

Help files use TinyMUSH format: topic entries start with `& TOPIC NAME` on their own line, followed by the help text.

### Alias Configuration

`data/goTinyAlias.conf` defines aliases using five directive types:

```
alias          sa       say              # command alias
flag_alias     halt     HALT             # flag alias
function_alias strlen   strlen           # function alias
attr_alias     AAHEAR   AMHEAR           # attribute alias
power_alias    boot     boot             # power alias
bad_name       God                       # forbidden player name
```

This replaces both `alias.conf` and `compat.conf` from C TinyMUSH.

### Seed Database

`data/minimal.FLAT` is a minimal TinyMUSH flatfile containing:
- Room #0 (Room Zero / default starting room)
- Player #1 (Wizard / God) with password `potrzebie`
- Room #2 (Master Room)

## First-Time Setup

### From Release Binary

```bash
# 1. Extract
tar xzf gotinymush_*.tar.gz
cd gotinymush_*/

# 2. Start (imports seed database into bbolt)
./gotinymush -conf data/game.yaml -db data/minimal.FLAT -bolt data/game.bolt \
  -textdir data/text -aliasconf data/goTinyAlias.conf

# 3. Connect with any MUD client
telnet localhost 6250
connect Wizard potrzebie

# 4. Change the God password immediately
@newpassword me = mynewpassword
```

### From Docker

```bash
# 1. Build and start
docker compose up -d --build

# 2. Open admin panel to upload database
# http://localhost:8443/admin/

# 3. Or skip admin panel — set env vars for direct boot
# Add MUSH_BOLT and MUSH_DB to docker-compose.yml, restart

# 4. Connect
telnet localhost 6250
connect Wizard potrzebie
```

### Importing an Existing TinyMUSH Database

```bash
./gotinymush -conf data/game.yaml -db /path/to/mygame.FLAT -bolt data/game.bolt \
  -textdir data/text -aliasconf data/goTinyAlias.conf \
  -comsysdb /path/to/mod_comsys.db
```

On subsequent starts, omit `-db` and `-comsysdb`:

```bash
./gotinymush -conf data/game.yaml -bolt data/game.bolt \
  -textdir data/text -aliasconf data/goTinyAlias.conf
```

## Running the Server

### Foreground

```bash
./gotinymush -conf data/game.yaml -bolt data/game.bolt \
  -textdir data/text -aliasconf data/goTinyAlias.conf
```

### Background (systemd)

Create `/etc/systemd/system/gotinymush.service`:

```ini
[Unit]
Description=GoTinyMUSH Server
After=network.target

[Service]
Type=simple
User=mush
WorkingDir=/opt/gotinymush
Environment=MUSH_GODPASS=
ExecStart=/opt/gotinymush/gotinymush \
  -conf data/game.yaml \
  -bolt data/game.bolt \
  -textdir data/text \
  -aliasconf data/goTinyAlias.conf
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now gotinymush
```

### Debug Mode

```bash
MUSH_DEBUG=true ./gotinymush -conf data/game.yaml -bolt data/game.bolt ...
```

A pprof debug endpoint is automatically started on port 6060 (`http://localhost:6060/debug/pprof/`).

## Backup and Restore

### In-Game Backup

```
@archive
```

### Scheduled Backups

In `game.yaml`:

```yaml
archive_dir: backups
archive_interval: 60
archive_retain: 24
archive_hook: "scp %f user@backup-host:/backups/"
```

### Restore from Archive

```bash
./gotinymush -restore backups/archive-20260214-120000.tar.gz \
  -bolt data/game.bolt -conf data/game.yaml
```

Archives are `.tar.gz` files containing the bolt database, config, text files, and dictionary with integrity checksums.

## Additional Build Targets

The repository includes two additional commands:

| Command | Purpose |
|---|---|
| `cmd/server` | Main game server |
| `cmd/evaltest` | Interactive/batch softcode evaluator for testing |
| `cmd/dbloader` | Standalone database loader and inspector |

Build any of them with:

```bash
go build -o <name> ./cmd/<name>
```
