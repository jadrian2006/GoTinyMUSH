# GoTinyMUSH Commands Reference

Complete reference for all built-in commands. Commands marked ✅ are standard TinyMUSH 3.x compatible. Commands marked 🆕 are new in GoTinyMUSH.

---

## Communication

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| say | `say <message>` or `"<message>` | Speak to the room | ✅ |
| pose | `pose <action>` or `:<action>` | Pose/emote to the room | ✅ |
| ; | `;<action>` | Pose with no space after name | ✅ |
| page | `page <player>=<message>` | Send private message to a player | ✅ |
| r | `r <message>` | Reply to last page | ✅ |
| whisper | `whisper <player>=<message>` | Whisper to player in same room | ✅ |
| @emit | `@emit <message>` | Emit raw text to current room | ✅ |
| think | `think <expression>` | Evaluate and show result to self only | ✅ |
| @pemit | `@pemit <target>=<message>` | Private emit to a specific target | ✅ |
| @oemit | `@oemit <target>=<message>` | Emit to room excluding target | ✅ |
| @remit | `@remit <room>=<message>` | Emit to a specific room | ✅ |
| @wall | `@wall <message>` | Broadcast to all connected players | ✅ |

### say

```
say <message>
"<message>
say/noeval <message>
```

Speaks to the current room. The speaker sees `You say "<message>"` and others see `<Name> says "<message>"`. Respects speech locks on the room.

**Switches:** `/noeval` — skip evaluation of softcode in message.

**Permissions:** Speech lock on room must pass.

**Example:**
```
> say Hello everyone!
You say "Hello everyone!"
```

### pose

```
pose <action>
:<action>
pose/noeval <action>
pose/nospace <action>
```

Emotes an action. Everyone in the room sees `<Name> <action>`.

**Switches:** `/noeval` — skip evaluation. `/nospace` — no space between name and action (same as `;`).

**Permissions:** Speech lock on room must pass.

**Example:**
```
> :waves hello.
Raimier waves hello.
```

### page

```
page <player>=<message>
page <player>=:<pose>
page <player>=;<nospace-pose>
page/noeval <player>=<message>
```

Sends a private message to a connected player. Supports pose format with `:` or `;` prefix. Stores last-paged player for the `r` (reply) command.

**Switches:** `/noeval` — skip evaluation of message text.

**Permissions:** Target must pass page lock. Presence lock checked.

### @emit

```
@emit <message>
@emit/room <target>=<message>
@emit/noeval <message>
```

Emits raw text to the room with no player name prefix.

**Switches:** `/room` — emit to the room containing `<target>`. `/noeval` — skip evaluation.

**Permissions:** Speech lock on room must pass.

### @pemit

```
@pemit <target>=<message>
@pemit/contents <target>=<message>
@pemit/list <target1 target2 ...>=<message>
@pemit/silent <target>=<message>
@pemit/noeval <target>=<message>
@pemit/object <target>=<message>
```

Sends a message to a specific target.

**Switches:**
- `/contents` — send to all contents of target location
- `/list` — targets is space-separated list of dbrefs
- `/silent` — suppress "You pemit..." feedback
- `/noeval` — skip evaluation of message
- `/object` — match target as object, not player name

**Permissions:** Must control target, be nearby, or have Long_Fingers power. Wizards bypass.

### @oemit

```
@oemit <target>=<message>
@oemit/noeval <target>=<message>
```

Emits message to the room containing target, excluding the target.

**Switches:** `/noeval` — skip evaluation.

### @remit

```
@remit <room>=<message>
```

Emits message to all players in a specific room (by dbref).

### @wall

```
@wall <message>
@wall/wizard <message>
@wall/admin <message>
@wall/pose <message>
@wall/emit <message>
```

Broadcasts a message to all connected players (or subset).

**Switches:**
- `/wizard` — send only to wizards
- `/admin` — send to wizards and royalty
- `/pose` — pose format: `## <Name> <message>`
- `/emit` — raw emit with no prefix

**Permissions:** Requires `announce` power. ✅

---

## Movement

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| go | `go <direction>` | Move through an exit | ✅ |
| home | `home` | Return to home location | ✅ |
| enter | `enter <object>` | Enter an ENTER_OK object | ✅ |
| leave | `leave` | Leave the current container | ✅ |

### go

```
go <exit-name>
```

Moves through a named exit in the current room. Exits are matched by exact alias (semicolon-separated names). Checks exit lock, fires SUCC/OSUCC/ASUCC on success, FAIL/OFAIL/AFAIL on failure. Also checks zone exits and master room exits as fallback.

### home

```
home
```

Returns the player to their home location (set by `@link`). Displays "There's no place like home..." three times.

### enter

```
enter <object>
```

Enters an object that has the ENTER_OK flag. Checks enter lock, fires ENTER/OENTER/AENTER messages.

**Permissions:** Object must have ENTER_OK flag. Enter lock must pass.

### leave

```
leave
```

Leaves the current container object, returning to the room it's in. Fires LEAVE/OLEAVE/ALEAVE messages.

---

## Building

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| @create | `@create <name>[=<cost>]` | Create a new thing | ✅ |
| @dig | `@dig <name>[=<exit_to>[=<exit_from>]]` | Create a new room | ✅ |
| @open | `@open <exit>=<destination>` | Create a new exit | ✅ |
| @describe | `@describe <obj>=<text>` | Set an object's description | ✅ |
| @name | `@name <obj>=<new-name>` | Rename an object | ✅ |
| @set | `@set <obj>=<flag>` or `@set <obj>=<attr>:<value>` | Set flags or attributes | ✅ |
| @destroy | `@destroy <obj>` | Destroy an object | ✅ |
| @link | `@link <obj>=<dest>` | Link an exit, set home, or set dropto | ✅ |
| @unlink | `@unlink <obj>` | Unlink an exit or clear dropto | ✅ |
| @parent | `@parent <obj>=<parent>` | Set an object's parent | ✅ |
| @clone | `@clone <obj>[=<name>]` | Clone an object | ✅ |
| @wipe | `@wipe <obj>[/<pattern>]` | Clear all (or matching) attributes | ✅ |
| @chown | `@chown <obj>=<player>` | Change object ownership | ✅ |
| @chzone | `@chzone <obj>=<zone>` | Change object's zone | ✅ |
| @edit | `@edit <obj>/<attr>=<search>,<replace>` | Search-and-replace in an attribute | ✅ |

