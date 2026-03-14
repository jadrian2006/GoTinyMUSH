# New & Extended Functions

Functions in GoTinyMUSH that are **new** (not present in C TinyMUSH 3.x) or **extended** beyond C behavior. GoTinyMUSH registers 556+ functions total; approximately 170+ are not present in C TinyMUSH 3.1.

---

## Entirely New Systems

### Event Bus

Pub/sub event bus for inter-object communication without direct references.

| Function | Description | Example |
|----------|-------------|---------|
| `publish(queue, data)` | Publish data to a named queue | `publish(sled.tick, [secs()])` |
| `subscribe(queue, obj/attr)` | Subscribe obj/attr to fire when queue publishes | `subscribe(sled.tick, me/ON_TICK)` |
| `unsubscribe(queue, obj/attr)` | Remove subscription | `unsubscribe(sled.tick, me/ON_TICK)` |
| `queues([name])` | List all queues, or with name+`subs`/`stats` get details | `queues(sled.tick subs)` |

### JSON

Full JSON manipulation in softcode.

| Function | Description | Example |
|----------|-------------|---------|
| `json(type[, args...])` | Create JSON value (object, array, string, number, bool, null) | `json(object, key, value)` |
| `json_query(json, op[, path])` | Query JSON: get, exists, type, members, count, isnull | `json_query(%0, get, name)` |
| `json_mod(json, op[, args])` | Modify JSON: set, insert, replace, remove, push, patch | `json_mod(%0, set, hp, 50)` |
| `json_pp(json)` | Pretty-print JSON with indentation | `json_pp({"a":1})` |
| `json_test(str)` | Test if string is valid JSON, returns 1/0 | `json_test({"valid":true})` |
| `json_to_array(json, arrayname)` | Load JSON array into a mutable array | `json_to_array([1,2,3], myarr)` |
| `array_to_json(arrayname[, mode])` | Convert mutable array to JSON array | `array_to_json(myarr)` |
| `stringtojson(str[, mode])` | Convert MUSH string to JSON string | `stringtojson(hello world)` |
| `listtojson(list[, delim])` | Convert MUSH list to JSON array | `listtojson(a b c)` |
| `jsontolist(json[, delim])` | Convert JSON array to MUSH list | `jsontolist([1,2,3])` |
| `jsonescape(str)` | Escape string for JSON embedding | `jsonescape(He said "hi")` |

### Arrays

Per-player mutable arrays (session-scoped, not persisted to attributes).

| Function | Description | Example |
|----------|-------------|---------|
| `array(name[, item...])` | Create array, optionally with initial items | `array(inv, sword, shield)` |
| `adestroy(name)` | Destroy an array | `adestroy(inv)` |
| `apush(name, item[, items...])` | Push items onto end of array | `apush(inv, potion)` |
| `apop(name)` | Remove and return last item | `apop(inv)` |
| `ashift(name)` | Remove and return first item | `ashift(inv)` |
| `aunshift(name, item[, items...])` | Prepend items to front of array | `aunshift(inv, key)` |
| `aget(name, index)` | Get item at index (0-based) | `aget(inv, 0)` |
| `aset(name, index, value)` | Set item at index | `aset(inv, 0, axe)` |
| `alen(name)` | Return array length | `alen(inv)` |
| `alist(name[, delim])` | Return array as delimited list | `alist(inv, |)` |
| `aload(name, list[, delim])` | Load list into array (replaces contents) | `aload(inv, a b c)` |
| `larrays()` | List all arrays owned by caller | `larrays()` |

### SQL (SQLite3)

Embedded SQLite3 database access. Requires `use_sql` power.

| Function | Description | Example |
|----------|-------------|---------|
| `sql(query[, row_delim[, field_delim]])` | Execute SQL query from softcode | `sql(SELECT * FROM players)` |
| `sqlescape(text)` | Escape string for safe SQL interpolation | `sqlescape(O'Brien)` |

### Navigation System

