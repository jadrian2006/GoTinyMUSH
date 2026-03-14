# Flags & Powers Reference

GoTinyMUSH flag and power definitions, sourced from `pkg/gamedb/types.go` (constants),
`pkg/server/flags.go` (FlagTable + permission handlers), and `pkg/server/commands.go`
(display letters + power names).

## Object Flags

Flags are stored in three 32-bit words (`Flags[0]`, `Flags[1]`, `Flags[2]`).

### Flag Word 0

| Flag | Letter | Set Permission | Description | Compat |
|------|--------|----------------|-------------|--------|
| WIZARD | `W` | God only | Full administrative privileges | ✅ |
| ROYALTY | `Z` | Wizard | Elevated privileges below Wizard | ✅ |
| DARK | `D` | Special | Hides object from contents/who lists. Non-wizards can only set on exits or self (with `hide` power) | ✅ |
| HAVEN | `H` | Anyone | Prevents paging (players) or teleporting into (rooms) | ✅ |
| HALTED | `h` | Anyone | Prevents object from running queue entries or $-commands | ✅ |
| GOING | `G` | God clear-only | Marks object for pending destruction. Cannot be set directly; only God can clear it | ✅ |
| SAFE | `s` | Anyone | Protects object from `@destroy` | ✅ |
| DESTROY_OK | `d` | Anyone | Allows non-owner destruction | ✅ |
| INHERIT | `I` | Inherits | Inherits wizard permissions from owner. Setter must already have INHERIT or be wizard | ✅ |
| NOSPOOF | `N` | Anyone | Prepends `[object]` prefix to messages showing the true source | ✅ |
| VISUAL | `V` | Anyone | Makes object's attributes visible to everyone | ✅ |
| OPAQUE | `O` | Anyone | Prevents others from seeing contents | ✅ |
| QUIET | `Q` | Anyone | Suppresses set/trigger confirmation messages | ✅ |
| PUPPET | `p` | Anyone | Relays messages seen by the puppet to its owner | ✅ |
| STICKY | `S` | Anyone | Things: go home when dropped. Rooms: dropped things go home | ✅ |
| TRANSPARENT | `t` | Anyone | Allows looking through to contents or linked room | ✅ |
| AUDIBLE | `a` | Anyone | Sounds propagate through exits; enables LISTEN forwarding | ✅ |
| MONITOR | `M` | Anyone | Enables LISTEN pattern matching on the object | ✅ |
| VERBOSE | `v` | Anyone | Shows the commands an object executes | ✅ |
| TRACE | `T` | Anyone | Shows detailed evaluation trace for debugging | ✅ |
| TERSE | `q` | Anyone | Suppresses room descriptions on movement | ✅ |
| MYOPIC | `m` | Anyone | Prevents seeing dbrefs on objects | ✅ |
| ROBOT | `r` | Anyone (players only) | Marks a player as a robot/bot. Can only be set on TYPE_PLAYER objects | ✅ |
| IMMORTAL | `i` | Wizard | Cannot be killed | ✅ |
| ENTER_OK | `e` / `E` | Anyone | Allows other objects to enter this object | ✅ |
| LINK_OK | `L` | Anyone | Allows others to link exits to this room | ✅ |
| JUMP_OK | `J` | Anyone | Allows teleporting to/from this location | ✅ |
| CHOWN_OK | `C` | Anyone | Allows others to `@chown` the object | ✅ |
| HAS_STARTUP | `=` | God (internal) | Object has a STARTUP attr (set automatically) | ✅ |

### Flag Word 1