### @create

```
@create <name>[=<cost>]
```

Creates a new THING object in your inventory. Cost is clamped between `createmin` and `createmax` config values. Wizards build for free.

**Permissions:** Must have quota. Costs pennies (configurable).

**Example:**
```
> @create Crystal Tuner=10
Crystal Tuner created as object #1234
```

### @dig

```
@dig <room-name>[=<exit-to>[=<exit-from>]]
@dig/teleport <room-name>
```

Creates a new room. Optionally creates exits to and from the new room.

**Switches:** `/teleport` — teleport to the new room after creating it.

**Permissions:** Must have quota. Costs dig_cost pennies.

**Example:**
```
> @dig Grand Hall=North;n=South;s
Grand Hall created with room number 5678.
Opened.
Linked.
Opened.
Linked.
```

### @open

```
@open <exit-name>[=<destination>]
```

Creates a new exit from the current room. Optionally links it to a destination. Exit names can contain aliases separated by semicolons (e.g., `North;n;no`).

**Permissions:** Must control the room (or have `open_anywhere` power). Costs open_cost pennies.

### @set

```
@set <obj>=<flag>
@set <obj>=!<flag>
@set <obj>=<attr>:<value>
@set <obj>/<attr>=<attr-flag>
@set <obj>/<attr>=!<attr-flag>
@set/quiet <obj>=<flag>
```

Sets or clears flags, attributes, or attribute flags on an object.

**Switches:** `/quiet` — suppress "Set."/"Cleared." confirmation messages.

**Attribute flags:** WIZARD, DARK, MDARK, VISUAL, NO_COMMAND, NO_CLONE, PRIVATE, REGEXP, CASE, NOPARSE, GOD, NOPROG, ODARK, HTML, NOW.

**Permissions:** Must control the target. Wizard/God flags require wizard/God permissions.

**Example:**
```
> @set me=DARK
Set.
> @set box=DESC:A wooden box.
Set.
> @set box/CMD_OPEN=NO_COMMAND
Set.
```

### @destroy

```
@destroy <obj>
@destroy/override <obj>
@destroy/instant <obj>
```

Destroys an object. By default, marks it GOING for deferred cleanup by the database checker.

**Switches:**
- `/override` — destroy SAFE objects (bypasses safety check)
- `/instant` — destroy immediately without GOING delay

**Permissions:** Must control the target (or DESTROY_OK things in inventory). Cannot destroy #0 or God. `no_destroy` power protects objects.

### @link

```
@link <exit>=<destination>
@link <exit>=variable
@link <room>=<dropto>
@link <thing/player>=<home>
```

Links exits to destinations, sets room dropto, or sets object/player home.

**Permissions:** Must control the object. Destination must be controlled, LINK_OK, or pass link lock. Variable exits require `link_variable` power.

### @clone

```
@clone <obj>[=<new-name>]
@clone/parent <obj>
@clone/inventory <obj>
@clone/location <obj>
@clone/preserve <obj>
```

Creates a copy of an object.

**Switches:**
- `/parent` — set the original as the clone's parent instead of copying attributes
- `/inventory` — place clone in your inventory (default: current room)
- `/location` — place clone at the source object's location
- `/preserve` — copy flags from the source object

**Permissions:** Must be able to examine the source. Cannot clone players.

### @wipe

```
@wipe <obj>
@wipe <obj>/<pattern>
```

Clears all attributes from an object, or only those matching a wildcard pattern.

**Permissions:** Must control the target.

### @chown

```
@chown <obj>=<player>
@chown <obj>/<attr>=<player>
@chown/nostrip <obj>=<player>
```

Transfers ownership of an object or individual attribute.

**Switches:** `/nostrip` — (God only) skip stripping privilege flags after chown.

After chown, privilege flags are stripped and HALT is set. Things must be in your inventory unless you have `chown_anything` power.

**Permissions:** Must control target (or target is CHOWN_OK and passes chown lock). Must control new owner (or have `chown_anything`).

### @chzone

```
@chzone <obj>=<zone>
@chzone <obj>=none
@chzone/add <obj>=<zone>
@chzone/remove <obj>=<zone>
@chzone/nostrip <obj>=<zone>
```

Sets or changes an object's zone. Zones provide shared command/exit inheritance.

**Switches:**
- `/add` — add an additional zone (multi-zone system, requires `multizone_enabled` config)
- `/remove` — remove a zone from the object
- `/nostrip` — (wizard only) skip stripping privilege flags

**Permissions:** Must control the target and the zone object (or be wizard).

### @edit

```
@edit <obj>/<attr>=<search>,<replace>
```

Performs search-and-replace on an attribute value. Special search patterns:
- `$` — append `<replace>` to end
- `^` — prepend `<replace>` to start
- `\$` or `\^` — search for literal `$` or `^`

**Permissions:** Must control the target.

---

## Locks

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| @lock | `@lock <obj>=<key>` | Set a lock on an object | ✅ |
| @unlock | `@unlock <obj>` | Remove a lock from an object | ✅ |

### @lock