39 functions for 3D grid-based flight, navigation, terrain, weather, and tactical calculations. See [FLIGHT.md](FLIGHT.md) for comprehensive documentation.

**Heading functions:**

| Function | Description | Example |
|----------|-------------|---------|
| `hvec(heading)` | Unit vector for 32-point compass heading | `hvec(0)` -> `1 0` (East) |
| `hdelta(from, to)` | Signed turn delta between headings | `hdelta(0, 12)` -> `12` |
| `hname(heading[, mode])` | Human-readable heading name | `hname(4)` -> `NE` |
| `h2deg(heading)` | Convert heading to degrees | `h2deg(8)` -> `90` |
| `deg2h(degrees)` | Convert degrees to heading | `deg2h(90)` -> `8` |
| `vec2h(dx, dy)` | Convert vector to nearest heading | `vec2h(1, 1)` -> `4` |

**Grid coordinate functions:**

| Function | Description | Example |
|----------|-------------|---------|
| `gridabs(address)` | Grid address to absolute X Y | `gridabs(EL-453-NE)` -> `115 453` |
| `absgrid(x, y)` | Absolute X Y to grid address | `absgrid(115, 453)` -> `EL-453-NE` |
| `griddist(addr1, addr2)` | Distance between two grid addresses | `griddist(AA-0-NE, ZZ-999-NE)` |
| `griddist3d(pos1, pos2)` | 3D distance between space-delimited positions | `griddist3d(0 0 0, 3 4 0)` -> `5` |
| `gridcourse(from, to)` | Heading and distance to destination | `gridcourse(AA-0-NE, EL-453-NE)` |
| `gridlocfull(x, y, z)` | Full location string with grid + altitude | `gridlocfull(100, 200, 50)` |
| `gridparsefull(locstr)` | Parse full location string back to x y z | `gridparsefull(EL-453-NE/50)` |
| `gps(zone[, x, y, z])` | GPS coordinates relative to zone | `gps(#13401, 100, 200, 50)` |

**Navigation functions:**

| Function | Description | Example |
|----------|-------------|---------|
| `gridnav(pos, hdg, spd[, climb[, drift]])` | Project position along heading | `gridnav(100 200 50, 0, 10)` |
| `drift(pos, range)` | Apply random perturbation per axis | `drift(100 200 50, 5 5 1)` |
| `vrand(mag[, mag_y, mag_z])` | Random vector within magnitude bounds | `vrand(5)` |
| `vrandc(mag)` | Random vector constrained to magnitude | `vrandc(5)` |
| `altclamp(alt)` | Clamp altitude to valid range | `altclamp(-50)` -> `0` |

**Tactical functions:**

| Function | Description | Example |
|----------|-------------|---------|
| `bearing(pos1, pos2)` | Heading from pos1 to pos2 | `bearing(100 200, 300 400)` -> `4` |
| `pitch(pos1, pos2)` | Vertical angle between 3D positions | `pitch(0 0 0, 0 0 100)` -> `90` |
| `closing(p1, h1, s1, p2, h2, s2)` | Closing rate between two movers | `closing(0 0, 0, 10, 100 0, 16, 10)` |
| `relvel(h1, s1, h2, s2)` | Relative velocity vector | `relvel(0, 10, 16, 10)` |
| `eta(pos, hdg, spd, target)` | Estimated ticks to reach target | `eta(0 0, 0, 10, 100 0)` -> `10` |
| `intercept(pos, spd, tgt, tgt_hdg, tgt_spd)` | Intercept heading to meet target | `intercept(0 0, 15, 100 0, 8, 10)` |

**Map/POI functions:**

| Function | Description | Example |
|----------|-------------|---------|
| `mapinstance(zone[, field])` | Query map instance data | `mapinstance(#13401, loc)` |
| `mapparse(data, field)` | Parse map instance data field | `mapparse(%0, x)` |
| `poiformat(name, x, y, z[, height[, tags]])` | Format a POI data string | `poiformat(Reef, 100, 200, -30)` |
| `poiparse(poi, field)` | Parse POI data field | `poiparse(%0, name)` |
| `poiinrange(pos, poi, range)` | Test if position is within range of POI | `poiinrange(100 200, %0, 50)` |
| `poidist(pos, poi)` | Distance from position to POI | `poidist(100 200, %0)` |
| `poibearing(pos, poi)` | Heading from position to POI | `poibearing(100 200, %0)` |

