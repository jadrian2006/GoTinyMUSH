# GoTinyMUSH New Features

Features added to GoTinyMUSH beyond the original TinyMUSH 3.3 baseline, or significant improvements to existing functionality.

## Per-Player Message Markers

Players can set attributes that wrap incoming messages with custom delimiters, enabling MUD clients (BeipMU, TinyFugue, MUSHclient, Mudlet) to match patterns and route messages to separate windows.

**Setting a marker:**
```
&MARKER_<type> me=<open>|<close>
```

**Supported message types:** `SAY`, `POSE`, `PAGE`, `WHISPER`, `EMIT`
**Channel messages:** Use the channel name as the type (e.g. `MARKER_Public`)

**Examples:**
```
&MARKER_POSE me=>>>POSE>|<<<
&MARKER_SAY me=[SAY]|[/SAY]
&MARKER_PAGE me=---PAGE---|---
&MARKER_Public me=[PUB]|[/PUB]
```

With `MARKER_POSE` set to `>>>POSE>|<<<`:
```
>>>POSE>Wizard stretches.<<<
```

The `|` separates opening and closing markers. If no `|` is present, the entire value is used as a prefix. Values are raw text, not evaluated as softcode. Markers only affect what the setting player receives. To remove a marker, clear the attribute: `&MARKER_POSE me=`

See: `help MARKERS` in-game.

---

## @program (Interactive Input)

Captures a player's next line of input and executes a stored attribute with the input available as `%0`. Used for building interactive menus, form-fill systems, and multi-step wizards in softcode.

**Usage:**
```
@program <player>=<obj>/<attr>
```

The target player's next input line (that doesn't start with `|` or `@quitprogram`) is passed as `%0` to the specified attribute, which is executed in the queue.

**Escape mechanisms:**
- Lines starting with `|` bypass the program and execute as normal commands (e.g. `|look`)
- `@quitprogram` cancels the active program

**Example:**
```
&ASK_NAME me=$ask name:@pemit %#=What is your character's name?;@program %#=me/HANDLE_NAME
&HANDLE_NAME me=@pemit %#=You said your name is: %0
```

See: `help @program` in-game.

---

## Improved Boolean Expression (Lock) Evaluation

The lock system now correctly handles compound lock types with Sub1 node structure:

- **Indirect locks** (`@#dbref`): Fetches the LOCK attribute from the referenced object and evaluates it recursively
- **Carry locks** (`+#dbref` or `+attr:pattern`): Tests whether the player carries the specified object, or whether any carried object matches an attribute pattern
- **Is locks** (`=#dbref` or `=attr:pattern`): Tests exact identity match, or whether the player's attribute matches a pattern
- **Owner locks** (`$#dbref`): Tests whether the player's owner matches the referenced object's owner

Attribute-based carry and is locks (`+attr:pattern`, `=attr:pattern`) now correctly search inventory contents and player attributes respectively.

---

## Zone Security Improvements

- **CheckZoneForPlayer**: Zone control lock checks now work correctly for player objects, checking CONTROL_OK on the zone master object and evaluating A_LCONTROL
- **StripPrivFlags**: When changing zones, privilege flags (IMMORTAL, INHERIT, ROYALTY, WIZARD, STAFF, etc.) and all powers are stripped from non-player objects, matching TinyMUSH 3.3 behavior

---

## SQLite3 SQL Module

Full SQLite3 integration for softcode and wizard use.

**Softcode functions:**
- `sql(<query>,<row-delim>,<field-delim>)` - Execute SQL queries from softcode (requires `use_sql` power)
- `sqlescape(<text>)` - Escape strings for safe SQL interpolation

**Wizard commands:**
- `@sql <query>` - Interactive SQL query tool with row/field display
- `@sqlinit` - Re-initialize SQL connection (God-only)
- `@sqldisconnect` - Close SQL connection (God-only)

Enable with `-sqldb <path>` flag or `MUSH_SQL=true` + `MUSH_SQLDB=<path>` environment variables.

See: [SQL Documentation](SQL.md)

---

## Spellcheck System

Hybrid spellcheck with local dictionary and optional remote LanguageTool API.

**Softcode functions:**
- `spell(<text>)` - Check spelling, returns ANSI-highlighted text (red underline for misspellings)
- `spell(<text>,g)` - Check spelling and grammar (cyan underline for grammar issues)
- `spellcheck(<text>)` - Returns structured error data instead of highlighted text

**Features:**
- Base dictionary + learned words file (auto-populated from API validation)
- Per-object `DICTIONARY` attributes for custom word lists
- ANSI flag-aware: only adds color codes if the player has the ANSI flag set

Enable with `MUSH_SPELLCHECK=true` and optionally `MUSH_DICTURL=<languagetool-url>`.

---

## SSL/TLS Support

Dual-listener architecture with independent plaintext and TLS listeners on separate ports.

**Configuration (game.yaml):**
- `cleartext: true/false` — enable/disable plaintext listener (default: true)
- `tls: true/false` — enable/disable TLS listener (default: false)
- `tls_port: 6251` — TLS port (default: main port + 1)
- `tls_cert` / `tls_key` — paths to certificate and key files