```
@lock <obj>=<key-expression>
@lock/<type> <obj>=<key-expression>
@lock/attr <obj>/<attr>
```

Sets a lock on an object. Lock keys are boolean expressions using object refs, `&` (AND), `|` (OR), `!` (NOT).

**Lock type switches:** Each switch sets the corresponding lock type on the object.

| Switch | Lock Type | Controls |
|--------|-----------|----------|
| (default) | Default lock | Picking up objects, passing exits |
| `/enter` | Enter lock | Entering the object |
| `/leave` | Leave lock | Leaving the object |
| `/use` | Use lock | Using the object |
| `/give` | Give lock | Giving the object to someone |
| `/receive` | Receive lock | Receiving objects |
| `/drop` | Drop lock | Dropping the object |
| `/page` | Page lock | Paging the player |
| `/speech` | Speech lock | Speaking in the room |
| `/tport` `/teleport` | Teleport lock | Teleporting the object |
| `/telout` | Telout lock | Teleporting out of the location |
| `/link` | Link lock | Linking to the object |
| `/parent` | Parent lock | Setting the object as parent |
| `/chown` | Chown lock | Chowning the object |
| `/dark` | Dark lock | Dynamic darkness based on evaluator |
| `/control` | Control lock | Additional control permission |
| `/known` | Known lock | Perception: known presence |
| `/heard` | Heard lock | Perception: heard presence |
| `/moved` | Moved lock | Perception: movement seen |
| `/knows` | Knows lock | Perception: subject knows |
| `/hears` | Hears lock | Perception: subject hears |
| `/moves` | Moves lock | Perception: subject sees movement |
| `/user` | User lock | General-purpose user lock |
| `/attr` | Attribute lock | Lock an individual attribute (AF_LOCK) |

**Permissions:** Must control the target.

**Example:**
```
> @lock North=#1234
Locked.
> @lock/enter spaceship=adapted&!banned
Locked.
> @lock/attr me/SECRET_DATA
Attribute locked.
```

### @unlock

```
@unlock <obj>
@unlock/<type> <obj>
@unlock/attr <obj>/<attr>
```

Removes a lock. Accepts all the same type switches as `@lock`.

---

## Administration

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| @teleport | `@teleport [<obj>=]<dest>` | Teleport object to destination | ✅ |
| @force | `@force <obj>=<command>` | Force object to execute command | ✅ |
| @trigger | `@trigger <obj>/<attr>[=<args>]` | Trigger an attribute as action | ✅ |
| @wait | `@wait <secs>=<command>` | Delay command execution | ✅ |
| @notify | `@notify <obj>[/<attr>][=<count>]` | Release semaphore waiters | ✅ |
| @halt | `@halt [<obj>]` | Clear queued commands | ✅ |
| @boot | `@boot <player>` | Disconnect a player | ✅ |
| @toad | `@toad <player>[=<recipient>]` | Convert player to thing | ✅ |
| @wall | `@wall <message>` | Broadcast to all | ✅ |
| @newpassword | `@newpassword <player>=<pass>` | Set player's password | ✅ |
| @pcreate | `@pcreate <name>=<password>` | Create a new player | ✅ |
| @botcreate | `@botcreate <name>` | Create a bot player with API key | 🆕 |
| @find | `@find <pattern>` | Search for objects by name | ✅ |
| @stats | `@stats [<player>]` | Show database statistics | ✅ |
| @ps | `@ps` | Show queue entries | ✅ |
| @dump | `@dump` | Create database archive (alias for @archive) | ✅ |
| @fixdb | `@fixdb <#dbref>` | Rebuild content/exit chains | ✅ |
| @fixall | `@fixall` | Rebuild all chains in database | ✅ |
| @backup | `@backup [<path>]` | Create bolt database backup | ✅ |
| @readcache | `@readcache` | Reload text file cache | ✅ |
| @archive | `@archive` | Create full game archive | 🆕 |
| @admin | `@admin <param>=<value>` | Set runtime config parameters | ✅ |
| @power | `@power <obj>=<power>` | Set/clear powers on object | ✅ |
| @quota | `@quota [<player>][=<amount>]` | View or set build quotas | ✅ |
| @dbck | `@dbck` | Run database consistency check | ✅ |
| @sweep | `@sweep` | Scan for listeners in room | ✅ |
| @search | `@search [<filters>]` | Advanced object search | ✅ |
| @entrances | `@entrances [<obj>]` | Find exits/links pointing to object | ✅ |
| @decompile | `@decompile <obj>` | Output object as build commands | ✅ |
| @hook | `@hook/<type> <cmd>=<attr>` | Set command hooks | ✅ |
| @instance | `@instance/<switch> <args>` | Manage room instances | 🆕 |
| @list | `@list <option>` | List game configuration | ✅ |
| @apikey | `@apikey generate\|revoke <obj>` | Manage API keys | 🆕 |
| @dictionary | `@dictionary <word>` | Look up word in spellcheck dictionary | 🆕 |

### @teleport

```
@teleport <destination>
@teleport <victim>=<destination>
@teleport/quiet <victim>=<dest>
@teleport/loud <victim>=<dest>
```

Teleports an object to a new location. Fires the full C TinyMUSH teleport sequence: OXTPORT, LEAVE/OLEAVE/ALEAVE, move, TPORT/OTPORT/ATPORT, MOVE/OMOVE/AMOVE, ENTER/OENTER/AENTER.

**Switches:**
- `/quiet` — suppress all enter/leave messages
- `/loud` — force messages even for DARK objects

**Permissions:** Must control the victim (or be wizard/TEL_UNRESTRICTED). Destination must be JUMP_OK, controlled, or wizard/TEL_ANYWHERE. FIXED flag blocks non-wizard teleport. Checks tport lock on victim and telout lock on source.