**Topology/terrain functions:**

| Function | Description | Example |
|----------|-------------|---------|
| `topology(zone, x, y)` | Terrain elevation at coordinates | `topology(#13401, 100, 200)` |
| `topoflush(zone)` | Clear cached topology data for zone | `topoflush(#13401)` |
| `depth(zone, x, y[, sea_level])` | Water depth at coordinates | `depth(#13401, 100, 200)` |
| `islandchk(zone, x, y[, radius])` | Check if position is on/near land | `islandchk(#13401, 100, 200)` |
| `topomap(zone[, x, y, radius])` | ASCII terrain map around position | `topomap(#13401, 100, 200, 10)` |
| `current(zone, x, y[, opts...])` | Water or air current vector at position | `current(#13401, 100, 200)` |
| `temperature(zone, x, y[, depth])` | Temperature at position and depth | `temperature(#13401, 100, 200)` |

### OOB / GMCP

Out-of-band protocol support for rich clients (BeipMU, Mudlet, etc.).

| Function | Description | Example |
|----------|-------------|---------|
| `oob(player, package, data)` | Send OOB data to player's client | `oob(%#, beip.tilemap.data, %0)` |
| `hasgmcp(player)` | Test if player's client supports GMCP | `hasgmcp(%#)` |
| `gmcppackages(player)` | List GMCP packages player has negotiated | `gmcppackages(%#)` |
| `hasmsdp(player)` | Test if player's client supports MSDP | `hasmsdp(%#)` |

### Bot / API Key

Support for bot players with WebSocket API key authentication.

| Function | Description | Example |
|----------|-------------|---------|
| `hasapikey(player)` | Test if player has an API key set | `hasapikey(#13364)` |
| `isapikey(player)` | Alias for hasapikey | `isapikey(#13364)` |

### Connection Logging

| Function | Description | Example |
|----------|-------------|---------|
| `connlog(player[, count])` | Connection log entries for player | `connlog(%#, 5)` |
| `addrlog(player[, count])` | Address/IP log entries for player | `addrlog(%#, 5)` |

---

## Extended Functions (exist in C but enhanced)

| Function | Enhancement | Example |
|----------|-------------|---------|
| `log(value[, base])` | Added optional base argument (C 3.1 only supports natural log) | `log(100, 10)` -> `2` |
| `lastcreate(obj[, type])` | Added type argument to match C 3.3 semantics (returns last-created dbref by type R/E/T/P) | `lastcreate(me, T)` |
| `ldelete(list, pos[, pos2...][, delim])` | Supports multi-position deletion (C only supports single position) | `ldelete(a b c d, 2 4)` |
| `timefmt(fmt[, secs])` | Supports both `$Y` (C 3.1 style) and `%Y` (standard strftime) format codes | `timefmt($Y)` or `timefmt(%Y)` |
| `lcon(obj[, type])` | Added optional type filter argument | `lcon(here, PLAYER)` |
| `lexits(obj[, type])` | Added optional type filter argument | `lexits(here)` |
| `vmul(v1, v2)` | Supports scalar-first, vector-first, and elementwise multiplication | `vmul(3, 1 2 3)` -> `3 6 9` |
| `search([restrictions])` | Enhanced with additional search classes | `search(type=THING zone=#100)` |

---

## New Conditionals