| Flag | Letter | Set Permission | Description | Compat |
|------|--------|----------------|-------------|--------|
| ABODE | `A` | Anyone | Allows players to set their home to this room | ✅ |
| FREE / FLOATING | `F` | Anyone | Suppresses "disconnected room" warnings in `@dbck` | ✅ |
| UNFINDABLE | `U` | Anyone | Cannot be found with `@whereis` or `loc()` by non-wizards | ✅ |
| PARENT_OK | `Y` | Anyone | Allows others to `@parent` to this object | ✅ |
| LIGHT | `l` | Anyone | Object is visible even in DARK rooms | ✅ |
| AUDITORIUM | `n` | Anyone | Only @-listeners can speak (moderator mode) | ✅ |
| ANSI | `X` | Anyone | Object sees ANSI color codes | ✅ |
| HEAD | `?` | Anyone | Player is a section head (admin marker) | ✅ |
| FIXED | `f` | Anyone | Prevents `@teleport` and `home` for the player | ✅ |
| UNINSPECTED | `g` | Wizard/Royalty | Marks player as not yet approved by staff | ✅ |
| ZONE | `o` | Anyone | Object acts as a Zone Master Object (ZMO) | ✅ |
| NOBLEED | `-` | Anyone | Prevents ANSI codes from bleeding past this object's output | ✅ |
| STAFF | `w` | Wizard | Staff-level access (below Wizard) | ✅ |
| GAGGED | `j` | Wizard | Prevents the player from speaking or posing | ✅ |
| KEY | `K` | Anyone | General-purpose marker flag | ✅ |
| STOP | `!` | Anyone | Stops $-command matching at this object (don't check parent) | ✅ |
| BOUNCE | `b` | Anyone | Exits: rejects with REJECT message. Rooms/Things: echoes back | ✅ |
| CONTROL_OK | `z` | Anyone | Allows zone-based control of this object | ✅ |
| VACATION | `\|` | Anyone | Player is on vacation (may be purged less aggressively) | ✅ |
| HTML | `~` | Anyone | Object receives HTML-formatted output | ✅ |
| BLIND | `B` | Wizard | Suppresses "has arrived" / "has left" messages | ✅ |
| SUSPECT | `u` | Wizard | Logs commands by this player for review | ✅ |
| WATCHER | `+` | Special | Receives connect/disconnect notifications. Requires `watch` power or wizard to set | ✅ |
| CONNECTED | `c` | God (internal) | Player is currently connected (set/cleared by server) | ✅ |
| SLAVE | `x` | Wizard | Restricted guest-like player (limited commands) | ✅ |
| COMMANDS | `$` | God (internal) | Object has $-command attributes (auto-detected) | ✅ |
| HAS_LISTEN | `@` | God (internal) | Object has LISTEN attr (auto-detected) | ✅ |
| HAS_FORWARDLIST | `&` | God (internal) | Object has FORWARDLIST attr (auto-detected) | ✅ |
| HAS_DAILY | `*` | God (internal) | Object has DAILY attr (auto-detected) | ✅ |
| PLAYER_MAILS | `` ` `` | God (internal) | Player has mail (auto-detected) | ✅ |
| CONSTANT | (none) | God (internal) | Attributes cannot be changed | ✅ |

### Flag Word 2

| Flag | Letter | Set Permission | Description | Compat |
|------|--------|----------------|-------------|--------|
| REDIR_OK | `>` | Anyone | Allows `@redirect` to target this object | ✅ |
| HAS_REDIRECT | `<` | God (internal) | Object has active redirections | ✅ |
| ORPHAN | `y` | Anyone | Disables parent attribute inheritance | ✅ |
| HAS_DARKLOCK | `.` | God (internal) | Object has a DarkLock set (checked by `Darkened()`) | ✅ |
| NODEFAULT | (none) | Anyone | Disables `attr_defaults` processing | ✅ |
| PRESENCE | `^` | Anyone | Enables presence-system lock filtering (UNREAL) | ✅ |
| SPEECHMOD | (none) | God (internal) | Object has a speech modifier attr | ✅ |
| HAS_PROPDIR | `,` | God (internal) | Object has Propdir attribute | ✅ |
| INSTANCE | `^` | Anyone | Marks object as an instanced template | 🆕 |
| MARKER0-MARKER9 | `0`-`9` | Anyone | Ten user-definable marker flags | ✅ |

### Flag Aliases

Several flags have alternate names that resolve to the same bit:

| Alias | Canonical Name | Notes |
|-------|---------------|-------|
| SEE_THROUGH | TRANSPARENT | Go alias |
| HEAR_THROUGH | AUDIBLE | Go alias |
| FLOATING | FREE | Go alias |
| ZONE_PARENT | ZONE | Go alias |
| HAS_COMMANDS | COMMANDS | Go alias |
| NO_BLEED | NOBLEED | Underscore variant |
| OOB | AUDITORIUM | Go alias; reuses AUDITORIUM bit for Out-Of-Band data |

## Attribute Flags

Set with `@set obj/attr = [!]FLAG` or at definition time with `@attribute/access`.

### Per-Instance Attribute Flags

Set on individual attribute instances:

| Flag | Letter | @set Name | Description | Compat |
|------|--------|-----------|-------------|--------|
| LOCKED | `+` | (via @lock/attr) | Attribute is locked against changes | ✅ |
| NO_COMMAND | `$` | NO_COMMAND | Suppresses $-command matching on this attr | ✅ |
| CASE | `C` | CASE | Case-sensitive regexp matching | ✅ |
| DEFAULT | `D` | — | Checks `attr_defaults` object for fallback | ✅ |
| HTML | `H` | HTML | Don't HTML-escape output | ✅ |
| PRIVATE / NO_INHERIT | `I` | PRIVATE | Not inherited by children via @parent | ✅ |
| RMATCH | `M` | — | Sets match result into registers | ✅ |
| NO_NAME | `N` | — | Suppresses name prepend in OATTR context | ✅ |
| NOPARSE | `P` | NOPARSE | Don't evaluate during $-command check | ✅ |
| NOW | `Q` | NOW | Execute $-command match immediately (not queued) | ✅ |
| REGEXP | `R` | REGEXP | Use regular expression matching for $-commands | ✅ |
| STRUCTURE | `S` | — | Attribute holds a structure instance | ✅ |
| TRACE | `T` | — | Trace this attribute's u() evaluations | ✅ |
| VISUAL | `V` | VISUAL | Anyone can see this attribute | ✅ |
| NO_CLONE | `c` | NO_CLONE | Don't copy when @cloning | ✅ |
| DARK | `d` | DARK | Only God (#1) can see | ✅ |
| GOD | `g` | GOD | Only God can change | ✅ |
| CONSTANT | `k` | — | Server-only; nobody can change | ✅ |
| MDARK | `m` | MDARK | Only wizards can see | ✅ |
| WIZARD | `w` | WIZARD | Only wizards can change | ✅ |
| ODARK | — | ODARK | Only owner can see | ✅ |
| NOPROG | — | NOPROG | Alias for NO_COMMAND | ✅ |
| PROPAGATE | `p` | PROPAGATE | Auto-copy from parent to child on @parent/@clone | 🆕 |

### Definition-Level Attribute Flags

Set on attribute definitions (via `@attribute/access` or conf file `user_attr_access`):

| Flag | @attribute Name | Description | Compat |
|------|----------------|-------------|--------|
| DARK | DARK | Only God can see | ✅ |
| GOD | GOD | Only God can modify | ✅ |
| IS_LOCK | IS_LOCK | Attribute stores a lock expression | ✅ |
| LOCKED | LOCKED | Attribute is locked (instance-level) | ✅ |
| NO_CLONE | NO_CLONE | Not copied on @clone | ✅ |
| NO_COMMAND | NO_COMMAND | Suppresses $-cmd matching | ✅ |
| NO_INHERIT | NO_INHERIT | Not inherited (alias for PRIVATE) | ✅ |
| VISUAL | VISUAL | Anyone can read | ✅ |
| WIZARD | WIZARD | Only wizards can modify | ✅ |
| PROPAGATE | PROPAGATE | Auto-propagate to children | 🆕 |

## Powers

Powers grant specific capabilities to objects, set with `@power object = [!]power`.
Only wizards can set powers; some are God-only. Powers are stored in two 32-bit
words (`Powers[0]`, `Powers[1]`).

| Power | Aliases | God Only | Description | Compat |
|-------|---------|----------|-------------|--------|
| announce | — | No | Can use `@wall` to broadcast to all players | ✅ |
| attr_read | mdark_attr | No | Can read MDARK (wizard-dark) attributes | ✅ |
| attr_write | wiz_attr | No | Can write WIZARD-flagged attributes | ✅ |
| boot | — | No | Can `@boot` other players | ✅ |
| builder | — | No | Can create rooms/exits (when `building` is restricted) | ✅ |
| change_quotas | quota | No | Can modify other players' quota | ✅ |
| chown_anything | — | No | Can `@chown` any object | ✅ |
| cloak | — | Yes | Completely invisible to non-God players (above DARK) | ✅ |
| comm_all | — | No | Full access to all comsys channels | ✅ |
| control_all | — | Yes | Controls all objects (effectively wizard-level control) | ✅ |
| expanded_who | wizard_who | No | Sees the wizard WHO display with idle times and IPs | ✅ |
| find_unfindable | — | No | Can locate UNFINDABLE objects | ✅ |
| free_money | — | No | Actions don't cost money | ✅ |
| free_quota | — | No | Building doesn't consume quota | ✅ |
| guest | — | Yes | Marks player as a guest (restricted) | ✅ |
| halt | — | No | Can `@halt/all` to clear all queues | ✅ |
| hide | — | No | Can set self DARK to hide from WHO | ✅ |
| idle | — | No | Exempt from idle timeout | ✅ |
| link_any_home | — | No | Can link objects home to any location | ✅ |
| link_to_anything | — | No | Can link exits to any destination | ✅ |
| link_variable | — | No | Can create variable exits (`@link exit = <expression>`) | ✅ |
| long_fingers | — | No | Can act on objects in other rooms | ✅ |
| no_destroy | — | No | Object is protected from destruction (even by owner) | ✅ |
| open_anywhere | — | No | Can `@open` exits in rooms they don't control | ✅ |
| pass_locks | — | No | Automatically passes all lock checks | ✅ |
| poll | — | No | Can set the `@doing` poll header | ✅ |
| prog | — | No | Can use `@program` to put other players into program mode | ✅ |
| search | — | No | Can `@search` objects not owned by self | ✅ |
| see_all | — | No | Can examine any object (like `examine/all`) | ✅ |
| see_hidden | — | No | Can see DARK players on WHO and in rooms | ✅ |
| see_queue | — | No | Can `@ps/all` to view the global queue | ✅ |
| stat_any | — | No | Can `@stats` any player | ✅ |
| steal_money | steal | No | Can take money from other objects | ✅ |
| tel_anywhere | — | No | Can `@teleport` to any location (bypasses JUMP_OK check) | ✅ |
| tel_unrestricted | tel_anything | No | Can `@teleport` any object (bypasses control check on victim) | ✅ |
| unkillable | — | No | Cannot be killed | ✅ |
| use_sql | — | Yes | Can use SQL-related functions | ✅ |
| watch_logins | watch | No | Receives connect/disconnect notifications (enables WATCHER flag) | ✅ |
| bot | — | No | Can manage API keys for owned objects (`@apikey`) | 🆕 |

### Power Aliases

| Alias | Canonical Power |
|-------|----------------|
| attr_read | mdark_attr |
| attr_write | wiz_attr |
| expanded_who | wizard_who |
| quota | change_quotas |
| steal_money | steal |
| tel_anything | tel_unrestricted |
| watch_logins | watch |

## Flag Permission Handlers

These match C TinyMUSH's `fh_*` handlers in `flags.c`:

- **fh_dark_bit**: Non-wizards can only set DARK on exits or on themselves (if they have the `hide` power).
- **fh_going_bit**: GOING can never be set directly. Only God can clear it.
- **fh_inherit**: Setter must already have INHERIT flag or be wizard.
- **fh_watcher**: WATCHER requires the `watch` power or wizard status.
- **fh_player_bit**: ROBOT can only be set on PLAYER-type objects.