**Command-line flags:** `-tls-cert`, `-tls-key`, `-tls-port`

**Environment variables:** `MUSH_TLS`, `MUSH_CLEARTEXT`, `MUSH_TLS_PORT`, `MUSH_TLS_CERT`, `MUSH_TLS_KEY`

Both listeners can run simultaneously, or either can be disabled independently. TLS uses Go's `crypto/tls` with standard PEM certificate/key files. Generate a self-signed cert for testing:

```bash
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
```

Test with: `openssl s_client -connect localhost:6251`

---

## Structure/Instance System

Typed data structures for softcode, matching TinyMUSH 3.x's funvars system.

**Defining structures:**
```
think [structure(player, name level hp mp, s i i i, Anonymous 1 100 50, |)]
```
Defines a `player` structure with 4 typed components (string, integer, integer, integer), defaults, and `|` output delimiter.

**Type system:** `a` (any), `c` (char), `d` (dbref), `i` (integer), `f` (float), `s` (string/no-spaces)

**Creating and using instances:**
```
think [construct(pc1, player, name level, Bob 5)]
think [z(pc1, name)]    -> Bob
think [z(pc1, hp)]      -> 100  (default)
think [modify(pc1, level hp, 6 95)]
think [unload(pc1)]     -> Bob|6|95|50
```

**Persistence via attributes:**
```
think [write(me/PC1_DATA, pc1)]     -> saves to attribute with form-feed delimiters
think [read(me/PC1_DATA, pc1_copy, player)]  -> loads from attribute
```

**Functions:** `structure()`, `construct()`, `destruct()`, `unstructure()`, `z()`, `modify()`, `load()`, `unload()`, `read()`, `write()`, `delimit()`, `lstructures()`, `linstances()`, `store()`, `items()`

Per-player namespaced, mutex-protected, with reference counting (can't delete structures with active instances).

---

## Flight/Navigation System

Softcode functions for 3D grid-based flight and navigation, supporting a 4-quadrant coordinate system with 32-point compass headings. For comprehensive documentation including grid maps, the full heading reference table, and softcode examples for building flight loops, autopilot, radar, and combat systems, see [FLIGHT.md](FLIGHT.md).

**Heading system (32-point compass, 0=East counterclockwise):**
```
think [hvec(0)]        -> 1 0        (East unit vector)
think [hvec(8)]        -> 0 1        (North unit vector)
think [hname(4)]       -> NE
think [hdelta(0, 12)]  -> 12         (turn left 12 points from E to NW)
think [h2deg(8)]       -> 90         (North = 90 degrees)
think [vec2h(1, 1)]    -> 4          (NE direction)
```

**Grid coordinates (4-quadrant, AA-ZZ x 000-999):**
```
think [gridabs(EL-453-NE)]         -> 115 453
think [absgrid(115, 453)]          -> EL-453-NE
think [griddist(AA-0-NE, ZZ-999-NE)]  -> 1206.09
think [gridcourse(AA-0-NE, EL-453-NE)]  -> 6 469.33  (heading + distance)
```

**Navigation projection:**
```
think [gridnav(100 200 50, 0, 10)]      -> 110 200 50   (move East at speed 10)
think [gridnav(100 200 50, 8, 10, 5)]   -> 100 210 55   (move North, climb 5)
```

**Drift/entropy (random perturbation per tick):**
```
think [gridnav(100 200 50, 0, 10, 0, 3)]     -> 112.1 198.7 51.4  (move + drift ±3)
think [drift(100 200 50, 5 5 1)]              -> 103.2 197.8 50.4  (per-axis drift)
think [vrand(5)]                               -> -2.3 1.7 3.1     (random vector, mag 0-5)
```

**Multi-object tactical functions:**
```
think [bearing(100 200, 300 400)]              -> 4   (heading NE to face target)
think [pitch(100 200 50, 100 200 80)]          -> 90  (straight up)
think [closing(0 0, 0, 10, 100 0, 16, 10)]    -> 20  (head-on, closing at 20/tick)
think [closing(0 0, 0, 10, 100 0, 0, 10)]     -> -10 (both going East, separating)
think [eta(0 0, 0, 10, 100 0)]                -> 10  (10 ticks to reach target)
think [intercept(0 0, 15, 100 0, 8, 10)]      -> 6   (fly heading NNE to intercept)
think [relvel(0, 10, 16, 10)]                  -> -20 0  (relative velocity)
```

**Functions:** `hvec()`, `hdelta()`, `hname()`, `h2deg()`, `deg2h()`, `vec2h()`, `gridabs()`, `absgrid()`, `griddist()`, `gridcourse()`, `gridnav()`, `drift()`, `vrand()`, `vrandc()`, `bearing()`, `pitch()`, `closing()`, `relvel()`, `eta()`, `intercept()`

---

## Per-Player Message Markers

See above for full details. This is a GoTinyMUSH-original feature not present in TinyMUSH 3.3.