### @force

```
@force <obj>=<command>
@force/now <obj>=<command>
```

Forces an object to execute a command as if they typed it.

**Switches:** `/now` — execute immediately instead of queueing.

**Permissions:** Must control the target.

**Example:**
```
> @force puppet=say Hello!
```

### @trigger

```
@trigger <obj>/<attr>[=<arg0>,<arg1>,...]
@trigger/now <obj>/<attr>[=<args>]
@trigger/quiet <obj>/<attr>[=<args>]
```

Executes the contents of an attribute as a command.

**Switches:**
- `/now` — execute immediately instead of queueing
- `/quiet` — suppress "Triggered." feedback

**Permissions:** Must control the object.

**Example:**
```
> @trigger me/GREET=Hello,World
Triggered.
```

### @wait

```
@wait <seconds>=<command>
@wait <obj>=<command>
@wait <obj>/<attr>=<command>
```

Delays execution of a command. When given an object, acts as a semaphore — the command waits until `@notify` releases it. The `/<attr>` form uses a named semaphore attribute instead of the default A_SEMAPHORE.

### @notify

```
@notify <obj>[/<attr>][=<count>]
@notify/all <obj>[/<attr>]
@notify/first <obj>[/<attr>]
```

Releases waiters on an object's semaphore.

**Switches:**
- `/all` — wake all waiting entries
- `/first` — wake first entry (default behavior)

**Permissions:** Must control the object or object must be LINK_OK.

### @halt

```
@halt [<obj>]
@halt/all
```

Removes queued commands for an object (or yourself if no argument).

**Switches:** `/all` — halt ALL objects' queue entries (requires `halt` power).

Note: `@halt` only clears queue entries. It does NOT set the HALT flag (use `@set obj=HALT` for that).

### @boot

```
@boot <player>
@boot/port <port-number>
@boot/quiet <player>
```

Disconnects a player from the game.

**Switches:**
- `/port` — boot by port number instead of player name
- `/quiet` — suppress "You have been booted" message to victim

**Permissions:** Requires `boot` power. Cannot boot God or yourself.

### @toad

```
@toad <player>[=<recipient>]
```

Converts a player into a thing named "a slimy toad named <Name>". All owned objects are transferred to the recipient (or the executing wizard).

**Permissions:** Wizard only. Cannot toad God, yourself, or wizards (unless God).

### @pcreate

```
@pcreate <name>=<password>
```

Creates a new player without logging them in. Places the player at the start room.

**Permissions:** Wizard only.

### @botcreate 🆕

```
@botcreate <name>
```

Creates a bot player with the ROBOT flag and generates an API key. Bots authenticate via API key only, never interactively. Placed at the creator's location.

**Permissions:** Wizard only.

**Example:**
```
> @botcreate Cormac
Bot player Cormac created as #13364 with ROBOT flag.
API Key: a1b2c3d4...
Store this key securely - it will not be shown again.
```

### @archive 🆕

```
@archive
@archive/list
```

Creates a comprehensive game archive including bolt database snapshot, SQL checkpoint, config files, text files, dictionary files, and alias configs. Archives are stored in the `backups/` directory.

**Switches:** `/list` — list existing archives with sizes and timestamps.

**Permissions:** Wizard only.

### @stats

```
@stats
@stats/all
@stats/me
@stats/player <name>
```

Shows database statistics (object counts by type).

**Switches:**
- `/all` — full database stats (default for wizards)
- `/me` — stats for objects you own
- `/player` — stats for objects owned by specified player

**Permissions:** Wizard or `stat_any` power (non-wizards can use `/me`).

### @ps

```
@ps
@ps/all
@ps/brief
@ps/long
```

Shows queued command entries.

**Switches:**
- `/all` — show all queue entries (wizard)
- `/brief` — abbreviated output
- `/long` — detailed output

### @search

```
@search type=<type> name=<pattern> flags=<flags> owner=<player> zone=<zone> parent=<parent>
```

Advanced object search with multiple filter criteria. Supports type, name pattern, flag matching, owner, zone, and parent filters.

**Permissions:** Wizard or `search` power (non-wizards see only owned objects).

### @entrances

```
@entrances [<obj>]
```

Lists all exits, links, and droptos pointing to an object (or current room if no argument).

### @decompile

```
@decompile <obj>
@decompile <obj>/<attr-pattern>
@decompile/pretty <obj>
```

Outputs an object as a series of build commands that can recreate it.

**Switches:** `/pretty` — add blank lines between entries for readability.

### @hook

```
@hook/before <command>=<obj>/<attr>
@hook/after <command>=<obj>/<attr>
@hook/override <command>=<obj>/<attr>
@hook/ignore <command>=<obj>/<attr>
@hook/list [<command>]
@hook/clear <command>
```

Sets hooks that fire before, after, or instead of built-in commands.

**Switches:**
- `/before` — fire attribute before the command
- `/after` — fire attribute after the command
- `/override` — replace the command entirely
- `/ignore` — cancel the command silently
- `/list` — show defined hooks
- `/clear` — remove all hooks for a command

**Permissions:** Wizard only. Requires `hooks_enabled` config.

### @instance 🆕

```
@instance/create <template>[=<name>]
@instance/destroy <instance>
```

Creates or destroys room instances. Clones a template THING and all its interior rooms/exits, remapping references.

**Switches:**
- `/create` — create an instance from a template
- `/destroy` — destroy an instance and its interior

**Permissions:** Must control the template. Requires `instances_enabled` config.

### @list

```
@list <option>
```