| Function | Description | Example |
|----------|-------------|---------|
| `iffalse(cond, false_result[, true_result])` | Execute false_result when cond is false | `iffalse(0, yes, no)` -> `yes` |
| `iftrue(cond, true_result[, false_result])` | Execute true_result when cond is true | `iftrue(1, yes, no)` -> `yes` |
| `ifzero(cond, zero_result[, nonzero_result])` | Execute zero_result when cond is 0 | `ifzero(0, zero, other)` -> `zero` |
| `isfalse(value)` | Returns 1 if value is false/0/empty | `isfalse(0)` -> `1` |
| `istrue(value)` | Returns 1 if value is true/nonzero/nonempty | `istrue(hello)` -> `1` |
| `usetrue(obj/attr, value)` | Call u-function only if value is true, return value | `usetrue(me/FN, %0)` |
| `usefalse(obj/attr, value)` | Call u-function only if value is false, return value | `usefalse(me/FN, %0)` |
| `udefault(obj/attr, default[, args...])` | Call u-function, return default if result is empty | `udefault(me/FN, N/A, %0)` |
| `reswitch(val, pat, res[, pat, res...][, def])` | Regex switch (first match) | `reswitch(%0, ^a, alpha, ^b, beta)` |
| `reswitchall(val, pat, res[, ...])` | Regex switch (all matches) | `reswitchall(%0, a, has-a, b, has-b)` |
| `reswitchi(val, pat, res[, ...])` | Case-insensitive regex switch | `reswitchi(%0, ^a, alpha)` |
| `reswitchalli(val, pat, res[, ...])` | Case-insensitive regex switch (all matches) | `reswitchalli(%0, a, A)` |

---

## New Iteration

| Function | Description | Example |
|----------|-------------|---------|
| `while(eval_fn, cond_fn, list, cond_val[, isep[, osep]])` | Iterate list while condition matches | `while(me/FN, me/CHK, a b c, 1)` |
| `until(cond, body, initial[, delim])` | Loop body until condition is true | `until(gt(##,10), add(##,1), 0)` |
| `loop(start, end, step, body[, osep])` | Numeric for-loop | `loop(1, 5, 1, mul(##,##))` -> `1 4 9 16 25` |
| `list(list, body[, delim[, osep]])` | Iterate list, output body for each | `list(a b c, ucstr(##))` |
| `list2(l1, l2, body[, d1[, d2[, osep]]])` | Parallel iterate two lists | `list2(1 2, a b, cat(##,#+))` |
| `iter2(l1, l2, body[, d1[, d2[, osep]]])` | Parallel iterate (iter-style) | `iter2(1 2, a b, ##-#+)` |
| `itext2(level)` | Secondary iterator text at nesting level | `itext2(0)` |
| `whentrue(list, body[, delim[, osep]])` | Output body only when result is true | `whentrue(1 0 1, ##)` -> `1 1` |
| `whenfalse(list, body[, delim[, osep]])` | Output body only when result is false | `whenfalse(1 0 1, ##)` -> `0` |
| `whentrue2(l1, l2, body[, ...])` | Parallel whentrue | `whentrue2(1 0, a b, ##)` |
| `whenfalse2(l1, l2, body[, ...])` | Parallel whenfalse | `whenfalse2(1 0, a b, #+)` |
| `filterbool(obj/attr, list[, delim[, osep]])` | Filter list by boolean function result | `filterbool(me/IS_EVEN, 1 2 3 4)` |

---

## New String Functions

