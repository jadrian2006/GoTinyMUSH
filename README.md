# GoTinyMUSH

A modern reimplementation of [TinyMUSH 3.3](https://github.com/TinyMUSH/TinyMUSH) in Go.

GoTinyMUSH is a from-scratch port of the classic TinyMUSH server, replacing ~76,000 lines of C with ~24,000 lines of Go while preserving compatibility with existing TinyMUSH databases, softcode, and help files.

## Quick Start (Release Binary)

Download the latest release for your platform from [Releases](https://github.com/jadrian2006/GoTinyMUSH/releases).

### 1. Extract

**Linux / macOS:**
```bash
tar xzf gotinymush_*.tar.gz
cd gotinymush_*/
```

**Windows:**
Extract the `.zip` file, then open a terminal (PowerShell or Command Prompt) in the extracted folder.

### 2. Start the server

The release includes `data/minimal.FLAT`, a seed database with Room Zero, a Master Room, and a Wizard character. Start with:

**Linux / macOS:**
```bash
./gotinymush -conf data/game.yaml -db data/minimal.FLAT -bolt data/game.bolt \
  -textdir data/text -aliasconf data/goTinyAlias.conf
```

**Windows (PowerShell):**
```powershell
.\gotinymush.exe -conf data\game.yaml -db data\minimal.FLAT -bolt data\game.bolt `
  -textdir data\text -aliasconf data\goTinyAlias.conf
```

The first run imports the flatfile into a bbolt database (`game.bolt`). On subsequent starts, omit `-db` and the server loads directly from bbolt:

**Linux / macOS:**
```bash
./gotinymush -conf data/game.yaml -bolt data/game.bolt \
  -textdir data/text -aliasconf data/goTinyAlias.conf
```

**Windows (PowerShell):**
```powershell
.\gotinymush.exe -conf data\game.yaml -bolt data\game.bolt `
  -textdir data\text -aliasconf data\goTinyAlias.conf
```

**Importing an existing TinyMUSH database:** If you have a `.FLAT` file from TinyMUSH 3.x, use it instead of `minimal.FLAT`:

```bash
./gotinymush -conf data/game.yaml -db mygame.FLAT -bolt data/game.bolt -textdir data/text -aliasconf data/goTinyAlias.conf
```

### 3. Connect

Use any MUD client (MUSHclient, Mudlet, TinTin++, BeipMU) or plain telnet:

```
telnet localhost 6250
```

You will see the connect screen. The seed database includes a **Wizard** character with the default password **`potrzebie`** (the classic TinyMUSH default):

```
connect Wizard potrzebie
```

### 4. Change the God password

**Change the God password immediately after first login.** The God character is player `#1` (Wizard). Only God can change God's password — other Wizards cannot.

From within the game (as God):
```
@newpassword me = mynewpassword
```

From the command line (server does not need to be running):

**Linux / macOS:**
```bash
MUSH_GODPASS=mynewpassword ./gotinymush -bolt data/game.bolt -conf data/game.yaml
```

**Windows (PowerShell):**
```powershell
$env:MUSH_GODPASS="mynewpassword"; .\gotinymush.exe -bolt data\game.bolt -conf data\game.yaml
```

The `-godpass` flag also works but **environment variables are recommended** since command-line arguments are visible in process listings (`ps`, Task Manager). The `MUSH_GODPASS` env var sets the password and exits without starting the server.

### 5. First steps once connected

```
@version                          -- verify the server is running
look                              -- see your current room
examine me                        -- inspect your character
@dig My First Room                -- create a new room
@describe here=A cozy room.       -- set room description
@wall Hello from GoTinyMUSH!      -- broadcast to all connected players
```

### 6. Creating new players

Anyone can create a character from the login screen:

```
create MyCharacter mypassword
```

New players start in the room configured by `player_starting_room` in `game.yaml` (default: Room #0). They are regular players — use `@set <player>=WIZARD` from a Wizard character to grant admin privileges.

## Configuration

Edit `data/game.yaml` to customize your game. Key settings:

```yaml
mud_name: MyMUSH            # Name shown in WHO list and logs
port: 6250                  # TCP listen port
player_starting_room: 0     # Room # where new players appear
master_room: 2              # Global command room
idle_timeout: 3600          # Seconds before idle disconnect (0 = never)

# Economy
money_name_singular: penny
money_name_plural: pennies
starting_money: 150

# Guest system (uncomment to enable)
# guest_char_num: -1
# guest_prefixes: "Red Blue Green Yellow White"
# guest_basename: Guest
```

See `data/game.yaml` for the full list of options with comments.

### Alias Configuration

`data/goTinyAlias.conf` registers command aliases, flag aliases, function aliases, and attribute aliases. This replaces the old `alias.conf` and `compat.conf` from TinyMUSH 3.x. Edit this file to add custom aliases.

### Text Files

Files in `data/text/` are hot-reloaded — edit them while the server is running and changes take effect immediately:

| File | Purpose |
|---|---|
| `connect.txt` | Login screen shown to new connections |
| `motd.txt` | Message of the day (shown after login) |
| `news.txt` | `news` command content |
| `newuser.txt` | Shown to newly created characters |
| `register.txt` | Registration info screen |
| `help.txt` | Player help database |
| `wizhelp.txt` | Wizard help database |

## What Changed from TinyMUSH 3.3

### Language: C to Go

The entire codebase has been rewritten in Go. No C code remains. This eliminates the classes of bugs that plagued the original (buffer overflows, memory leaks, use-after-free) and makes the server easier to build, deploy, and extend.

| | TinyMUSH 3.3 | GoTinyMUSH |
|---|---|---|
| Language | C | Go |
| Lines of code | ~76,000 | ~24,000 |
| Build system | CMake + autoconf | `go build` |
| Dependencies | libcrypt, GDBM/QDBM | Pure Go (bbolt, yaml) |
| Deployment | Manual compile + scripts | Single binary or Docker |

### Database: GDBM/QDBM to bbolt

TinyMUSH used GDBM (or QDBM) with a custom chunked object cache (`udb_ochunk.c`, `udb_ocache.c`, `udb_obj.c`). GoTinyMUSH replaces all of this with [bbolt](https://go.etcd.io/bbolt), an embedded key/value store written in pure Go.

- **Import**: Reads TinyMUSH flatfile format directly (`.FLAT` files with `+T`, `+S`, `+N`, `!` object headers, `>` attributes)
- **Runtime**: All objects live in memory with bbolt as the persistence layer
- **No LMDB/GDBM dependency**: bbolt is pure Go, no CGO required

### Configuration: .conf to YAML

TinyMUSH's configuration (`netmush.conf`) used a custom key-value format with `include` directives. GoTinyMUSH uses YAML (`game.yaml`). The old `alias.conf` / `compat.conf` include system is replaced by a unified alias config file (`goTinyAlias.conf`).

### Text Files: Hot-Reloading

Connect screens, MOTD, and other text files are loaded from a directory and watched for changes using `fsnotify`. Edit `connect.txt` and the change takes effect immediately without a restart.

### Help System: Parsed at Startup

TinyMUSH's help system used binary `.indx` index files generated by an external tool. GoTinyMUSH parses the `.txt` help files directly at startup, building an in-memory index. Supports exact and prefix matching (`help @swi` finds `@switch`).

### Password Hashing

TinyMUSH used Unix `crypt(3)`. GoTinyMUSH uses the same DES-based crypt for backward compatibility with existing player passwords, so players can connect to an imported database without resetting passwords.

## Current Status

### Working

- **TCP server** with connect/create/WHO/QUIT
- **Flatfile import** into bbolt with full round-trip fidelity
- **376+ softcode functions** covering the full TinyMUSH 3.x function set plus RhostMUSH extensions:
  - Math: arithmetic, trig, hyperbolic, exp/log, bitwise, vector, distance, comparison, logic
  - Vector: vadd, vsub, vmul, vdot, vmag, vunit, vdim, vcross, vdist, vlerp, vnear, vclamp
  - Flight/navigation: hvec, hdelta, hname, gridabs, griddist, gridcourse, drift, bearing, pitch, eta, intercept
  - String: manipulation, formatting, ANSI, borders, encoding, regex, spellcheck, printf, tr
  - Encoding: encode64, decode64, digest (SHA256/SHA1/MD5/SHA512), crc32, tobin/todec/tohex/tooct, roman
  - List: manipulation, sorting, set operations, aggregation, grouping
  - Iteration: iter, parse, map, filter, fold, while, until, step, mix, munge, iter2, whentrue/whenfalse
  - Conditionals: if/ifelse/switch/case + variants
  - Object: queries, flags, powers, pronouns, timestamps, memory, locks, zones, visibility
  - Registers: q-registers, named variables, let, localize, private
  - Regex: regmatch/i, regedit/i, regeditall/i, regrab/i, regraball/i, regrep/i, regparse/i
  - Stack: push/pop/peek/dup/swap/toss/lstack
  - Structure/instance system: typed structures with persistence
  - Connection: lwho, conn, idle, doing, pmatch, ports, session
  - Side-effects: create, set, tel, link, trigger, wipe, force, wait, pemit, remit, oemit
  - System: search, stats, config, eval, fcount/fdepth, starttime, version
  - SQL: sql(), sqlescape()
- **163 commands** including:
  - Communication: say, pose, emit, @pemit, @oemit, @remit, whisper, page, think
  - Building: @create, @dig, @open, @destroy, @describe, @name, @set, @link, @parent, @lock, @unlock, @chzone, @clone, @wipe
  - Movement: go, home, @teleport, enter, leave, get, drop, give
  - Information: look, examine, inventory, score, WHO, @search, @decompile, @find, @version, @stats, @ps
  - Help: help, @help, qhelp, wizhelp, news
  - Admin: @boot, @newpassword, @motd, @wall, @force, @power, @archive
  - Comsys: addcom, delcom, clearcom, comlist, @ccreate, @cdestroy, @clist, @cwho, @cboot, @cemit, @cset
  - ~55 attribute-setting commands (@success, @fail, @sex, @alias, @listen, etc.)
- **Archive/backup system** with @archive, @archive/list, scheduled archives, retention, post-archive hooks, and -restore flag
- **Comsys** (channel system) with channel creation, aliases, per-player subscriptions, and bbolt persistence
- **Softcode queue** with @trigger, @wait, @force, $-command matching, @startup, @notify, @halt
- **Eval engine** with full %-substitution, [...] function calls, {...} literal grouping, nested evaluation, registers, iter tokens
- **Attribute flags** with per-instance flag support via `@set obj/attr=[!]flagname` and `@lock/attr`/`@unlock/attr`
- **Permission system** with Controls, Examinable, Wizard, WizRoy, CanReadAttr, CanSetAttr
- **Parent chain inheritance** for attributes (up to 10 levels)
- **Flag system** with the standard TinyMUSH flag set
- **Power system** with @power command
- **Guest system** with configurable prefixes/suffixes
- **Spellcheck** with hybrid dictionary (base + learned + LanguageTool API)
- **Per-player message markers** for MUD client integration
- **@program** (interactive input capture)
- **Boolean lock evaluation** with indirect, carry, is, and owner lock types
- **Zone security** with CheckZoneForPlayer, CONTROL_OK/A_LCONTROL enforcement
- **Side-effect functions**: create(), set(), tel(), link(), trigger(), wipe(), force(), wait(), pemit(), remit(), oemit(), think()
- **ANSI 256-color / TrueColor** support via %x substitutions and ansi() function
- **Docker deployment** with docker-compose
- **SQLite3 SQL module** with sql(), sqlescape(), @sql, @sqlinit, @sqldisconnect
- **SSL/TLS** with dual cleartext/TLS listeners on independent ports

For detailed documentation on new features, see [docs/FEATURES.md](docs/FEATURES.md). For the flight/navigation system, see [docs/FLIGHT.md](docs/FLIGHT.md).

### Not Yet Implemented

- Mail system (@mail)
- IPv6

## Building from Source

```bash
go build -o gotinymush ./cmd/server
```

On Windows this produces `gotinymush.exe` automatically.

## Docker

The Docker image includes the seed database and all config files. First-time setup:

```bash
# Build and start (imports seed database automatically)
docker compose up -d --build

# Set the God password
docker compose exec gotinymush sh -c 'MUSH_GODPASS=mynewpassword gotinymush -bolt /game/data/game.bolt -conf /game/data/game.yaml'

# Connect
telnet localhost 6250
```

To import your own TinyMUSH database instead of the seed:

```yaml
# docker-compose.yml — add under volumes:
volumes:
  - ./mygame.FLAT:/game/data/mygame.FLAT
# and set under environment:
environment:
  MUSH_DB: /game/data/mygame.FLAT
```

## Command-Line Reference

```
gotinymush -conf <config.yaml> -db <flatfile> -bolt <database.bolt> [options]
```

| Flag | Environment Variable | Description |
|---|---|---|
| `-conf` | `MUSH_CONF` | Path to YAML game config |
| `-db` | `MUSH_DB` | Path to TinyMUSH flatfile (for initial import) |
| `-bolt` | `MUSH_BOLT` | Path to bbolt database (enables persistence) |
| `-import` | `MUSH_IMPORT=true` | Force reimport from flatfile into bbolt |
| `-restore` | `MUSH_RESTORE` | Restore from archive before boot |
| `-godpass` | `MUSH_GODPASS` | Set God (#1) password and exit (use env var for security) |
| `-port` | `MUSH_PORT` | Override listen port |
| `-textdir` | `MUSH_TEXTDIR` | Path to text files directory |
| `-aliasconf` | `MUSH_ALIASCONF` | Path to alias config file(s), comma-separated |
| `-comsysdb` | `MUSH_COMSYSDB` | Path to mod_comsys.db for channel import |
| `-dictdir` | `MUSH_DICTDIR` | Path to dictionary directory for spellcheck |
| `-sqldb` | `MUSH_SQLDB` | Path to SQLite3 database file |
| `-fresh` | `MUSH_FRESH=true` | Delete bolt DB on startup for clean reimport |
| `-tls-cert` | `MUSH_TLS_CERT` | Path to TLS certificate file |
| `-tls-key` | `MUSH_TLS_KEY` | Path to TLS private key file |
| `-tls-port` | `MUSH_TLS_PORT` | TLS listen port (default: port+1) |
| | `MUSH_TLS=true` | Enable TLS listener |
| | `MUSH_CLEARTEXT=false` | Disable cleartext listener (default: true) |
| | `MUSH_SPELLCHECK=true` | Enable spellcheck functions |
| | `MUSH_SQL=true` | Enable SQL functions |
| | `MUSH_ARCHIVE_DIR` | Archive output directory |
| | `MUSH_ARCHIVE_INTERVAL` | Auto-archive interval in minutes |
| | `MUSH_ARCHIVE_RETAIN` | Keep last N archives |
| | `MUSH_ARCHIVE_HOOK` | Shell command after archive |

Environment variables are used as defaults when flags are not provided. Command-line flags always take priority.

## Backups

### Manual backup

```
@archive
```

Creates a `.tar.gz` in the archive directory (default: `backups/`) containing the bolt database, config, text files, and dictionary. Only Wizard players can run this.

### Scheduled backups

In `game.yaml`:

```yaml
archive_dir: backups
archive_interval: 60    # every 60 minutes
archive_retain: 24      # keep last 24 archives
archive_hook: "scp %f user@backup-host:/backups/"  # optional post-archive command
```

### Restore from backup

```bash
./gotinymush -restore backups/archive-20260214-120000.tar.gz -bolt data/game.bolt -conf data/game.yaml
```

This validates checksums, restores the database, and prompts before overwriting config files that differ.

## TLS/SSL

To enable encrypted connections:

```yaml
# game.yaml
tls: true
tls_port: 6251
tls_cert: data/cert.pem
tls_key: data/key.pem
```

Or via environment variables:

```bash
MUSH_TLS=true MUSH_TLS_CERT=cert.pem MUSH_TLS_KEY=key.pem ./gotinymush -conf data/game.yaml -bolt data/game.bolt
```

Both cleartext (port 6250) and TLS (port 6251) listeners run simultaneously by default. Set `cleartext: false` in config to disable the plaintext listener.

## Testing

```bash
# Run all Go tests
go test ./...

# Run batch eval tests
go run ./cmd/evaltest -batch tests/eval_basic.txt

# Interactive eval testing
go run ./cmd/evaltest -db game.FLAT -player 1
> [add(1,2)]
3
> [iter(a b c,[ucstr(##)])]
A B C
```

## Project Structure

```
cmd/
  server/       Main server entry point
  evaltest/     Interactive softcode evaluator for testing
  dbloader/     Standalone database loader/inspector
pkg/
  archive/      Archive/backup/restore system
  eval/         Softcode evaluation engine (exec, %-subs, functions)
  flatfile/     TinyMUSH flatfile parser
  boltstore/    bbolt persistence layer
  gamedb/       Database types (Object, DBRef, flags, attributes)
  server/       TCP server, commands, help system, softcode queue
  crypt/        DES password hashing (TinyMUSH compat)
data/
  game.yaml             Example game configuration
  goTinyAlias.conf      Command/flag/function/attribute aliases
  text/                 Help files, connect screens, MOTD
tests/
  eval_basic.txt        Batch softcode evaluation tests
```

## License & Credits

GoTinyMUSH is distributed under the [Artistic License 1.0](https://opensource.org/licenses/Artistic-1.0), the same license as TinyMUSH 3.3. See [LICENSE](LICENSE) for full details and upstream copyright notices.

For a complete list of contributors across the TinyMU* family of servers, see [CREDITS](CREDITS).