Lists game configuration. Available options:
- `functions` — all built-in and user-defined functions
- `commands` — all registered commands
- `flags` — all object flags
- `powers` — all powers (wizard)
- `attributes` — built-in attribute names
- `user_attributes` — user-defined attributes (wizard)
- `switches` — command switches
- `options` — game configuration parameters
- `default_flags` — default flags for new objects
- `permissions` — command permissions (wizard)
- `func_permissions` — function permissions (wizard)
- `costs` — building costs
- `db_stats` — database statistics (wizard)
- `globals` — global configuration (wizard)

### @apikey 🆕

```
@apikey generate <object>
@apikey revoke <object>
```

Generates or revokes an API key for a player or thing object. API keys are used for REST/WebSocket authentication.

**Permissions:** Wizard, or caller has `bot` power and owns the target.

### @dictionary 🆕

```
@dictionary <word>
```

Looks up a word in the spellcheck dictionary. Requires the spellcheck engine to be enabled.

### @power

```
@power <obj>=<power>
@power <obj>=!<power>
```

Sets or clears a power on an object.

Available powers include: `announce`, `boot`, `builder`, `change_quotas`, `chown_anything`, `cloak`, `comm_all`, `control_all`, `find_unfindable`, `free_money`, `free_quota`, `guest`, `halt`, `hide`, `idle`, `link_any_home`, `link_to_anything`, `link_variable`, `long_fingers`, `mdark_attr`, `no_destroy`, `open_anywhere`, `pass_locks`, `poll`, `prog`, `search`, `see_all`, `see_hidden`, `see_queue`, `stat_any`, `steal`, `tel_anywhere`, `tel_unrestricted`, `unkillable`, `use_sql`, `watch`, `wiz_attr`, `wizard_who`, `bot`.

**Permissions:** Wizard only. Some powers (control_all, guest, use_sql, cloak) are God-only.

### @quota

```
@quota
@quota <player>
@quota <player>=<amount>
```

Views or modifies a player's building quota.

**Permissions:** View own quota freely. Set quotas requires wizard or `change_quotas` power.

### @admin

```
@admin <param>=<value>
```

Sets runtime game configuration parameters. See `@list options` for available parameters.

**Permissions:** Wizard only.

### @sweep

```
@sweep
@sweep/commands
@sweep/listen
@sweep/players
@sweep/here
@sweep/exits
@sweep/inventory
```

Scans the current location for objects with listeners, $commands, connected players, puppets, and audible flags.

**Switches:** Each switch limits the scan to that category. No switches means scan all.

### @fixdb

```
@fixdb #<dbref>
```

Rebuilds content and exit chains for the entire database (the dbref argument is accepted for compatibility but a full rebuild runs regardless).

**Permissions:** God only.

### @fixall

```
@fixall
```

Rebuilds all content and exit chains across the entire database.

**Permissions:** God only.

### @readcache

```
@readcache
```

Reloads all text files (connect.txt, motd.txt, etc.) from the text directory.

**Permissions:** Wizard only.

### @backup

```
@backup [<filename>]
```

Creates a backup of the bolt database. Default filename includes timestamp.

**Permissions:** Wizard only.

### @dbck

```
@dbck
```

Runs database consistency check: purges GOING objects and repairs all content/exit chains.

**Permissions:** Wizard only.

---

## Evaluation

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| @eval | `@eval <expression>` | Evaluate a softcode expression | ✅ |
| @switch | `@switch <expr>=<pat1>,<act1>,...` | Conditional branching | ✅ |
| @dolist | `@dolist <list>=<command>` | Iterate over a list | ✅ |
| @program | `@program <obj>/<attr>` | Enter interactive attribute editor | ✅ |
| @quitprogram | `@quitprogram` | Exit interactive editor | ✅ |
| @function | `@function <name>=<obj>/<attr>` | Define a softcode function | ✅ |
| @drain | `@drain <obj>[/<attr>]` | Drain semaphore wait queue | ✅ |
| @verb | `@verb <obj>=<actor>,<attrs...>` | Generic did_it with named attributes | ✅ |

### @switch

```
@switch <expr>=<pat1>,<act1>[,<pat2>,<act2>,...][,<default>]
@switch/all <expr>=<pat1>,<act1>,...
@switch/first <expr>=<pat1>,<act1>,...
```

Evaluates an expression and compares it against patterns. Executes the action for the first matching pattern (or all with `/all`). An odd trailing entry acts as the default case. `#$` in action bodies is replaced with the evaluated expression.

**Switches:**
- `/all` — fire ALL matching cases
- `/first` — stop at first match (default)
- `/now` — execute immediately (default in Go)

Patterns support wildcards (`*`, `?`) and numeric comparisons (`>N`, `<N`, `>=N`, `<=N`).

**Example:**
```
> @switch 5=>3,{say Greater},{say Not greater}
```

### @dolist

```
@dolist <list>=<command>
@dolist/delimit <sep> <list>=<command>
@dolist/space <list>=<command>
@dolist/now <list>=<command>
@dolist/notify <list>=<command>
```

Iterates over elements of a list, executing a command for each. `##` in the command is replaced with the current element, `#@` with the 1-based iteration number.

**Switches:**
- `/delimit` — use a custom delimiter (first token after switch)
- `/space` — use literal space as delimiter
- `/now` — execute each iteration immediately instead of queueing
- `/notify` — queue `@notify me` after loop completes (for semaphore synchronization)

**Example:**
```
> @dolist apple banana cherry=say I have a ##.
```

### @function

```
@function <name>=<obj>/<attr>
@function/privileged <name>=<obj>/<attr>
@function/preserve <name>=<obj>/<attr>
@function/delete <name>
@function
```

Defines a user function backed by a softcode attribute.

**Switches:**
- `/privileged` — function runs with wizard permissions
- `/preserve` — preserve q-registers across calls
- `/delete` — remove a function definition
- (no args) — list all defined functions

**Permissions:** Wizard only.