| Function | Description | Example |
|----------|-------------|---------|
| `spell(text[, mode])` | Spellcheck with ANSI highlighting (mode `g` for grammar) | `spell(teh quik fox)` |
| `spellcheck(text)` | Spellcheck returning structured error data | `spellcheck(teh)` |
| `html_escape(text)` | Escape HTML entities (`<` -> `&lt;`) | `html_escape(<b>hi</b>)` |
| `html_unescape(text)` | Unescape HTML entities | `html_unescape(&lt;b&gt;)` |
| `url_escape(text)` | URL-encode string | `url_escape(hello world)` -> `hello%20world` |
| `url_unescape(text)` | URL-decode string | `url_unescape(hello%20world)` |
| `printf(format, args...)` | Printf-style string formatting | `printf(%-10s %5d, Bob, 42)` |
| `strdistance(str1, str2)` | Levenshtein edit distance | `strdistance(kitten, sitting)` -> `3` |
| `strlenvis(text)` | Visible length (ignoring ANSI codes) | `strlenvis(ansi(r,hello))` -> `5` |
| `caplist(list[, delim])` | Capitalize first letter of each word | `caplist(hello world)` -> `Hello World` |
| `spellnum(number)` | Spell out number in English | `spellnum(42)` -> `forty-two` |
| `garble(text[, pct[, mode]])` | Randomly garble text | `garble(hello, 50)` |
| `tr(text, from, to)` | Character transliteration | `tr(hello, helo, HELO)` -> `HELLO` |
| `strip(text[, chars])` | Strip specified characters | `strip(hello!, !)` -> `hello` |
| `asc(char)` | ASCII code of character | `asc(A)` -> `65` |
| `chr(code)` | Character from ASCII code | `chr(65)` -> `A` |
| `soundex(text)` | Soundex phonetic code | `soundex(Robert)` -> `R163` |
| `soundlike(str1, str2)` | Compare soundex codes | `soundlike(Robert, Rupert)` -> `1` |

---

## New Object Functions

| Function | Description | Example |
|----------|-------------|---------|
| `isinstance(obj)` | Test if object is an instance | `isinstance(#500)` |
| `irooms(instance)` | List rooms in an instance | `irooms(#500)` |
| `ivehicle(instance)` | Get vehicle object of instance | `ivehicle(#500)` |
| `lrooms(obj[, depth[, type]])` | List rooms reachable from obj | `lrooms(here, 3)` |
| `lcmds(obj[, pattern])` | List `$`-commands on object | `lcmds(me)` |
| `scan_zone(zone[, type])` | List objects in a zone by type | `scan_zone(#100, PLAYER)` |
| `hasflags(obj, flag_expr)` | Test flag expression (OR-of-AND groups) | `hasflags(me, WIZARD DARK)` |
| `zones(obj)` | List zone chain for object | `zones(#500)` |
| `playmem(player)` | Memory usage of player's objects | `playmem(%#)` |
| `objid(obj)` | Persistent object ID (survives recycling) | `objid(me)` |
| `createtime(obj)` | Creation timestamp of object | `createtime(me)` |
| `attrcnt(obj[, pattern])` | Count attributes on object | `attrcnt(me)` |
| `isobjid(str)` | Test if string is a valid objid | `isobjid(#123:456)` |

---

## New Math Functions

| Function | Description | Example |
|----------|-------------|---------|
| `between(val, low, high)` | Test if value is in range | `between(5, 1, 10)` -> `1` |
| `bound(val, low, high)` | Clamp value to range | `bound(15, 0, 10)` -> `10` |
| `avg(num1, num2...)` | Average of values | `avg(10, 20, 30)` -> `20` |
| `median(num1, num2...)` | Median of values | `median(1, 5, 3)` -> `3` |
| `fmod(x, y)` | Floating-point modulo | `fmod(5.5, 2.0)` -> `1.5` |
| `cosh(x)` / `sinh(x)` / `tanh(x)` | Hyperbolic trig functions | `cosh(1)` |
| `tobin(n)` / `todec(s)` / `tohex(n)` / `tooct(n)` | Base conversion | `tohex(255)` -> `FF` |
| `roman(n)` | Convert to Roman numerals | `roman(42)` -> `XLII` |
| `nand(a, b...)` / `nor(a, b...)` / `xnor(a, b...)` | Additional logic gates | `nand(1, 1)` -> `0` |
| `alphamax(str...)` / `alphamin(str...)` | Alphabetic max/min | `alphamax(apple, banana)` -> `banana` |
| `floordiv(a, b)` | Floor division | `floordiv(7, 2)` -> `3` |

---

## New List Functions

