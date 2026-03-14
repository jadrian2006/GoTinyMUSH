# Behavioral Changes from C TinyMUSH

This document catalogs all known behavioral differences between GoTinyMUSH and C TinyMUSH 3.1/3.3, even in cases where both implementations are arguably "correct." These are not bugs -- they are architectural differences, intentional improvements, or areas where the specification is ambiguous.

For actual bugs found and fixed during the compatibility audit, see [compatibility.md](compatibility.md).

---

## Evaluation Engine

### Leading bracket expressions in triggered attributes

When a triggered attribute contains a leading bracket expression followed by a command:

```
&MY_ATTR obj = [setq(0,hello)]@switch 1=1,{@pemit %#=[r(0)]}
```

- **C**: Does not dispatch `@switch` -- the `[setq()]` prefix prevents the command dispatcher from recognizing the line as a command. The bracket expression evaluates but the command is treated as literal text.
- **Go**: Evaluates the bracket expression first, then dispatches the remaining text as a command. The `@switch` executes correctly.
- **Go behavior is correct.** Fixed in commit `01b6437`.

### Register scoping

**Stack registers (`push`/`pop`/`peek`):**
- **C**: Persists stack data on a player attribute (`A_STACK`). Stack survives server restarts and is shared across all evaluation contexts for that player.
- **Go**: Stack is per-`EvalContext`. It exists only for the duration of the current evaluation and is not persisted.

**`uprivate()`/`private()` register isolation:**
- **C**: Sets `rdata = NULL`, giving inner code a completely fresh, empty register space. Inner code cannot see any caller registers.
- **Go**: Clones the caller's registers. Inner code has read access to caller's register values (equivalent to `ulocal()` rather than true isolation).
- **Impact**: Softcode relying on `private()` to guarantee zero initial register state may see stale values in Go.

### `%#` in chained triggers

When object A triggers object B, which triggers object C:

- **C**: `%#` in C's triggered attribute may resolve to the intermediate THING (object B) rather than the original player who initiated the action.
- **Go**: `%#` preserves the original enactor (the player) throughout the entire trigger chain, regardless of depth.
- **Go behavior is correct** per the TinyMUSH specification. The enactor should always be the player who originated the action, not intermediate objects.

### Register sharing in `u()` calls

Both C and Go share register space between `u()` caller and callee:

```
[setq(p, outer)][u(me/FN)]  -- FN contains [setq(p, inner)]
-- After: r(p) = inner (clobbered)
```

This is documented C behavior, not a difference. However, it is a common source of bugs in softcode. Use unique register names across nested `u()` chains (e.g., `r(w)` in inner functions when outer uses `r(p)`).

---

## Queue Processing

### Dual-speed queue ticks

- **C**: Fixed tick rate for queue processing (typically 1 second).
- **Go**: Dual-speed system -- 1-second ticks when queue has active entries, 5-second ticks when idle. The idle rate reduces CPU usage when no commands are pending.
- **Impact**: Commands queued during idle periods may experience up to 5 seconds of latency before the queue wakes. The `WakeQueue` mechanism triggers immediate processing when new entries arrive.

### @wait timing

- **C**: `@wait 1=command` fires after approximately 1 tick cycle.
- **Go**: `@wait 1=command` fires within approximately 1.4 seconds (after WakeQueue fix). The slight overhead comes from the queue wake mechanism.

### @dolist/notify

- **C** and **Go**: Both correctly support `@dolist/notify` which sends a `@notify` after the last iteration completes, allowing semaphore-gated sequencing.
- No behavioral difference.

---

## Object Handling

### objid() for persistent identification

- **C 3.1**: Does not have `objid()`. Objects are identified only by dbref, which can be recycled.
- **Go**: `objid(obj)` returns a persistent identifier in `#dbref:generation` format that remains unique even after the dbref is recycled and reused.

### createtime() tracking

- **C 3.1**: Does not track creation time per object.
- **Go**: `createtime(obj)` returns the Unix timestamp when the object was created.

### lastcreate() semantics

- **C 3.3**: `lastcreate(obj, type)` returns the last-created object of the given type (R/E/T/P).
- **Go**: Now matches C 3.3 behavior (was previously returning a timestamp, fixed during audit).

---

## Security

### @mail/clear on safe messages

- **C**: Allows `@mail/clear` on messages marked as safe (a C bug -- safe should prevent clearing).
- **Go**: Correctly refuses to clear safe messages.
- **Go is correct.** The SAFE flag exists specifically to prevent accidental deletion.

### objeval permission enforcement