### @drain

```
@drain <obj>[/<attr>]
```

Removes all semaphore wait entries from the queue for an object and resets its semaphore counter.

**Permissions:** Must control the target.

### @verb

```
@verb <obj>=<actor>,<what>,<whatdef>,<owhat>,<owhatdef>,<awhat>,{<args>}
```

Generic `did_it` mechanism. Evaluates named attributes on `<obj>` and sends messages to actor (what), room (owhat), and queues an action (awhat). Default text is used when the attribute is empty.

**Permissions:** Must control the actor.

### @program

```
@program <obj>/<attr>
```

Enters interactive mode for editing an attribute. Each line you type is appended. Use `@quitprogram` or `.` to exit.

**Permissions:** Must control the object. Requires `prog` power.

### @quitprogram

```
@quitprogram
```

Exits interactive `@program` mode, saving the accumulated text to the attribute.

---

## Attribute Management

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| @attribute | `@attribute/<switch> <args>` | Manage attribute definitions | ✅ |
| @attlist | `@attlist [<pattern>]` | List attribute definitions | ✅ |
| @cpattr | `@cpattr <src>=<dst>[,<dst>...]` | Copy attributes between objects | ✅ |
| @mvattr | `@mvattr <obj>=<src>,<dst>[,...]` | Move attributes on an object | ✅ |

### @attribute

```
@attribute/access <attr>=<flags>
@attribute/rename <old>=<new>
@attribute/delete <attr>
@attribute/propagate <attr>=<parent>
```

Manages user-defined attribute definitions.

**Switches:**
- `/access` — set attribute access flags
- `/rename` — rename an attribute definition
- `/delete` — delete an attribute definition
- `/propagate` — set propagation from parent to children

**Permissions:** Wizard only.

### @attlist

```
@attlist [<pattern>]
```

Lists attribute definitions, optionally filtered by wildcard pattern.

### @cpattr

```
@cpattr <obj>/<attr>=<dst-obj>[/<dst-attr>][,<dst-obj>[/<dst-attr>],...]
```

Copies an attribute value to one or more destinations. If the destination has no `/<attr>`, uses the source attribute name.

**Permissions:** Must control destination objects.

### @mvattr

```
@mvattr <obj>=<src-attr>,<dst-attr>[,<dst-attr>,...]
```

Copies a source attribute to destination attributes on the same object, then clears the source.

**Permissions:** Must control the object.

---

## Attribute Setters

All attribute setters follow the pattern: `@<cmd> <obj>=<text>`. Setting empty text clears the attribute. All require guest restriction (no guests).

### Success/Failure Messages

| Command | Attribute | Seen By | When |
|---------|-----------|---------|------|
| @success | SUCC | Actor | Lock passes |
| @osuccess | OSUCC | Room | Lock passes |
| @asuccess | ASUCC | (action) | Lock passes |
| @fail | FAIL | Actor | Lock fails |
| @ofail | OFAIL | Room | Lock fails |
| @afail | AFAIL | (action) | Lock fails |

### Enter/Leave Messages

| Command | Attribute | Seen By | When |
|---------|-----------|---------|------|
| @enter | ENTER | Actor | Entering |
| @oenter | OENTER | Room | Entering |
| @oxenter | OXENTER | Old room | Entering |
| @aenter | AENTER | (action) | Entering |
| @leave | LEAVE | Actor | Leaving |
| @oleave | OLEAVE | Room | Leaving |
| @aleave | ALEAVE | (action) | Leaving |
| @oxleave | OXLEAVE | New room | Leaving |

### Drop Messages

| Command | Attribute | Seen By | When |
|---------|-----------|---------|------|
| @drop | DROP | Actor | Dropping |
| @odrop | ODROP | Room | Dropping |
| @adrop | ADROP | (action) | Dropping |

### Kill Messages

| Command | Attribute | Seen By | When |
|---------|-----------|---------|------|
| @kill | KILL | Actor | Killing |
| @okill | OKILL | Room | Killing |
| @akill | AKILL | (action) | Killing |

### Use Messages

| Command | Attribute | Seen By | When |
|---------|-----------|---------|------|
| @use | USE | Actor | Using |
| @ouse | OUSE | Room | Using |
| @ause | AUSE | (action) | Using |

### Move Messages

| Command | Attribute | Seen By | When |
|---------|-----------|---------|------|
| @move | MOVE | Actor | Teleporting |
| @omove | OMOVE | Room | Teleporting |
| @amove | AMOVE | (action) | Teleporting |

### Description Variants

| Command | Attribute | Description |
|---------|-----------|-------------|
| @describe | DESC | Object description (also a building command) |
| @odescribe | ODESC | Others-see description |
| @adescribe | ADESC | Action on describe |
| @idesc | IDESC | Inside description (for containers) |

### Payment Messages

| Command | Attribute | Description |
|---------|-----------|-------------|
| @pay | PAY | Message to payer |
| @opay | OPAY | Message to room on payment |
| @apay | APAY | Action on payment |
| @cost | COST | Cost to use the object |

### Teleport Messages

| Command | Attribute | Seen By | When |
|---------|-----------|---------|------|
| @tport | TPORT | Actor | Being teleported |
| @otport | OTPORT | New room | Being teleported |
| @oxtport | OXTPORT | Old room | Being teleported |
| @atport | ATPORT | (action) | Being teleported |

### Enter/Leave/Use Failure Messages

| Command | Attribute | Seen By | When |
|---------|-----------|---------|------|
| @efail | EFAIL | Actor | Enter lock fails |
| @oefail | OEFAIL | Room | Enter lock fails |
| @aefail | AEFAIL | (action) | Enter lock fails |
| @lfail | LFAIL | Actor | Leave lock fails |
| @olfail | OLFAIL | Room | Leave lock fails |
| @alfail | ALFAIL | (action) | Leave lock fails |
| @ufail | UFAIL | Actor | Use lock fails |
| @oufail | OUFAIL | Room | Use lock fails |
| @aufail | AUFAIL | (action) | Use lock fails |

