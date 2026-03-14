# TinyMUSH Compatibility Report

GoTinyMUSH is a Go reimplementation of TinyMUSH 3.3. This document reports the results of a comprehensive compatibility audit comparing GoTinyMUSH against C TinyMUSH 3.1p6, run across 26 test scripts in both mortal (unprivileged) and wizard (privileged) modes.

**Audit date**: 2026-03-14
**Test infrastructure**: Node.js scripts connecting to Go (:6886) and C (:9886) servers on 192.168.100.12, comparing output side-by-side.

---

## Audit Summary

- **18 behavioral areas** evaluated, ALL resolved
- **696 function tests**: 696 pass / 0 fail / 71 skipped = **100% match**
- **164 command/system tests**: 164/164 pass
- **Mail**: 52 tests, 50 pass, 1 C bug (Go correct), 1 skip
- **Phase 1 commands**: 237 pass, 2 C bugs, 52 C-version-gap failures (not Go bugs)

All remaining test "failures" are either Go-only features (not present in C 3.1), C server bugs, or C server state issues. Zero real Go bugs remain.

---

## Function Compatibility by Suite

| Suite | Pass | Total | Match |
|-------|------|-------|-------|
| Math | 118 | 118 | 100% |
| Strings | 89 | 89 | 100% |
| Lists | 100 | 100 | 100% |
| Objects | 74 | 74 | 100% |
| Comparison/Logic | 83 | 83 | 100% |
| Register/Stack | 20 | 20 | 100% |
| Time/Misc | 47 | 47 | 100% |
| Vector Math | 20 | 20 | 100% |
| Regex | 26 | 26 | 100% |
| Format/Table | 19 | 19 | 100% |
| Iter/Loop | 47 | 47 | 100% |
| Misc Functions | 39 | 39 | 100% |
| Structure | 5 | 5 | 100% |
| Comsys | 9 | 9 | 100% |
| **Total** | **696** | **696** | **100%** |

---

## Behavioral Areas Resolved (18 areas)

1. **Object permission checks** -- con/exit/next/lcon/lexits permission enforcement, Examinable checks on loc/where/home/money/parent/zone/name
2. **Object function semantics** -- rloc (blind walk vs room stop), xcon (range slice vs recursive), lastcreate (dbref-by-type vs timestamp)
3. **Flag functions** -- andflags/orflags `!` negation, hasflag attribute flag checking, hasflags OR-of-AND semantics
4. **Security** -- objeval Cannot_Objeval check, controls zone/inherit/CONTROL_OK checks
5. **String editing** -- edit() `^`/`$` anchors, squish() custom delimiter, trim() substring vs charset
6. **String measurement** -- mid() negative start adjustment, lpos() character-set vs substring, wordpos() 0-based vs 1-based
7. **String formatting** -- ljust/rjust/center multi-char fill truncation, border/cborder/rborder paragraph formatting, ansipos color state
8. **Encryption** -- encrypt/decrypt printable ASCII range (mod 95) matching C algorithm
9. **Math precision** -- add/sub/mul float output, modulo true-modulo vs remainder, t()/notbool() dbref handling
10. **List iteration** -- itext/inum level numbering (absolute vs relative), `#@` 1-based counting, while() semantics
11. **List operations** -- sort() numeric autodetect, member/remove case sensitivity, setunion/setdiff/setinter sort order, isort() index positions
12. **Iterator mechanics** -- parse() vs iter() textual substitution, filter() strict `'1'` check, fold/map/filter position arguments
13. **Register scoping** -- uprivate/private fresh register space, register preservation across @trigger chains
14. **Queue processing** -- @wait timing, @dolist/notify behavior, @trigger leading bracket dispatch
15. **Side-effect functions** -- trigger/force/link/wipe/wait stub completion, check_command restriction
16. **Search/locate** -- search/lsearch empty eval restriction, locate permission checks
17. **Mail system** -- @mail/clear on safe messages (C bug found), full @malias alias system
18. **Comsys** -- channel functions (cinfo, cwho, cemit, etc.) matching C output format

---

## Known Differences (not bugs)

These are intentional architectural differences where Go behavior differs from C but neither is wrong.

### Stack persistence
- **C**: Persists stack data on a player attribute (survives restart)
- **Go**: Uses per-EvalContext stack (session-scoped, lost on restart)
- **Rationale**: Architectural choice; Go's approach avoids attribute pollution

### Leading bracket expressions in triggered attrs
- **C**: `[setq()]@command` in a triggered THING attribute -- C does not dispatch the `@command` portion
- **Go**: Evaluates the bracket expression, then correctly dispatches the remaining command
- **Fixed in commit**: `01b6437`