| Function | Description | Example |
|----------|-------------|---------|
| `lavg(list[, delim])` | Average of numeric list | `lavg(10 20 30)` -> `20` |
| `lsub(list[, delim])` | Sequential subtraction of list | `lsub(100 30 20)` -> `50` |
| `lmul(list[, delim])` | Product of numeric list | `lmul(2 3 4)` -> `24` |
| `ldiv(list[, delim])` | Sequential division of list | `ldiv(100 5 2)` -> `10` |
| `listmatch(list, pattern[, delim])` | List items matching wildcard | `listmatch(a1 b2 a3, a*)` -> `a1 a3` |
| `nummatch(list, num[, delim])` | Count numeric matches | `nummatch(1 2 1 3, 1)` -> `2` |
| `nummember(list, num[, delim])` | Position of numeric match | `nummember(10 20 30, 20)` -> `2` |
| `setsymdiff(l1, l2[, delim])` | Symmetric difference of sets | `setsymdiff(a b c, b c d)` -> `a d` |
| `ledit(list, from, to[, delim])` | Edit each element in list | `ledit(a1 a2, a, b)` -> `b1 b2` |
| `merge(l1, l2, pattern)` | Merge lists using pattern mask | `merge(abc, XYZ, 010)` -> `aYc` |
| `choose(list, weights[, delim])` | Weighted random selection | `choose(a b c, 1 2 3)` |
| `group(list, size[, delim[, osep]])` | Group list into chunks | `group(a b c d, 2)` -> `a b|c d` |
| `wildgrep(obj, pattern, value_pat)` | Grep attributes by value wildcard | `wildgrep(me, *, hello*)` |
| `randextract(list[, count[, delim]])` | Random selection without replacement | `randextract(a b c d, 2)` |
| `elementpos(list, item[, delim])` | All positions of item in list | `elementpos(a b a c, a)` -> `1 3` |

---

## New Encoding / Hashing

| Function | Description | Example |
|----------|-------------|---------|
| `encode64(text)` | Base64 encode | `encode64(hello)` -> `aGVsbG8=` |
| `decode64(text)` | Base64 decode | `decode64(aGVsbG8=)` -> `hello` |
| `digest(algo, text)` | Cryptographic hash (md5, sha1, sha256, sha512) | `digest(sha256, hello)` |
| `crc32(text)` | CRC32 checksum | `crc32(hello)` |
| `hmac(algo, key, text)` | HMAC signature | `hmac(sha256, mykey, data)` |

---

## New Miscellaneous Functions

| Function | Description | Example |
|----------|-------------|---------|
| `helptext(category, topic)` | Retrieve help file text programmatically | `helptext(help, @dig)` |
| `objcall(obj/attr[, args...])` | Call function on object (like u but different arg passing) | `objcall(me/FN, arg1)` |
| `callfn(name[, args...])` | Call a named function dynamically | `callfn(add, 1, 2)` -> `3` |
| `nextdbref()` | Return next available dbref number | `nextdbref()` -> `#14500` |
| `textsearch(category, query)` | Search help files by keyword | `textsearch(help, trigger)` |
| `beep()` | Send BEL character to caller (wizard-only) | `beep()` |
| `singletime(secs)` | Format seconds as human-readable duration | `singletime(3661)` -> `1h` |
| `command([args])` | Information about the current command being processed | `command()` |
| `ccount()` / `cdepth()` | Channel count / depth | `ccount()` |
| `lvars()` | List local variable names | `lvars()` |
| `programmer(player)` | Check if player is in @program mode | `programmer(%#)` |
| `wildparse(str, pattern, result)` | Parse string by wildcard pattern | `wildparse(abc-123, *-*, ##-#+)` |
| `elockstr(obj, locktype, lockstr)` | Evaluate lock string | `elockstr(me, DefaultLock, =me)` |
| `session(player)` | Session info for connected player | `session(%#)` |
| `hasmodule(name)` | Test if server module is loaded | `hasmodule(sql)` |

---

## New Formatting Functions