### Sensory Attribute Setters

| Command | Attribute | Description |
|---------|-----------|-------------|
| @smell | SMELL | Smell description (actor) |
| @osmell | OSMELL | Smell description (room) |
| @asmell | ASMELL | Action on smell |
| @touch | TOUCH | Touch description (actor) |
| @otouch | OTOUCH | Touch description (room) |
| @atouch | ATOUCH | Action on touch |
| @taste | TASTE | Taste description (actor) |
| @otaste | OTASTE | Taste description (room) |
| @ataste | ATASTE | Action on taste |
| @sound | SOUND | Sound description (actor) |
| @osound | OSOUND | Sound description (room) |
| @asound | ASOUND | Action on sound |

### Format Overrides

| Command | Attribute | Description |
|---------|-----------|-------------|
| @conformat | CONFORMAT | Custom contents display format |
| @exitformat | EXITFORMAT | Custom exits display format |
| @nameformat | NAMEFORMAT | Custom name display format |
| @roomformat | ROOMFORMAT | Custom room display format |

### Miscellaneous Attribute Setters

| Command | Attribute | Description |
|---------|-----------|-------------|
| @sex | SEX | Player sex/gender |
| @alias | ALIAS | Object alias |
| @away | AWAY | Away message (for pages) |
| @idle | IDLE | Idle message |
| @listen | LISTEN | Listen pattern (for ^-pattern matching) |
| @ahear | AHEAR | Action on hearing a match |
| @startup | STARTUP | Command executed on server startup |
| @daily | DAILY | Command executed daily |
| @charges | CHARGES | Number of uses remaining |
| @runout | RUNOUT | Action when charges reach zero |
| @reject | REJECT | Page rejection message |
| @ealias | EALIAS | Enter aliases (semicolon-separated) |
| @lalias | LALIAS | Leave aliases (semicolon-separated) |
| @filter | FILTER | Output filter pattern |
| @infilter | INFILTER | Input filter pattern |
| @forwardlist | FORWARDLIST | Forward messages to listed objects |
| @prefix | PREFIX | Output prefix text |
| @inprefix | INPREFIX | Input prefix text |

---

## SQL

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| @sql | `@sql <query>` | Execute a SQL query | 🆕 |
| @sqlinit | `@sqlinit` | Initialize SQL database | 🆕 |
| @sqldisconnect | `@sqldisconnect` | Disconnect SQL database | 🆕 |

### @sql

```
@sql <query>
```

Executes a SQL query against the game's SQLite database. Returns results in tabular format.

**Permissions:** Requires `use_sql` power.

### @sqlinit

```
@sqlinit
```

Initializes or reconnects the SQL database.

**Permissions:** Wizard only.

### @sqldisconnect

```
@sqldisconnect
```

Disconnects the SQL database connection.

**Permissions:** Wizard only.

---

## Session

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| QUIT | `QUIT` | Disconnect from the game | ✅ |
| LOGOUT | `LOGOUT` | Return to login screen | 🆕 |
| @doing | `@doing <text>` | Set your DOING message | ✅ |
| @password | `@password <old>=<new>` | Change your password | ✅ |
| @version | `@version` | Show server version | ✅ |
| @uptime | `@uptime` | Show server uptime | ✅ |
| @motd | `@motd [<text>]` | View or set message of the day | ✅ |

### QUIT

```
QUIT
```

Disconnects from the game, closing the connection.

### LOGOUT 🆕

```
LOGOUT
```

Disconnects the character but keeps the socket open, returning to the login screen. Allows reconnecting as a different character without dropping the TCP connection.

### @password

```
@password <old-password>=<new-password>
```

Changes your own password. Must provide the correct current password.

### @motd

```
@motd
@motd <message>
@motd/wizard [<message>]
@motd/down [<message>]
@motd/full [<message>]
```

Views or sets the message of the day. Without arguments, displays the current MOTD.

**Switches:**
- `/wizard` — wizard MOTD
- `/down` — down MOTD (shown when server is going down)
- `/full` — full MOTD (shown when game is full)

**Permissions:** Setting MOTD requires wizard.

---

## Help System

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| help | `help [<topic>]` | Player help | ✅ |
| wizhelp | `wizhelp [<topic>]` | Wizard help | ✅ |
| qhelp | `qhelp [<topic>]` | Quick help | ✅ |
| news | `news [<topic>]` | Game news | ✅ |
| man | `man [<topic>]` | MUSH manual | ✅ |
| wiznews | `wiznews [<topic>]` | Wizard news | ✅ |
| +jhelp | `+jhelp [<topic>]` | Jobs help | 🆕 |

All help commands search their respective text files for matching topics. Without a topic, they display the default entry.

---

## Sensory Commands

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| smell | `smell [<obj>]` | Smell an object or the room | ✅ |
| touch | `touch [<obj>]` | Touch an object or the room | ✅ |
| taste | `taste [<obj>]` | Taste an object or the room | ✅ |
| listen | `listen [<obj>]` | Listen to an object or the room | ✅ |

These query commands check the SMELL/TOUCH/TASTE/SOUND attribute on the target (or room if no argument). If the attribute is set, it displays the message and fires the O-variant (to room) and A-variant (action). If not set, displays a default message (e.g., "You don't smell anything special.").

---