- **C**: `Cannot_Objeval()` requires same owner or wizard status; cannot evaluate as God (#1).
- **Go**: Now matches C behavior (was previously allowing unrestricted objeval, fixed during audit).

### Evaluation lock strictness

- **Go**: Some edge cases in evaluation lock enforcement are stricter than C. Specifically, attribute permission checks on `get()`/`xget()`/`get_eval()` and `hasattr()`/`hasattrp()` are now enforced where C was permissive.

---

## String Processing

### timefmt format codes

- **C 3.1**: Uses `$Y`, `$m`, `$d` etc. (dollar-sign prefixed format codes).
- **Go**: Accepts both `$Y` (C 3.1 style) and `%Y` (standard strftime style). This is a superset -- all C softcode works unchanged.

```
think timefmt($Y-%m-%d)   -- works in both
think timefmt(%Y-%m-%d)   -- works in Go only
```

### nescape first character

- **C**: `nescape()` escapes the first character of the string if it is a special character.
- **Go**: Matches C behavior -- escapes first character only.

### ansi() SGR code generation

- **C**: `ansi(hr, text)` produces combined SGR codes like `\e[1;31m`.
- **Go**: Matches C behavior -- produces the same combined SGR format.

### encrypt()/decrypt() algorithm

- **C**: Uses printable ASCII range (32-126), mod 95 arithmetic.
- **Go**: Now matches C algorithm (was previously using full byte range mod 256, fixed during audit).
- **Impact**: Encrypted strings are now interchangeable between C and Go servers.

---

## Formatting

### table() argument layout

- **C**: `table(list, field_width, line_length, output_sep, field_sep, pad_char)` with truncation when field content exceeds width.
- **Go**: Matches C argument layout with correct truncation behavior (fixed during audit).

### border/cborder/rborder

- **C 3.3**: `border(text, width[, fill_char])` -- word-wrapping paragraph formatter with left/right margin fills.
- **Go**: Single-line title bar formatter with fill character. Different function entirely from C's paragraph wrapper.

```
-- C:  border(Long paragraph text here, 20, =)  -> word-wrapped paragraph
-- Go: cborder(Title, 40, =)  -> =========== Title ============
```

Go's implementation is useful for header bars (common in MUSH output formatting) but does not replicate C's word-wrap paragraph mode. Use `wrap()` for paragraph formatting in Go.

### border prefix width accounting

- **Go**: `cborder()` and `rborder()` account for prefix text width per alignment mode, ensuring the total line width matches the requested width even with multi-character fill patterns.

---

## List Processing

### sort() autodetect

- **C**: `sort(list)` auto-detects whether the list contains numbers and sorts numerically if so.
- **Go**: Now matches C autodetect behavior (was previously defaulting to alpha sort, fixed during audit).

```
think sort(10 2 1)   -- Both: 1 2 10 (numeric autodetect)
think sort(a c b)    -- Both: a b c (alpha)
```

### setunion/setdiff/setinter output order

- **C**: Output is sorted alphabetically.
- **Go**: Output preserves input order (hash-map based).

```
think setunion(c a b, d)
-- C:  a b c d
-- Go: c a b d
```

Both produce correct set results; only the ordering differs.

### member()/remove() case sensitivity

- **C**: Case-sensitive comparison (`strcmp`).
- **Go**: Now matches C case-sensitive behavior (was previously using `EqualFold`, fixed during audit).

---

## Connection and WHO

### WHO/DOING format

- **Go**: Column layout and spacing in WHO/DOING output closely matches C but may have minor whitespace differences in edge cases (long player names, many connected players).
- **Impact**: Softcode that parses WHO output by fixed column positions should work, but pattern-matching-based parsing is more robust.

### conn()/idle() return format

- **C**: Returns integer seconds as a string.
- **Go**: Matches C return format.
- For disconnected players, both return `-1`.

---

## Database and Persistence

### Flatfile format

- **C**: Uses TinyMUSH flatfile database format with specific header markers.
- **Go**: Reads and writes the same flatfile format for compatibility. Databases can be loaded from C into Go.
- **Caveat**: Go-specific features (arrays, event bus subscriptions, API keys) are stored in the flatfile using Go-specific attribute markers that C will ignore on import.

### GOING/GARBAGE objects

- **C**: Objects marked GOING are destroyed on the next database dump cycle.
- **Go**: Matches C behavior for GOING flag and garbage collection timing.

---

## Networking

### Dual-listener architecture

- **C**: Single listener, optionally with SSL compiled in.
- **Go**: Independent plaintext and TLS listeners on separate ports, both configurable independently. Can run both simultaneously or either alone.

### LOGOUT command

- **C**: Not present. Players must `QUIT` (disconnect) to return to login screen.
- **Go**: `LOGOUT` returns the connection to the login screen without closing the TCP socket. The player can reconnect or connect as a different character without re-establishing the connection.

---

## Comsys (Channel System)

### Channel function availability

- **C 3.1**: Limited channel functions.
- **Go**: Full comsys function suite: `cinfo()`, `cwho()`, `cwhoall()`, `comowner()`, `comdesc()`, `comheader()`, `comalias()`, `cominfo()`, `comtitle()`, `cemit()`, `comlist()`.
- All C-compatible functions produce matching output format.

---

## Error Messages

### Format differences

Many error messages have slightly different wording between C and Go. Common patterns:

| Situation | C | Go |
|-----------|---|-----|
| Object not found | `#-1 NO MATCH` | `#-1 NOT FOUND` |
| Bad type | `#-1 NO SUCH TYPE` | `#-1 NOT FOUND` |
| Permission denied | `#-1 PERMISSION DENIED` | `#-1 PERMISSION DENIED` |
| Division by zero | (returns 0 silently) | `#-1 DIVIDE BY ZERO` |

Go generally prefers explicit error messages where C sometimes fails silently. Softcode that checks for `#-1` prefix (e.g., `strmatch(result, #-*)`) will work correctly with both servers.

---

## Summary of Impact on Existing Softcode

For softcode migrating from C TinyMUSH to GoTinyMUSH:

1. **Most softcode works unchanged.** The 696/696 function test pass rate confirms this.
2. **Stack-based softcode** may behave differently if it relies on cross-session stack persistence.
3. **border()** calls need review -- Go's version is a title-bar formatter, not a paragraph wrapper.
4. **Error message parsing** that checks for specific error text (rather than `#-*` prefix) may need updating.
5. **Set operation output order** differs (preserved vs sorted) -- softcode that depends on sorted output from `setunion()` etc. should add an explicit `sort()` call.
6. **@listen pattern matching** and **@redirect** are stored but not executed in Go.