| Function | Description | Example |
|----------|-------------|---------|
| `align(spec, col1[, col2...])` | Aligned column output | `align(20L 10R, Name, Score)` |
| `tables(list, widths[, opts...])` | Multi-column table with custom widths | `tables(a b c d, 10 10, >, <)` |
| `rtables(list, widths[, opts...])` | Right-aligned tables | `rtables(1 2 3 4, 10 10)` |
| `ctables(list, widths[, opts...])` | Center-aligned tables | `ctables(a b c d, 10 10)` |

---

## New Commands

| Command | Description |
|---------|-------------|
| `@botcreate <name>` | Create a ROBOT player with API key for WebSocket bot control |
| `@apikey <player>` | Display or regenerate API key for a bot player |
| `@instance <args>` | Manage room instances |
| `@dictionary <words>` | Add words to per-object spellcheck dictionary |
| `@archive <player>` | Archive a player's objects |
| `@sql <query>` | Interactive SQL query tool (wizard) |
| `@sqlinit` | Re-initialize SQL connection (God-only) |
| `@sqldisconnect` | Close SQL connection (God-only) |
| `LOGOUT` | Return to login screen without disconnecting socket |
| `+jhelp <topic>` | JSON-formatted help output |
| `@queue <subcommand>` | Event bus management: list, create, delete, stats |

---

## Attribute Definition Functions

| Function | Description | Example |
|----------|-------------|---------|
| `lattrdef([pattern])` | List defined attribute names | `lattrdef()` |
| `attrdefflags(attr)` | Get flags on an attribute definition | `attrdefflags(DESC)` |
| `hasattrdef(attr)` | Test if attribute name is defined | `hasattrdef(MYATTR)` |
| `setattrdef(attr, flags)` | Set flags on an attribute definition | `setattrdef(MYATTR, wizard)` |

---

## Structure / Instance Functions

Full typed data structure system. See [FEATURES.md](FEATURES.md) for details.

| Function | Description | Example |
|----------|-------------|---------|
| `structure(name, components, types, defaults, delim)` | Define a structure type | `structure(player, name level, s i, Bob 1, \|)` |
| `construct(inst, struct[, comps, vals])` | Create an instance | `construct(pc1, player)` |
| `destruct(inst)` | Destroy an instance | `destruct(pc1)` |
| `unstructure(name)` | Remove a structure definition | `unstructure(player)` |
| `z(inst, component)` | Read component value | `z(pc1, name)` -> `Bob` |
| `modify(inst, components, values)` | Modify component values | `modify(pc1, level, 5)` |
| `load(inst, comp1, val1[, ...])` | Load multiple values | `load(pc1, name, Alice, level, 3)` |
| `unload(inst[, delim])` | Dump all values as delimited string | `unload(pc1)` -> `Bob\|1` |
| `read(obj/attr, inst, struct)` | Load instance from attribute | `read(me/DATA, pc1, player)` |
| `write(obj/attr, inst)` | Save instance to attribute | `write(me/DATA, pc1)` |
| `delimit(struct[, new_delim])` | Get/set output delimiter | `delimit(player)` |
| `lstructures()` | List defined structures | `lstructures()` |
| `linstances()` | List active instances | `linstances()` |
| `store(inst, obj/attr)` | Alias for write | `store(pc1, me/DATA)` |
| `items(struct)` | List component names | `items(player)` -> `name level` |

---

## Vector Math Extensions

| Function | Description | Example |
|----------|-------------|---------|
| `vcross(v1, v2)` | Cross product (3D vectors) | `vcross(1 0 0, 0 1 0)` -> `0 0 1` |
| `vdist(v1, v2)` | Distance between two vectors | `vdist(0 0, 3 4)` -> `5` |
| `vlerp(v1, v2, t)` | Linear interpolation | `vlerp(0 0, 10 10, 0.5)` -> `5 5` |
| `vnear(pos, target, range)` | Test if within range | `vnear(0 0, 3 4, 10)` -> `1` |
| `vclamp(v, min, max)` | Clamp vector components | `vclamp(15 -5, 0 0, 10 10)` -> `10 0` |