## Player Object Commands

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| get / take | `get <object>` | Pick up an object | ✅ |
| drop | `drop <object>` | Drop an object from inventory | ✅ |
| give | `give <player>=<amount\|object>` | Give money or objects | ✅ |
| score | `score` | Show your money | ✅ |
| inventory | `inventory` | List carried objects | ✅ |
| look | `look [<obj>]` | Look at something | ✅ |
| examine | `examine [<obj>]` | Examine an object in detail | ✅ |
| use | `use <object>` | Use an object | ✅ |
| kill | `kill <player>=<cost>` | Attempt to kill a player | ✅ |
| slay | `slay <player>` | Wizard guaranteed kill | ✅ |
| @poor | `@poor <amount>` | Set all players' money | ✅ |
| WHO | `WHO` | List connected players | ✅ |
| DOING | `DOING` | List connected players with DOING | ✅ |

### get / take

```
get <object>
take <object>
```

Picks up a THING from the current room into your inventory. Checks the default lock on the object. Fires SUCC/OSUCC/ASUCC on success, FAIL/OFAIL/AFAIL on failure.

### drop

```
drop <object>
```

Drops an object from your inventory into the current room. Checks drop lock. Fires DROP/ODROP/ADROP.

### give

```
give <recipient>=<amount>
give <recipient>=<object>
```

Gives money (pennies) or an object to another player. For objects, the recipient must be ENTER_OK or controlled by the giver. Checks give-lock and receive-lock.

### look

```
look [<object>]
look here
look/outside
```

Displays the description of an object, or the current room if no argument.

**Switches:** `/outside` — look at the room containing your current location (useful from inside containers/vehicles).

### examine

```
examine [<object>]
examine <obj>/<attr-pattern>
examine/brief <obj>
examine/parent <obj>
```

Shows detailed information about an object including flags, attributes, contents, and exits.

**Switches:**
- `/brief` — show only the object header
- `/parent` — show attributes inherited from parent chain

**Permissions:** Must be able to examine (own, control, or VISUAL flag). Non-examinable objects fall back to `look` behavior.

### use

```
use <object>
```

Uses an object. Checks the use lock, then fires USE/OUSE/AUSE messages.

### kill

```
kill <player>=<cost>
```

Attempts to kill a player. Higher cost increases success probability. On success, target is sent home and KILL/OKILL/AKILL fire. On failure, the cost is still spent.

**Permissions:** Target must be killable (not have `unkillable` power, not in SAFE room).

### slay

```
slay <player>
```

Wizard-only guaranteed kill with no cost.

**Permissions:** Wizard only.

### @poor

```
@poor <amount>
```

Sets ALL players' pennies to the specified amount. Destructive operation.

**Permissions:** Wizard only.

---

## Channel System (Comsys)

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| addcom | `addcom <alias>=<channel>` | Add a channel alias | ✅ |
| delcom | `delcom <alias>` | Remove a channel alias | ✅ |
| clearcom | `clearcom` | Remove all channel aliases | ✅ |
| comlist | `comlist` | List your channel aliases | ✅ |
| comtitle | `comtitle <alias>=<title>` | Set channel title | ✅ |
| allcom | `allcom <on\|off\|who>` | Control all channels at once | ✅ |
| @ccreate | `@ccreate <channel>` | Create a channel | ✅ |
| @cdestroy | `@cdestroy <channel>` | Destroy a channel | ✅ |
| @clist | `@clist` | List all channels | ✅ |
| @cwho | `@cwho <channel>` | List channel members | ✅ |
| @cboot | `@cboot <channel>=<player>` | Remove player from channel | ✅ |
| @cemit | `@cemit <channel>=<message>` | Emit to a channel | ✅ |
| @cset | `@cset <channel>=<option>` | Set channel options | ✅ |
| @cinfo | `@cinfo <channel>` | Show channel information | ✅ |

---

## Event Bus

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| @queue | `@queue/<switch> [<args>]` | Manage event bus queues | ✅ |

### @queue

```
@queue/create <name>=<options>
@queue/set <name>=<options>
@queue/destroy <name>
@queue/lock <name>=<lockexpr>
@queue/list
@queue/info <name>
@queue/subs <name>
@queue/pubs <name>
@queue/stats [<name>]
@queue/stats/reset [<name>]
@queue/drain <name>
@queue/bus
@queue/alias <name>=<alias>
```

Manages the pub/sub event bus queue system.

**Switches:**
- `/create` — create a new event queue with options (rate, scope, max_subs, enabled)
- `/set` — modify queue options
- `/destroy` — remove a queue
- `/lock` — set access lock on a queue
- `/list` — list all queues
- `/info` — detailed queue information
- `/subs` — list subscribers
- `/pubs` — list publishers
- `/stats` — queue statistics
- `/stats/reset` — reset statistics
- `/drain` — clear pending events
- `/bus` — show bus overview
- `/alias` — set queue alias

**Permissions:** `/create`, `/set`, `/destroy`, `/lock` require wizard.

---

## Mail System

| Command | Syntax | Description | Compat |
|---------|--------|-------------|--------|
| @mail | `@mail [<switches>] [<args>]` | Built-in mail system | ✅ |
| @malias | `@malias [<switches>] [<args>]` | Mail alias management | ✅ |
| - | `- <message>` | Append to mail message being composed | ✅ |
| ~ | `~ <text>` | Mail composition shortcut | ✅ |

See `help @mail` in-game for full mail system documentation.

---

## Special Prefixes

These single-character prefixes are handled before normal command dispatch:

| Prefix | Equivalent | Description |
|--------|-----------|-------------|
| `"` | `say` | Say shortcut |
| `:` | `pose` | Pose shortcut |
| `;` | pose/nospace | Pose without space |
| `&` | `@set obj=attr:val` | Set variable attribute |
| `#` | `@force` | Force by dbref: `#123 command` |
| `\` | `@emit` | Emit shortcut |
