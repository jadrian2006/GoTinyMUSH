# Locks Reference

GoTinyMUSH lock system documentation, sourced from `pkg/server/boolexp.go` (parser +
evaluator), `pkg/server/admin_commands.go` (@lock command), and `pkg/gamedb/types.go`
(BoolExp types).

## Overview

Locks are boolean expressions stored as attributes on objects. When an action is
attempted (entering, using, giving, etc.), the server evaluates the lock expression
against the acting player. If the expression evaluates to true, the action succeeds;
otherwise it fails and the corresponding FAIL/OFAIL/AFAIL messages fire.

An empty (unset) lock always passes -- the object is unlocked.

## Lock Types

Each lock type corresponds to a well-known attribute and can be set with `@lock/switch`.

### Core Locks

| Lock Type | @lock Switch | Attribute (#) | Description | Compat |
|-----------|-------------|---------------|-------------|--------|
| Default | (none) / `/defaultlock` | DefaultLock (42) | Controls who can pick up things, traverse exits, or trigger SUCC/FAIL | ✅ |
| Enter | `/enter` or `/enterlock` | EnterLock (59) | Controls who can enter an object (ENTER_OK things/players) | ✅ |
| Leave | `/leave` or `/leavelock` | LeaveLock (60) | Controls who can leave an object (checked on `leave` command) | ✅ |
| Use | `/use` or `/uselock` | UseLock (62) | Controls who can use an object (triggers USE/OUSE messages) | ✅ |
| Give | `/give` or `/givelock` | GiveLock (63) | Controls who can give things to this object | ✅ |
| Receive | `/receive` or `/receivelock` | ReceiveLock (87) | Controls what objects this object will accept | ✅ |
| Drop | `/drop` or `/droplock` | DropLock (86) | Controls where/when an object can be dropped | ✅ |

### Communication Locks

| Lock Type | @lock Switch | Attribute (#) | Description | Compat |
|-----------|-------------|---------------|-------------|--------|
| Page | `/page` or `/pagelock` | PageLock (61) | Controls who can page this player | ✅ |
| Speech | `/speech` or `/speechlock` | SpeechLock (209) | Controls who can speak in this room | ✅ |

### Movement Locks

| Lock Type | @lock Switch | Attribute (#) | Description | Compat |
|-----------|-------------|---------------|-------------|--------|
| Teleport | `/tport` or `/tportlock` | TportLock (85) | Controls who can be teleported to/from (on victim) | ✅ |
| Telout | `/telout` or `/teloutlock` | TeloutLock (94) | Controls who can teleport out of this room | ✅ |

### Building Locks

| Lock Type | @lock Switch | Attribute (#) | Description | Compat |
|-----------|-------------|---------------|-------------|--------|
| Link | `/link` or `/linklock` | LinkLock (93) | Controls who can link exits to this destination | ✅ |
| Parent | `/parent` or `/parentlock` | ParentLock (98) | Controls who can @parent to this object | ✅ |
| Chown | `/chown` or `/chownlock` | ChownLock (217) | Controls who can @chown this object | ✅ |
| Open | — | OpenLock (144) | Controls where exits can be opened from this location | ✅ |

### Control Lock

| Lock Type | @lock Switch | Attribute (#) | Description | Compat |
|-----------|-------------|---------------|-------------|--------|
| Control | `/control` or `/controllock` | ControlLock (99) | Zone-based control: determines who controls objects in a zone. Set on ZMOs | ✅ |

### Visibility Locks

| Lock Type | @lock Switch | Attribute (#) | Description | Compat |
|-----------|-------------|---------------|-------------|--------|
| Dark | `/dark` or `/darklock` | DarkLock (219) | Determines who a DARK object is dark to. Passing = object IS dark to you; failing = you see through it. Sets HAS_DARKLOCK flag automatically | ✅ |
| User | `/user` or `/userlock` | UserLock (97) | General-purpose user-defined lock | ✅ |

### Presence Locks

These locks work with the PRESENCE flag to create selective visibility/audibility.
An object with PRESENCE set uses these locks to filter which players perceive its
messages. Both directions are checked -- sender's outbound locks AND target's inbound locks.

| Lock Type | @lock Switch | Attribute (#) | Direction | Description | Compat |
|-----------|-------------|---------------|-----------|-------------|--------|
| Known | `/known` or `/knownlock` | KnownLock (223) | Outbound | Who sees this PRESENCE object's presence? | ✅ |
| Heard | `/heard` or `/heardlock` | HeardLock (224) | Outbound | Who hears this PRESENCE object's speech? | ✅ |
| Moved | `/moved` or `/movedlock` | MovedLock (225) | Outbound | Who notices this PRESENCE object moving? | ✅ |
| Knows | `/knows` or `/knowslock` | KnowsLock (226) | Inbound | Who does this PRESENCE object see? | ✅ |
| Hears | `/hears` or `/hearslock` | HearsLock (227) | Inbound | Who does this PRESENCE object hear? | ✅ |
| Moves | `/moves` or `/moveslock` | MovesLock (228) | Inbound | Who does this PRESENCE object notice moving? | ✅ |

### Attribute Lock

Not a boolexp lock, but a per-attribute instance flag:

```
@lock/attr obj/ATTRNAME      -- Locks the attribute (sets AF_LOCK)
@unlock/attr obj/ATTRNAME    -- Unlocks the attribute (clears AF_LOCK)
```

## Lock Evaluation Keys

Lock expressions are boolean expressions built from these primitives:

### Object Match

| Syntax | Description | Example |
|--------|-------------|---------|
| `#dbref` | Matches if player IS or CARRIES the object | `@lock door = #1234` |
| `*player` | Matches by player name (resolved to dbref at set time) | `@lock door = *Bob` |
| `name` | Matches by object name (resolved to dbref at set time) | `@lock door = MyKey` |

When evaluating: the player passes if they ARE the referenced object or CARRY it
in their inventory.

### Attribute Match

| Syntax | Description | Example |
|--------|-------------|---------|
| `attr:pattern` | Wildcard match against attribute value on player or inventory | `@lock door = FACTION:Thieves*` |

The attribute is checked on the player first, then on each item in the player's
inventory. Pattern matching is case-insensitive with wildcards (`*`, `?`).

### Evaluation Lock

| Syntax | Description | Example |
|--------|-------------|---------|
| `attr/pattern` | Evaluates attr as softcode, compares result to pattern | `@lock door = CHECK/1` |

The attribute is evaluated as softcode with the player as the enactor (`%#`).
The attribute is first looked up on the `from` object (usually the object being
acted upon), falling back to the lock owner (`thing`). The result is compared
to the pattern using case-insensitive wildcard matching.

### Flag Check

Flag checking is done via attribute evaluation locks. Example:

```
@lock door = IS_WIZARD/1
```

Where IS_WIZARD is an attribute containing `[hasflag(%#,WIZARD)]`.

### Boolean Operators

| Syntax | Description | Example |
|--------|-------------|---------|
| `key1 & key2` | AND -- both keys must pass | `@lock door = #123 & FACTION:Guild` |
| `key1 \| key2` | OR -- either key passes | `@lock door = *Bob \| *Alice` |
| `!key` | NOT -- key must fail | `@lock door = !*Bob` |
| `(expr)` | Grouping -- overrides precedence | `@lock door = (*Bob \| *Alice) & RANK:3` |

Operator precedence: `!` (highest) > `&` > `|` (lowest). Use parentheses
to override.

### Special Prefixes

| Syntax | Type | Description | Example |
|--------|------|-------------|---------|
| `+obj` | Carry | Player must carry the object (not just BE it) | `@lock chest = +#5678` |
| `+attr:pattern` | Carry+Attr | Attribute match on inventory only (not player) | `@lock door = +GUILD:Thieves` |
| `=obj` | Is | Player must BE the object exactly (carrying doesn't count) | `@lock console = =#1234` |
| `=attr:pattern` | Is+Attr | Attribute match on player only (not inventory) | `@lock door = =RANK:Captain` |
| `$obj` | Owner | Player's owner must match object's owner | `@lock area = $#100` |
| `@obj` | Indirect | Evaluate the default lock on the referenced object | `@lock door = @#9999` |

### Carry vs Is vs Default

| Lock Key | Player IS obj | Player CARRIES obj |
|----------|---------------|-------------------|
| `#obj` (default) | Pass | Pass |
| `+#obj` (carry) | Fail | Pass |
| `=#obj` (is) | Pass | Fail |

## Lock Evaluation Details

### Wizard Bypass

The `pass_locks` power bypasses ALL lock evaluation -- `CouldDoIt()` returns
true immediately. This is the only power that bypasses locks. Note: wizard
status alone does NOT bypass locks (unlike some other MUSH implementations).

`CouldDoItStrict()` is used for locks that should be absolute (e.g., vehicle
leave locks) -- it never checks powers.

### Indirection Depth

Indirect locks (`@obj`) are followed recursively up to a maximum depth of 20
to prevent infinite loops.

### DarkLock Semantics

DarkLock has inverted semantics compared to other locks:
- **Passing** the DarkLock means the object IS dark to you (you cannot see it)
- **Failing** the DarkLock means you see through the dark (the object is visible)

This is checked by `Darkened()` -- an object is dark to a player if:
1. It has the DARK flag, AND
2. Either no DarkLock is set, OR the player passes the DarkLock

### Fail Messages

When a default lock check fails, the server processes three attributes:

| Attribute | Recipient | Description |
|-----------|-----------|-------------|
| FAIL / EFAIL / UFAIL / etc. | Player | Failure message shown to the actor |
| OFAIL / OEFAIL / OUFAIL / etc. | Room | Failure message shown to others (prepended with player name) |
| AFAIL / AEFAIL / AUFAIL / etc. | Queue | Action list executed on failure |

Each lock type has its own set of fail attributes (e.g., EnterLock uses
EFAIL/OEFAIL/AEFAIL, UseLock uses UFAIL/OUFAIL/AUFAIL).

## Examples

### Basic key lock
```
@lock treasure_chest = Gold Key
```
Player must be or carry the object named "Gold Key".

### Player-only lock
```
@lock secret_door = *Alice | *Bob
```
Only the players Alice or Bob can pass.

### Attribute-based lock
```
@lock guild_hall = GUILD:Adventurers*
```
Player (or something they carry) must have a GUILD attribute matching "Adventurers*".

### Evaluation lock
```
&CHECK_LEVEL door = [gte(get(%#/LEVEL),10)]
@lock door = CHECK_LEVEL/1
```
Evaluates CHECK_LEVEL on the door; player must have LEVEL >= 10.

### Carry-only lock
```
@lock vault = +Vault Key
```
Player must physically carry the Vault Key (being the key doesn't count).

### Identity lock
```
@lock console = =#1234
```
Only object #1234 itself can pass (carrying doesn't count).

### Owner lock
```
@lock shared_area = $#100
```
Player's owner must be the same as object #100's owner.

### Indirect lock
```
@lock east_gate = @#5000
```
Evaluates the default lock on object #5000 instead of the gate's own lock.

### Complex boolean
```
@lock restricted_room = (FACTION:Royal | *King) & !*Prisoner & RANK:3*
```
Must be royal faction or King, must NOT be Prisoner, and RANK must start with "3".

### Presence lock (selective visibility)
```
@set ghost = PRESENCE
@lock/heard ghost = SPIRIT_SIGHT:1
@lock/known ghost = SPIRIT_SIGHT:1
```
Only players with `SPIRIT_SIGHT` attribute set to `1` can hear or see the ghost.

### Zone control lock
```
@set #500 = ZONE
@lock/control #500 = BUILDER:1 | *Admin
```
Objects in zone #500 can be controlled by anyone with BUILDER:1 or the player Admin.

### Dark lock (selective darkness)
```
@set spy = DARK
@lock/dark spy = !*SecurityOfficer
```
The spy is dark to everyone EXCEPT SecurityOfficer (who fails the dark lock and
thus sees through the darkness).