### `%#` in chained triggers on THINGs
- **C**: May resolve `%#` to the intermediate THING object in a trigger chain
- **Go**: Preserves the original enactor (player) throughout the chain
- **Go behavior is correct** per the TinyMUSH specification

### Extended function set
- Go has approximately 170+ functions not present in C TinyMUSH 3.1 (event bus, JSON, arrays, navigation, conditionals, etc.)
- See [new-functions.md](new-functions.md) for the complete list

---

## Bugs Found and Fixed During Audit

### Function bugs (14)

1. **add/sub/mul** -- Truncated to int instead of returning float. Fixed to use `fval()` double output with zero-stripping.
2. **modulo** -- Used Go `%` remainder semantics instead of true mathematical modulo. Fixed: `modulo(-7,3)` now returns `2`.
3. **t()/notbool()** -- Missing `#-1` / `#N` dbref handling. Fixed: `t(#-1 NOT FOUND)` now returns `0`, matching C `xlate()`.
4. **edit()** -- `^` and `$` anchor patterns for prepend/append not supported. Fixed to match C behavior.
5. **squish()** -- Custom delimiter ignored, always squished spaces. Fixed to support custom character.
6. **trim()** -- Multi-char argument treated as character set (Go `strings.TrimLeft`). Fixed to treat as exact substring.
7. **mid()** -- Negative start did not adjust length. Fixed: `mid(str, -2, 5)` now adjusts nchars.
8. **lpos()** -- Used substring match instead of character-set match. Fixed to match C per-character semantics.
9. **ljust/rjust/center** -- Multi-char fill pattern could overshoot target width. Fixed to truncate final copy.
10. **itext/inum** -- Level numbering was inverted (relative vs absolute). Fixed to match C's absolute level numbering.
11. **`#@` (inum)** -- Was 0-based, C is 1-based. Fixed to start at 1.
12. **lnum()** -- 2-arg form used exclusive end; C uses inclusive. Fixed: `lnum(20,1)` now produces `20..1`.
13. **sort()** -- No autodetect; defaulted to alpha sort. Fixed to auto-detect numeric content.
14. **andflags/orflags** -- `!` negation was broken/ignored. Fixed flag negation logic.

### Command bugs (5)

1. **@mail/clear on safe messages** -- Go correctly refuses to clear safe messages; C allows it (C bug, not Go).
2. **Leading bracket + @switch dispatch** -- `[setq(0,X)]@switch` in deferred handler was not recognized. Fixed in `handleDeferredBodyCmd`.
3. **@trigger without args** -- Crashed or failed silently. Fixed to handle gracefully.
4. **@trigger register preservation** -- Registers were not cloned into triggered queue entries. Fixed to clone register context.
5. **@newpassword** -- Did not accept `#dbref` or `*player` syntax. Fixed to match @pcreate patterns.

---

## Skipped Categories

The following categories were intentionally skipped during audit. They represent features present in C TinyMUSH that are not yet implemented in GoTinyMUSH, or features where testing is impractical.

| Category | Reason |
|----------|--------|
| @listen pattern matching | Go stores the attribute but does not execute pattern-match dispatch |
| @redirect | Go stores the flag but does not implement output redirection |
| OUTPUTPREFIX/OUTPUTSUFFIX | Not implemented in Go |
| poll() | Not implemented in Go |
| @lemit | Not implemented in Go |
| GO_ONLY function tests (71 skipped) | Functions that exist only in Go (e.g., JSON, event bus, nav) -- no C equivalent to compare against |

These are logged as **GO_MISSING** (features not yet ported from C) rather than compatibility failures. They do not affect existing softcode that runs on both servers.

---

## Test Infrastructure

### Servers

| Server | Host | Port | Purpose |
|--------|------|------|---------|
| Go dev | 192.168.100.12 | 6886 | GoTinyMUSH under test |
| C ref | 192.168.100.12 | 9886 | C TinyMUSH 3.1p6 reference |

### Test accounts

Five dedicated accounts (AuditFunc, AuditEval, AuditObjM, AuditPermM, AuditCmdM), each with `auditpass` password, covering 26 test scripts across functions, evaluation, objects, permissions, and commands.

### Running the audit

```bash
cd /mnt/f/goTinyMush
./run_audit.sh          # both mortal + wizard phases
./run_audit.sh mortal   # mortal only
./run_audit.sh wizard   # wizard only
```

### Build and deploy chain

```bash
cd /mnt/f/goTinyMush
CGO_ENABLED=0 go build -o gotinymush ./cmd/gotinymush
cp gotinymush /mnt/f/CrystalMUSH/
cd /mnt/f/CrystalMUSH
docker cp gotinymush crystalmush-mush-1:/app/gotinymush
docker restart crystalmush-mush-1
```

See [gotinymush-audit-procedure.md](/mnt/f/vault/gotinymush-audit-procedure.md) for full procedure details.
