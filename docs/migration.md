# Migration Guide: C TinyMUSH to GoTinyMUSH

This guide covers migrating an existing TinyMUSH 3.x game to GoTinyMUSH. The process is straightforward: GoTinyMUSH reads standard TinyMUSH `.FLAT` files directly and aims for full softcode compatibility. Most games will work without any code changes.

## Overview

| Aspect | C TinyMUSH 3.x | GoTinyMUSH |
|---|---|---|
| Language | C (~76,000 lines) | Go (~24,000 lines) |
| Database | GDBM/QDBM | bbolt (pure Go) |
| Config format | `.conf` text | YAML (also reads `.conf`) |
| Build | CMake + autoconf + libcrypt + GDBM | `go build` (single binary, no deps) |
| Deploy | Manual compile + shell scripts | Single binary or Docker |
| Password hashing | Unix `crypt(3)` | DES crypt (compatible, same hashes) |
| Help system | Binary `.indx` files + `mkindx` tool | Parsed from `.txt` at startup |

## Step 1: Export Your Database

On your C TinyMUSH server, create a flatfile dump:

```
@dump/flat
```

This creates a `.FLAT` file (typically `your_game.FLAT` or `your_game.FLAT.#`). Copy this file to your GoTinyMUSH installation directory.

If you also use the comsys (channel system), locate the `mod_comsys.db` file in your game directory for channel import.

## Step 2: Convert Configuration

GoTinyMUSH can read legacy `.conf` files directly, but YAML is recommended for new deployments. The key differences:

### Configuration Mapping: `.conf` to YAML

Most directives map directly (same key name, underscore-separated). Here are the notable changes:

| C TinyMUSH `.conf` | GoTinyMUSH YAML | Notes |
|---|---|---|
| `mud_name MyMUSH` | `mud_name: MyMUSH` | Same key |
| `port 6250` | `port: 6250` | Same key |
| `master_room 2` | `master_room: 2` | Same key |
| `include alias.conf` | `alias_files: [goTinyAlias.conf]` | Unified alias file |
| `include compat.conf` | (merged into goTinyAlias.conf) | Not needed separately |
| `have_pueblo 1` | `pueblo_enabled: true` | Both key names accepted in `.conf` |
| `module comsys` | `comsys_enabled: true` | Module system replaced by toggles |
| `module mail` | `mail_enabled: true` | Module system replaced by toggles |
| `helpfile ... ...` | `-textdir data/text` | Help parsed from `.txt` at startup |

**Keys that work identically in both formats:** `mud_name`, `port`, `master_room`, `player_starting_room`, `player_starting_home`, `default_home`, `money_name_singular`, `money_name_plural`, `starting_money`, `paycheck`, `earn_limit`, `idle_timeout`, `queue_idle_chunk`, `function_invocation_limit`, `output_limit`, and all economy/permission/guest keys.

### Automatic `.conf` Loading

You can use your existing `.conf` file as-is:

```bash
./gotinymush -conf netmush.conf -db mygame.FLAT -bolt game.bolt \
  -textdir data/text -aliasconf data/goTinyAlias.conf
```

GoTinyMUSH auto-detects the format by file extension. Files ending in `.conf` (or anything other than `.yaml`/`.yml`) are parsed as legacy TinyMUSH format with `include` directive support (up to 10 levels deep).

### Recommended: Convert to YAML

For a fresh YAML config based on your `.conf`, extract the values you care about and write a `game.yaml`. See [configuration.md](configuration.md) for the full reference with defaults.

## Step 3: Import the Database

### First-Time Import

```bash
./gotinymush -conf data/game.yaml -db mygame.FLAT -bolt data/game.bolt \
  -textdir data/text -aliasconf data/goTinyAlias.conf
```

This reads the `.FLAT` file, imports all objects and attributes into a bbolt database, and starts the server. The import preserves:

- All objects (rooms, exits, things, players, garbage)
- All attributes (including user-defined attribute names and flags)
- Object flags, powers, pennies, zones, parents, links, homes, locks
- Player passwords (DES crypt hashes are compatible; players log in without resetting)

### Subsequent Starts

After the first import, the bbolt database is the primary store. Omit `-db`:

```bash
./gotinymush -conf data/game.yaml -bolt data/game.bolt \
  -textdir data/text -aliasconf data/goTinyAlias.conf
```

### Importing Comsys Channels

If you have a `mod_comsys.db` file from C TinyMUSH:

```bash
./gotinymush -conf data/game.yaml -db mygame.FLAT -bolt data/game.bolt \
  -comsysdb mod_comsys.db -textdir data/text -aliasconf data/goTinyAlias.conf
```

Channels and player aliases are imported into bbolt and served from there on subsequent starts.

### Force Reimport

To reimport the flatfile into an existing bbolt database (overwrites all data):

```bash
./gotinymush -import -conf data/game.yaml -db mygame.FLAT -bolt data/game.bolt ...
```

Or use `-fresh` to delete the bolt database and reimport cleanly:

```bash
./gotinymush -fresh -conf data/game.yaml -db mygame.FLAT -bolt data/game.bolt ...
```

## Step 4: Verify Help Files

C TinyMUSH uses `mkindx` to build binary `.indx` index files from help text. GoTinyMUSH parses `.txt` files directly at startup -- no index files needed. Copy your `help.txt`, `wizhelp.txt`, `news.txt`, and other text files into the `-textdir` directory. They will be indexed in memory automatically.

Text files are also **hot-reloaded**: edit them while the server is running and changes take effect immediately.

## Step 5: Test and Validate

1. Connect as the Wizard/God character and verify basic operations:
   ```
   connect Wizard <password>
   @version
   look
   examine me
   @search type=room
   ```

2. Test softcode that your game relies on heavily.

3. Check `@ps` for queued actions and verify `@startup` fired correctly.

4. If using channels, verify they loaded: `@clist`, `comlist`.

5. If using mail, verify mailboxes loaded: `@mail`.

---

## Feature Parity

GoTinyMUSH implements the full TinyMUSH 3.x feature set. A comprehensive compatibility audit (696 function tests, 164 command tests, 52 mail tests) achieved **100% match** across all test suites. See [compatibility.md](compatibility.md) for the full audit report.

### What's New (Not in C TinyMUSH)

GoTinyMUSH adds significant functionality beyond C TinyMUSH 3.x. See [new-functions.md](new-functions.md) for the complete list. Highlights include:

- **170+ new softcode functions** from TinyMUSH 4.x, RhostMUSH, PennMUSH, and GoTinyMUSH originals
- **Event bus** (`publish`/`subscribe`/`unsubscribe`/`queues`) for pub/sub inter-object communication
- **JSON functions** (`json`, `json_query`, `json_mod`, `json_pp`, `json_test`)
- **Array system** (`array`, `apush`, `apop`, `aget`, `aset`, `alen`, etc.)
- **Flight/navigation system** (32-point compass, grid coordinates, drift, tactical intercept)
- **Structure/instance system** with persistence
- **Web interface** with WebSocket, REST API, JWT authentication
- **OOB protocols** (GMCP, MSDP, MCP)
- **SQL integration** (`sql()`, `sqlescape()`) via embedded SQLite3
- **Spellcheck** (`spell()`, `spellcheck()`) via LanguageTool API
- **Floating-point math** (`fadd`, `fsub`, `fmul`, `fdiv`)
- **Archive/backup system** with scheduled backups, retention, and restore
- **AI chatbot service** for LLM-driven NPC characters
- **Channel mogrifiers**, **multiple zones**, **@hook** command system
- **Sensory commands** (`smell`, `taste`, `touch`, `listen`)
- **@roomformat** custom room rendering
- **Per-player message markers** for MUD client automation
- **Connection history** (`connlog`, `addrlog`)
- **Full-text help search** (`textsearch`)
- **ANSI 256-color and TrueColor** support
- **Hot-reloading** text files (connect screens, help, MOTD)

### What C Had That GoTinyMUSH Does NOT Implement

The following C TinyMUSH features are **not implemented** in GoTinyMUSH. These are niche features that were rarely used or are superseded by better alternatives:

| C Feature | Status | Alternative |
|---|---|---|
| Tcl module (`mod_tcl`) | Not implemented | Use softcode or the REST API |
| mSQL module (`mod_msql`) | Not implemented | Use `sql()` with SQLite3 |
| `@cron` (scheduled tasks) | Not implemented | Use `@wait` loops or external cron with REST API |
| RWHO protocol | Not implemented | Obsolete; use WHO API endpoint |
| Pueblo HTML rendering | Partial; `pueblo_enabled` config exists | Pueblo protocol negotiation recognized but HTML rendering limited |
| DNS slave process | Not implemented | Go handles DNS natively (non-blocking) |
| Property directories | Not implemented | Use attributes with structured naming |
| `mkindx` help indexer | Not needed | Help files parsed at startup from `.txt` |
| GDBM/QDBM database | Replaced | bbolt (pure Go, no external dependency) |
| Binary `.indx` help files | Not needed | In-memory index from `.txt` files |
| `dbconvert` utility | Not needed | Direct `.FLAT` import; bbolt is self-contained |
| IPv6 listeners | Not yet implemented | Planned |
| `register_site` directive | Noted but not enforced | Use firewall rules or reverse proxy |

## Behavioral Differences

GoTinyMUSH corrects several inconsistencies in C TinyMUSH. These changes may affect existing softcode. See [behavioral-changes.md](behavioral-changes.md) for the full catalog. Key items:

### @switch: First-Match-Only by Default

C TinyMUSH's deferred `@switch` (in action lists, triggers, `@force`) silently behaved as `@switch/all`, matching every case. GoTinyMUSH uses first-match-only for both direct and deferred `@switch`.

**Action required:** Add `/all` to any `@switch` that relies on matching multiple cases.

### Integer Arithmetic Truncation

`add()`, `sub()`, `mul()` truncate results to integers (matching C's `ival()` behavior). Use `fadd()`, `fsub()`, `fmul()`, `fdiv()` for floating-point results. These are new functions not present in C.

### Lock Serialization

Mixed AND/OR lock expressions now correctly preserve parentheses during serialization. If you had locks that relied on the broken precedence, they will now evaluate differently (correctly).

### Stack Registers

`push()`/`pop()`/`peek()` are per-evaluation-context in Go (not persisted on a player attribute as in C). Stack data does not survive across separate evaluations.

### Leading Bracket Expressions

When a triggered attribute has `[setq(0,X)]@switch ...`, GoTinyMUSH correctly evaluates the bracket expression and then dispatches the command. C TinyMUSH would silently drop the command.

---

## Migration Checklist

Use this checklist for a complete migration:

- [ ] **Export flatfile** from C TinyMUSH (`@dump/flat`)
- [ ] **Copy flatfile** (`.FLAT`) and optionally `mod_comsys.db` to GoTinyMUSH host
- [ ] **Install GoTinyMUSH** (release binary, `go build`, or Docker)
- [ ] **Create/convert config** (`game.yaml` or use existing `.conf`)
- [ ] **Set up alias config** (use `data/goTinyAlias.conf` or customize)
- [ ] **Copy text files** (`help.txt`, `wizhelp.txt`, `news.txt`, `connect.txt`, `motd.txt`) to `-textdir`
- [ ] **Run initial import** (`-db mygame.FLAT -bolt game.bolt`)
- [ ] **Set God password** (`MUSH_GODPASS=... gotinymush -bolt game.bolt -conf game.yaml`)
- [ ] **Connect and verify** (login, `@version`, `look`, `examine me`)
- [ ] **Test critical softcode** (global commands, key systems)
- [ ] **Audit `@switch`** usage: add `/all` where multi-match was relied upon
- [ ] **Verify channels** (`@clist`) if comsys was imported
- [ ] **Verify mail** (`@mail`) if mail was enabled
- [ ] **Configure backups** (`archive_dir`, `archive_interval`, `archive_retain`)
- [ ] **Set up TLS** if needed (`tls: true`, cert/key paths)
- [ ] **Enable web interface** if desired (`web_enabled: true`)
- [ ] **Update DNS/firewall** for new server address and ports
- [ ] **Announce migration** to players; note behavioral changes
- [ ] **Remove C TinyMUSH** infrastructure (GDBM, `mkindx`, startup scripts)
