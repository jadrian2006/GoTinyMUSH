# Configuration Reference

GoTinyMUSH is configured through a YAML file (typically `data/game.yaml`), command-line flags, and environment variables. All three layers are composable: environment variables provide defaults, the config file overrides those, and CLI flags take final priority.

## Configuration File

The primary configuration file is a YAML file passed via `-conf` (or `MUSH_CONF`). The server auto-detects format by extension:

- `.yaml` / `.yml` — YAML format (recommended)
- `.conf` / other — Legacy TinyMUSH text format (`key value` pairs, `#` comments, `include` directives)

Legacy `.conf` files are fully supported for migration. The server parses `include` directives recursively (up to 10 levels deep) and recognizes `@attribute/access` runtime directives.

### Minimal Example

```yaml
mud_name: MyMUSH
port: 6250
player_starting_room: 0
master_room: 2
```

## Environment Variables

Environment variables serve as defaults when the corresponding flag or config key is not set. Every CLI flag has a matching environment variable.

| Environment Variable | Description | Default |
|---|---|---|
| `MUSH_CONF` | Path to YAML/conf config file | (none) |
| `MUSH_DB` | Path to TinyMUSH `.FLAT` flatfile for import | (none) |
| `MUSH_BOLT` | Path to bbolt database file | (none) |
| `MUSH_IMPORT` | Set `true` to force reimport from flatfile | `false` |
| `MUSH_PORT` | TCP listen port | 6250 |
| `MUSH_TEXTDIR` | Path to text files directory | (none) |
| `MUSH_ALIASCONF` | Path to alias config file(s), comma-separated | (none) |
| `MUSH_COMSYSDB` | Path to `mod_comsys.db` for channel import | (none) |
| `MUSH_DICTDIR` | Path to dictionary directory for spellcheck | (none) |
| `MUSH_SQLDB` | Path to SQLite3 database file | (none) |
| `MUSH_FRESH` | Set `true` to delete bolt DB on startup for clean reimport | `false` |
| `MUSH_GODPASS` | Set God (#1) password at startup | (none) |
| `MUSH_DEBUG` | Set `true` to enable debug logging | `false` |
| `MUSH_RESTORE` | Path to archive to restore before boot | (none) |
| `MUSH_TLS` | Set `true` to enable TLS listener | `false` |
| `MUSH_TLS_CERT` | Path to TLS certificate file | (none) |
| `MUSH_TLS_KEY` | Path to TLS private key file | (none) |
| `MUSH_TLS_PORT` | TLS listen port | main port + 1 |
| `MUSH_CLEARTEXT` | Set `false` to disable cleartext listener | `true` |
| `MUSH_SPELLCHECK` | Set `true` to enable spellcheck functions | `false` |
| `MUSH_SQL` | Set `true` to enable SQL functions | `false` |
| `MUSH_ARCHIVE_DIR` | Archive output directory | `backups` |
| `MUSH_ARCHIVE_INTERVAL` | Auto-archive interval in minutes | 0 (disabled) |
| `MUSH_ARCHIVE_RETAIN` | Keep last N archives | 0 (unlimited) |
| `MUSH_ARCHIVE_HOOK` | Shell command after archive (`%f` = archive path) | (none) |
| `MUSH_SEEDDIR` | Seed files directory (Docker) | `/game/seed` |
| `MUSH_ADMIN_PASS` | Admin panel password (Docker setup mode) | (none) |
| `MUSH_DICTURL` | LanguageTool API URL override | (none) |

**Security note:** Use `MUSH_GODPASS` (environment variable) instead of `-godpass` (flag), because command-line arguments are visible in process listings (`ps`, Task Manager).

## Command-Line Flags

```
gotinymush -conf <config.yaml> -db <flatfile> -bolt <database.bolt> [options]
```

| Flag | Env Equivalent | Description |
|---|---|---|
| `-conf` | `MUSH_CONF` | Path to YAML game config |
| `-db` | `MUSH_DB` | Path to TinyMUSH flatfile (initial import) |
| `-bolt` | `MUSH_BOLT` | Path to bbolt database (persistence) |
| `-import` | `MUSH_IMPORT=true` | Force reimport from flatfile into bbolt |
| `-port` | `MUSH_PORT` | Override listen port |
| `-textdir` | `MUSH_TEXTDIR` | Path to text files directory |
| `-aliasconf` | `MUSH_ALIASCONF` | Alias config file(s), comma-separated |
| `-comsysdb` | `MUSH_COMSYSDB` | Path to `mod_comsys.db` for channel import |
| `-dictdir` | `MUSH_DICTDIR` | Dictionary directory for spellcheck |
| `-sqldb` | `MUSH_SQLDB` | SQLite3 database file |
| `-fresh` | `MUSH_FRESH=true` | Delete bolt DB on startup for clean reimport |
| `-godpass` | `MUSH_GODPASS` | Set God (#1) password at startup |
| `-restore` | `MUSH_RESTORE` | Restore from archive before boot |
| `-tls-cert` | `MUSH_TLS_CERT` | TLS certificate file |
| `-tls-key` | `MUSH_TLS_KEY` | TLS private key file |
| `-tls-port` | `MUSH_TLS_PORT` | TLS listen port |
| `-debug` | `MUSH_DEBUG=true` | Enable debug logging |

**Priority:** CLI flags > config file > environment variables > compiled defaults.

---

## Configuration Sections

All keys below are YAML keys for `game.yaml`. The YAML key names are identical to legacy `.conf` directive names (underscore-separated, lowercase).

### Identity

| Key | Type | Default | Description |
|---|---|---|---|
| `mud_name` | string | `GoTinyMUSH` | Name shown in WHO list and logs |
| `port` | int | `6250` | TCP listen port |

### Key Rooms

| Key | Type | Default | Description |
|---|---|---|---|
| `master_room` | int | `2` | Global command room (dbref number) |
| `player_starting_room` | int | `0` | Room where new players appear |
| `player_starting_home` | int | `0` | Default home for new players |
| `default_home` | int | `0` | Fallback home when a player's home is destroyed |

### Economy

| Key | Type | Default | Description |
|---|---|---|---|
| `money_name_singular` | string | `penny` | Singular currency name |
| `money_name_plural` | string | `pennies` | Plural currency name |
| `starting_money` | int | `150` | Currency given to new players |
| `paycheck` | int | `50` | Currency awarded per connect |
| `earn_limit` | int | `10000` | Max currency before paycheck stops |
| `page_cost` | int | `0` | Cost to page another player |
| `wait_cost` | int | `10` | Cost of `@wait` command |
| `link_cost` | int | `1` | Cost to link an exit |
| `create_min_cost` | int | `10` | Minimum cost to `@create` an object |
| `create_max_cost` | int | `505` | Maximum cost to `@create` an object |
| `dig_cost` | int | `10` | Cost to `@dig` a room |
| `open_cost` | int | `1` | Cost to `@open` an exit |
| `robot_cost` | int | `1000` | Cost to create a robot player |
| `sacrifice_adjust` | int | `-1` | Adjustment to sacrifice value |
| `sacrifice_factor` | int | `5` | Divisor for sacrifice value |
| `kill_min` | int | `10` | Min cost to attempt `kill` |
| `kill_max` | int | `100` | Max cost to attempt `kill` |
| `kill_guarantee` | int | `100` | Cost for guaranteed `kill` success |
| `pay_limit` | int | `10000` | Max pennies before insurance revoked |

### Idle / Timeout / Network

| Key | Type | Default | Description |
|---|---|---|---|
| `idle_timeout` | int | `3600` | Seconds before idle disconnect (0 = never) |
| `idle_wiz_dark` | bool | `false` | Auto-DARK wizards when idle |
| `keepalive_interval` | int | `60` | Seconds between IAC NOP keepalives (0 = disabled) |

### Queue / Eval Engine

| Key | Type | Default | Description |
|---|---|---|---|
| `queue_idle_chunk` | int | `3` | Queue entries processed per idle cycle |
| `function_invocation_limit` | int | `2500` | Max function calls per evaluation |
| `machine_command_cost` | int | `64` | Queue cost for machine-generated commands |
| `iter_limit` | int | `10000` | Max iterations per `iter`/`parse`/`map`/`filter` |
| `eval_time_limit` | int | `30` | Wall-clock seconds per queue entry (safety timeout) |
| `command_invocation_limit` | int | `2500` | Max commands per queue entry chain |
| `eval_output_limit` | int | `1048576` | Max output bytes per eval (1 MB) |
| `dolist_limit` | int | `10000` | Max elements per `@dolist` |

### Output

| Key | Type | Default | Description |
|---|---|---|---|
| `output_limit` | int | `16384` | Max bytes per output message to a player |

### Quotas

| Key | Type | Default | Description |
|---|---|---|---|
| `quotas` | bool | `false` | Enable object quota enforcement |
| `typed_quotas` | bool | `false` | Track quotas per object type |
| `start_quota` | int | `20` | Initial overall quota for new players |
| `start_room_quota` | int | `20` | Initial room quota |
| `start_exit_quota` | int | `20` | Initial exit quota |
| `start_thing_quota` | int | `20` | Initial thing quota |
| `start_player_quota` | int | `20` | Initial robot player quota |
| `room_quota` | int | `1` | Quota cost per room |
| `exit_quota` | int | `1` | Quota cost per exit |
| `thing_quota` | int | `1` | Quota cost per thing |
| `player_quota` | int | `1` | Quota cost per robot player |

### Permissions

| Key | Type | Default | Description |
|---|---|---|---|
| `match_own_commands` | bool | `false` | Objects match `$`-commands on themselves |
| `player_match_own_commands` | bool | `false` | Players match `$`-commands on themselves |
| `pemit_far_players` | bool | `false` | Allow `@pemit` to players in other rooms |
| `pemit_any_object` | bool | `false` | Allow `@pemit` to any object (not just players) |
| `examine_public_attrs` | bool | `true` | Non-owners can see VISUAL attributes |
| `public_flags` | bool | `true` | Non-owners can see object flags |
| `read_remote_name` | bool | `false` | Allow `name()` on objects in other rooms |
| `require_cmds_flag` | bool | `true` | Objects need COMMANDS flag to match `$`-commands |
| `switch_default_all` | bool | `true` | `@switch` default behavior (see behavioral changes) |
| `sweep_dark` | bool | `false` | `@sweep` reveals DARK listeners |
| `trace_topdown` | bool | `true` | TRACE output in top-down order |
| `trace_output_limit` | int | `200` | Max lines of TRACE output |

### Default Flags for New Objects

| Key | Type | Default | Description |
|---|---|---|---|
| `player_default_flags` | [3]int | `[0,0,0]` | Default flag words for new players |
| `room_default_flags` | [3]int | `[0,0,0]` | Default flag words for new rooms |
| `exit_default_flags` | [3]int | `[0,0,0]` | Default flag words for new exits |
| `thing_default_flags` | [3]int | `[0,0,0]` | Default flag words for new things |
| `robot_default_flags` | [3]int | `[0,0,0]` | Default flag words for new robots |

### Object Destruction

| Key | Type | Default | Description |
|---|---|---|---|
| `instant_recycle` | bool | `true` | DESTROY_OK things skip GOING state, destroy immediately |

### Guest System

| Key | Type | Default | Description |
|---|---|---|---|
| `guest_char_num` | int | `-1` | Dbref of guest template character (-1 = disabled) |
| `guest_prefixes` | string | (none) | Space-separated name prefixes for guest characters |
| `guest_suffixes` | string | (none) | Suffix appended to guest names |
| `guest_basename` | string | `Guest` | Base name for guest characters |
| `number_guests` | int | `30` | Max simultaneous guest connections |
| `guest_password` | string | `guest` | Default guest password |
| `guest_start_room` | int | `-1` | Starting room for guests (-1 = use `player_starting_room`) |

### Module Toggles

| Key | Type | Default | Description |
|---|---|---|---|
| `mail_enabled` | bool | `true` | Built-in `@mail` system |
| `comsys_enabled` | bool | `true` | Channel/comsys system |
| `mail_expiration` | int | `14` | Days before mail auto-expires (0 = never) |
| `hooks_enabled` | bool | `true` | `@hook` command pre/post/override system |
| `instances_enabled` | bool | `true` | `@instance` vehicle/container system |
| `sensory_enabled` | bool | `true` | `smell`/`touch`/`taste`/`listen` commands |
| `roomformat_enabled` | bool | `true` | `@roomformat` custom room rendering |
| `multizone_enabled` | bool | `true` | `@chzone/add`, `@chzone/remove`, `zones()` |
| `mogrifier_enabled` | bool | `true` | Channel message mogrifiers |

### Channels

| Key | Type | Default | Description |
|---|---|---|---|
| `public_channel` | string | (none) | Default public channel name |
| `public_calias` | string | (none) | Default public channel alias |
| `guests_channel` | string | (none) | Default channel for guest players |
| `guests_calias` | string | (none) | Guest channel alias |

### Security

| Key | Type | Default | Description |
|---|---|---|---|
| `god_dbref` | int | `1` | Dbref of the God player |
| `zone_nest_limit` | int | `20` | Max zone recursion depth |

### TLS / SSL

| Key | Type | Default | Description |
|---|---|---|---|
| `cleartext` | bool | `true` | Enable cleartext (non-TLS) listener |
| `tls` | bool | `false` | Enable TLS listener |
| `tls_port` | int | port + 1 | TLS listen port |
| `tls_cert` | string | (none) | Path to TLS certificate file |
| `tls_key` | string | (none) | Path to TLS private key file |

When `tls: true`, both `tls_cert` and `tls_key` must be set or the server will refuse to start. Both cleartext and TLS listeners can run simultaneously.

### Spellcheck

| Key | Type | Default | Description |
|---|---|---|---|
| `spellcheck_enabled` | bool | `false` | Enable `spell()`/`spellcheck()` functions |
| `spellcheck_url` | string | `https://api.languagetool.org/v2/check` | LanguageTool API endpoint |

Also requires `MUSH_DICTDIR` to point to a dictionary directory (e.g., `data/dict`).

### SQL

| Key | Type | Default | Description |
|---|---|---|---|
| `sql_enabled` | bool | `false` | Enable `sql()`/`sqlescape()` functions |
| `sql_database` | string | (none) | Path to SQLite3 database file |
| `sql_query_limit` | int | `100` | Max rows returned per query |
| `sql_timeout` | int | `5` | Query timeout in seconds |
| `sql_reconnect` | bool | `true` | Auto-reconnect on failure |

### Archive / Backup

| Key | Type | Default | Description |
|---|---|---|---|
| `archive_dir` | string | `backups` | Archive output directory |
| `archive_interval` | int | `0` | Auto-archive interval in minutes (0 = disabled) |
| `archive_retain` | int | `0` | Keep last N archives (0 = unlimited) |
| `archive_hook` | string | (none) | Shell command after archive (`%f` = archive path) |

### Web Server / REST API

| Key | Type | Default | Description |
|---|---|---|---|
| `web_enabled` | bool | `true` | Enable HTTPS/WebSocket server |
| `web_port` | int | `8443` | Web server listen port |
| `web_host` | string | (all interfaces) | Bind address |
| `web_domain` | string | (none) | Domain for automatic Let's Encrypt certificates |
| `web_static_dir` | string | `web/dist` | Path to built web client |
| `web_client_url` | string | (none) | URL of external web client container for reverse proxy |
| `web_cors_origins` | list | `[]` | Allowed CORS origins (empty = same-origin only) |
| `web_rate_limit` | int | `120` | Max requests per minute per IP |
| `jwt_secret` | string | (auto-generated) | JWT signing secret |
| `jwt_expiry` | int | `86400` | JWT token lifetime in seconds (24 hours) |
| `cert_dir` | string | (none) | Directory for generated TLS certificates |
| `scrollback_retention` | int | `86400` | Public channel scrollback retention in seconds |

### Pueblo

| Key | Type | Default | Description |
|---|---|---|---|
| `pueblo_enabled` | bool | `false` | Enable Pueblo HTML protocol |
| `pueblo_version` | string | `This world is Pueblo 1.0 enhanced` | Pueblo version string |

### Compatibility

| Key | Type | Default | Description |
|---|---|---|---|
| `c_is_command` | bool | `false` | `%c` substitutes current command (true) or literal (false) |
| `fix_escape_eval` | bool | `true` | Strip double-escaped sequences in queued attrs |

### Function Access Control

The `function_access` map restricts softcode function access by privilege level:

```yaml
function_access:
  FORCE: wizard
  WIPE: wizard
  BEEP: wizard
  SQL: god
```

Valid access levels: `public`, `wizard`, `god`, `disabled`.

### Attribute Configuration

| Key | Type | Description |
|---|---|---|
| `user_attr_access` | string | Default flags for user-defined attributes |
| `attr_types` | list | Pattern-based attribute flag assignment |
| `attr_access` | list | `@attribute/access` directives |

### Alias Files

```yaml
alias_files:
  - goTinyAlias.conf
```

Paths are resolved relative to the config file's directory. These files define command aliases, flag aliases, function aliases, attribute aliases, power aliases, and forbidden player names.

---

## Common Configuration Examples

### Production Game Server

```yaml
mud_name: CrystalMUSH
port: 6886
master_room: 2
player_starting_room: 100

idle_timeout: 7200
keepalive_interval: 60

tls: true
tls_port: 6887
tls_cert: /etc/letsencrypt/live/mush.example.com/fullchain.pem
tls_key: /etc/letsencrypt/live/mush.example.com/privkey.pem

web_enabled: true
web_port: 8443
web_domain: mush.example.com

archive_dir: /backups/mush
archive_interval: 60
archive_retain: 168
archive_hook: "rsync %f backup-host:/mush-backups/"

comsys_enabled: true
mail_enabled: true
mail_expiration: 30

function_invocation_limit: 5000
iter_limit: 20000
```

### Development / Testing

```yaml
mud_name: DevMUSH
port: 6250
idle_timeout: 0

web_enabled: true
web_port: 8443

sql_enabled: true
sql_database: data/game.sqlite3

spellcheck_enabled: true
```

### TLS-Only (No Cleartext)

```yaml
port: 6250
cleartext: false
tls: true
tls_port: 6251
tls_cert: data/cert.pem
tls_key: data/key.pem
```
