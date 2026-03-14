# GoTinyMUSH Functions Reference

This document is a comprehensive reference for all 556+ softcode functions available in GoTinyMUSH. Functions are organized by category matching the source files in `pkg/eval/functions/`.

**Compatibility key:**

- **C** = Standard TinyMUSH 3.x function, tested compatible
- **D** = Exists in TinyMUSH but with behavioral differences
- **N** = New in GoTinyMUSH (not in C TinyMUSH)

---

## Table of Contents

1. [Math](#1-math)
2. [Strings](#2-strings)
3. [Lists](#3-lists)
4. [Objects](#4-objects)
5. [Conditionals](#5-conditionals)
6. [Iteration](#6-iteration)
7. [Connection](#7-connection)
8. [Miscellaneous](#8-miscellaneous)
9. [Navigation](#9-navigation)
10. [JSON](#10-json)
11. [Comsys](#11-comsys)
12. [Mail](#12-mail)
13. [Event Bus](#13-event-bus)
14. [Database](#14-database)
15. [Arrays](#15-arrays)
16. [Structures](#16-structures)
17. [Attribute Definitions](#17-attribute-definitions)

---

## 1. Math

Source: `math.go`

### Arithmetic

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| ADD | `add(num1[, num2, ...])` | Sum of all arguments (float) | C |
| SUB | `sub(num1, num2)` | Subtraction (float) | C |
| MUL | `mul(num1[, num2, ...])` | Product of all arguments (float) | C |
| DIV | `div(num1, num2)` | Integer division | C |
| FADD | `fadd(num1[, num2, ...])` | Float addition | C |
| FSUB | `fsub(num1, num2)` | Float subtraction | C |
| FMUL | `fmul(num1[, num2, ...])` | Float multiplication | C |
| FDIV | `fdiv(num1, num2)` | Float division | C |
| MOD | `mod(num1, num2)` | Remainder (sign follows dividend, same as C `%`) | C |
| MODULO | `modulo(num1, num2)` | True mathematical modulo (always non-negative for positive divisor) | C |
| REMAINDER | `remainder(num1, num2)` | Alias for mod() | C |
| ABS | `abs(num)` | Absolute value | C |
| SIGN | `sign(num)` | Returns -1, 0, or 1 | C |
| INC | `inc(num)` | Increment by 1 (integer) | C |
| DEC | `dec(num)` | Decrement by 1 (integer) | C |
| ROUND | `round(num, places)` | Round to N decimal places (banker's rounding) | C |
| TRUNC | `trunc(num)` | Truncate to integer | C |
| FLOOR | `floor(num)` | Floor (round down) | C |
| CEIL | `ceil(num)` | Ceiling (round up) | C |
| SQRT | `sqrt(num)` | Square root | C |
| POWER | `power(base, exp)` | Exponentiation | C |
| MAX | `max(num1[, num2, ...])` | Maximum value | C |
| MIN | `min(num1[, num2, ...])` | Minimum value | C |
| PI | `pi()` | Returns 3.141592654 | C |
| E | `e()` | Returns 2.718281828 | C |
| FLOORDIV | `floordiv(num1, num2)` | Floor division | C |
| DIST2D | `dist2d(x1, y1, x2, y2)` | 2D Euclidean distance (rounded) | C |
| DIST3D | `dist3d(x1, y1, z1, x2, y2, z2)` | 3D Euclidean distance (rounded) | C |
| ATAN2 | `atan2(y, x)` | Two-argument arctangent (radians) | C |
| BOUND | `bound(value, min, max)` | Clamp scalar to range | C |
| AVG | `avg(num1[, num2, ...])` | Average of arguments | N |
| MEDIAN | `median(num1[, num2, ...])` | Median of arguments | N |
| ALPHAMAX | `alphamax(str1[, str2, ...])` | Alphabetically greatest string | C |
| ALPHAMIN | `alphamin(str1[, str2, ...])` | Alphabetically least string | C |

### Trigonometric

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| SIN | `sin(rad)` | Sine (radians) | C |
| SIND | `sind(deg)` | Sine (degrees) | C |
| COS | `cos(rad)` | Cosine (radians) | C |
| COSD | `cosd(deg)` | Cosine (degrees) | C |
| TAN | `tan(rad)` | Tangent (radians) | C |
| TAND | `tand(deg)` | Tangent (degrees) | C |
| ASIN | `asin(num)` | Arcsine (radians) | C |
| ASIND | `asind(num)` | Arcsine (degrees) | C |
| ACOS | `acos(num)` | Arccosine (radians) | C |
| ACOSD | `acosd(num)` | Arccosine (degrees) | C |
| ATAN | `atan(num)` | Arctangent (radians) | C |
| ATAND | `atand(num)` | Arctangent (degrees) | C |
| COSH | `cosh(num)` | Hyperbolic cosine | N |
| SINH | `sinh(num)` | Hyperbolic sine | N |
| TANH | `tanh(num)` | Hyperbolic tangent | N |

### Exponential / Logarithmic

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| EXP | `exp(num)` | e^num | C |
| LN | `ln(num)` | Natural logarithm | C |
| LOG | `log(num[, base])` | Logarithm (default base 10) | C |

### Bitwise

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| SHL | `shl(num, count)` | Shift left | C |
| SHR | `shr(num, count)` | Shift right | C |
| BAND | `band(num1, num2)` | Bitwise AND | C |
| BOR | `bor(num1, num2)` | Bitwise OR | C |
| BNAND | `bnand(num1, num2)` | Bitwise AND-NOT | C |

### Comparison

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| GT | `gt(num1, num2)` | Greater than (numeric) | C |
| GTE | `gte(num1, num2)` | Greater than or equal | C |
| LT | `lt(num1, num2)` | Less than | C |
| LTE | `lte(num1, num2)` | Less than or equal | C |
| EQ | `eq(num1, num2)` | Numeric equality | C |
| NEQ | `neq(num1, num2)` | Numeric inequality | C |
| COMP | `comp(str1, str2)` | String comparison (case-insensitive) | C |
| STREQ | `streq(str1, str2)` | Exact string equality (case-insensitive) | C |
| NCOMP | `ncomp(num1, num2)` | Numeric comparison (-1/0/1) | C |
| BETWEEN | `between(low, high, val)` | Returns 1 if low <= val <= high | N |

### Logic

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| AND | `and(val1[, val2, ...])` | Logical AND (numeric: 0=false) | C |
| OR | `or(val1[, val2, ...])` | Logical OR (numeric) | C |
| XOR | `xor(val1[, val2, ...])` | Logical XOR (odd count of true) | C |
| NOT | `not(val)` | Logical NOT (numeric) | C |
| NOTBOOL | `notbool(val)` | Boolean NOT (empty/0/#-=false) | C |
| T | `t(val)` | Boolean truth test (xlate semantics) | C |
| ANDBOOL | `andbool(val1[, val2, ...])` | Boolean AND (empty/0=false) | C |
| ORBOOL | `orbool(val1[, val2, ...])` | Boolean OR | C |
| XORBOOL | `xorbool(val1[, val2, ...])` | Boolean XOR | C |
| CAND | `cand(expr1[, expr2, ...])` | Short-circuit AND (no-eval) | C |
| CANDBOOL | `candbool(expr1[, expr2, ...])` | Short-circuit boolean AND | C |
| COR | `cor(expr1[, expr2, ...])` | Short-circuit OR (no-eval) | C |
| CORBOOL | `corbool(expr1[, expr2, ...])` | Short-circuit boolean OR | C |
| NAND | `nand(val1, val2[, ...])` | NAND gate | N |
| NOR | `nor(val1, val2[, ...])` | NOR gate | N |
| XNOR | `xnor(val1, val2[, ...])` | XNOR gate (boolean equality) | N |

### Vector Math

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| VADD | `vadd(v1, v2)` | Vector addition | C |
| VSUB | `vsub(v1, v2)` | Vector subtraction | C |
| VMUL | `vmul(v, scalar)` | Scalar multiplication | C |
| VDOT | `vdot(v1, v2)` | Dot product | C |
| VMAG | `vmag(v)` | Vector magnitude | C |
| VUNIT | `vunit(v)` | Unit vector | C |
| VDIM | `vdim(v)` | Number of dimensions | C |
| VCROSS | `vcross(v1, v2)` | 3D cross product | C |
| VDIST | `vdist(v1, v2)` | N-dimensional distance | N |
| VLERP | `vlerp(v1, v2, t)` | Linear interpolation (t=0 gives v1, t=1 gives v2) | N |
| VNEAR | `vnear(v1, v2, radius)` | Returns 1 if v2 is within radius of v1 | N |
| VCLAMP | `vclamp(v, min, max)` | Per-component clamp | N |

### Base Conversion

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| TOBIN | `tobin(num)` | Convert integer to binary string | N |
| TODEC | `todec(str)` | Convert from any base (0x, 0o, 0b prefix) to decimal | N |
| TOHEX | `tohex(num)` | Convert integer to hexadecimal (uppercase) | N |
| TOOCT | `tooct(num)` | Convert integer to octal | N |
| ROMAN | `roman(num)` | Convert integer (1-3999999) to Roman numerals | N |
| FMOD | `fmod(x, y)` | Floating-point modulo | N |

### Examples

```
> think add(1,2,3,4,5)
15
> think round(3.14159, 2)
3.14
> think vadd(1 2 3, 4 5 6)
5 7 9
> think bound(150, 0, 100)
100
> think between(1, 10, 5)
1
> think tohex(255)
FF
```

---

## 2. Strings

Source: `strings.go`

### Core String Operations

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| CAT | `cat(str1[, str2, ...])` | Concatenate with spaces | C |
| STRCAT | `strcat(str1[, str2, ...])` | Concatenate without spaces | C |
| STRLEN | `strlen(str)` | String length (ANSI-aware) | C |
| MID | `mid(str, start, len)` | Substring extraction | C |
| LEFT | `left(str, n)` | Left N characters | C |
| STRTRUNC | `strtrunc(str, n)` | Alias for left() | C |
| RIGHT | `right(str, n)` | Right N characters | C |
| LCSTR | `lcstr(str)` | Lowercase | C |
| UCSTR | `ucstr(str)` | Uppercase | C |
| CAPSTR | `capstr(str)` | Capitalize first character | C |
| POS | `pos(search, str)` | Find position of substring (1-based) | C |
| LPOS | `lpos(str, char[, start])` | List positions of character | C |
| EDIT | `edit(str, from, to)` | String replacement | C |
| REPLACE | `replace(str, pos, len, new)` | Positional replacement | C |
| TRIM | `trim(str[, side[, chars]])` | Trim whitespace/characters | C |
| SQUISH | `squish(str[, char])` | Compress repeated characters | C |
| LJUST | `ljust(str, width[, fill])` | Left-justify (pad right) | C |
| RJUST | `rjust(str, width[, fill])` | Right-justify (pad left) | C |
| CENTER | `center(str, width[, fill])` | Center text | C |
| REPEAT | `repeat(str, count)` | Repeat string N times | C |
| SPACE | `space(n)` | Generate N spaces | C |
| ESCAPE | `escape(str)` | Escape MUSH special characters | C |
| SECURE | `secure(str)` | Secure string from evaluation | C |
| NESCAPE | `nescape(str)` | Newline-preserving escape | C |
| NSECURE | `nsecure(str)` | Newline-preserving secure | C |
| ANSI | `ansi(codes, str)` | Apply ANSI color codes | C |
| STRIPANSI | `stripansi(str)` | Remove ANSI codes | C |
| BEFORE | `before(str, delim)` | Text before first delimiter | C |
| AFTER | `after(str, delim)` | Text after first delimiter | C |
| REVERSE | `reverse(str)` | Reverse string | C |
| SCRAMBLE | `scramble(str)` | Randomly shuffle characters | C |
| STRMATCH | `strmatch(str, pattern)` | Wildcard match (returns 0/1) | C |
| MATCH | `match(list, pattern[, delim])` | Wildcard match in list (returns position) | C |
| MATCHALL | `matchall(list, pattern[, delim])` | All matching positions | C |
| DELETE | `delete(str, pos, len)` | Delete substring | C |
| CHOMP | `chomp(str)` | Remove trailing newline | C |
| TRANSLATE | `translate(str, type)` | Translate CR/LF | C |
| WORDPOS | `wordpos(str, word[, delim])` | Position of word | C |
| INDEX | `index(str, delim, pos, count)` | Extract delimited fields | C |
| ENCRYPT | `encrypt(str, key)` | Simple XOR encryption | C |
| DECRYPT | `decrypt(str, key)` | Simple XOR decryption | C |
| WILDMATCH | `wildmatch(pattern, str, template)` | Wildcard match with template output | C |
| ART | `art(str)` | Article: "a" or "an" | C |
| SPEAK | `speak(speaker, str[, say[, pose[, possessive]]])` | Format speech | C |
| ANSIPOS | `ansipos(str, pos)` | Position accounting for ANSI | C |

### Type/Character Class Checking

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| ISNUM | `isnum(str)` | Is numeric? | C |
| ISDBREF | `isdbref(str)` | Is valid dbref? | C |
| ISWORD | `isword(str)` | Is alphabetic word? | C |
| ISALNUM | `isalnum(str)` | All alphanumeric? | C |
| ISALPHA | `isalpha(str)` | All alphabetic? | C |
| ISDIGIT | `isdigit(str)` | All digits? | C |
| ISUPPER | `isupper(str)` | All uppercase? | C |
| ISLOWER | `islower(str)` | All lowercase? | C |
| ISSPACE | `isspace(str)` | All whitespace? | C |
| ISPUNCT | `ispunct(str)` | All punctuation? | C |

### Border/Formatting

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| BORDER | `border(text[, width[, fill]])` | Left-aligned bordered text | C |
| CBORDER | `cborder(text[, width[, fill]])` | Center-aligned bordered text | C |
| RBORDER | `rborder(text[, width[, fill]])` | Right-aligned bordered text | C |

### Encoding/Hashing

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| ENCODE64 | `encode64(str)` | Base64 encode | N |
| DECODE64 | `decode64(str)` | Base64 decode | N |
| DIGEST | `digest(algorithm[, str])` | Cryptographic hash (md5, sha1, sha256, sha512) | N |
| CRC32 | `crc32(str)` | CRC32 checksum | N |
| HMAC | `hmac(algorithm, key, data)` | HMAC authentication code | N |
| HTML_ESCAPE | `html_escape(str)` | Escape HTML entities | N |
| HTML_UNESCAPE | `html_unescape(str)` | Unescape HTML entities | N |
| URL_ESCAPE | `url_escape(str)` | URL-encode string | N |
| URL_UNESCAPE | `url_unescape(str)` | URL-decode string | N |

### Extended String Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| PRINTF | `printf(format, args...)` | Formatted string output (RhostMUSH-style) | N |
| TR | `tr(str, from, to)` | Character transliteration | N |
| STRDISTANCE | `strdistance(str1, str2)` | Levenshtein edit distance | N |
| STRLENVIS | `strlenvis(str)` | Visible length (ignoring ANSI) | N |
| ASC | `asc(char)` | Character to ASCII code | N |
| CHR | `chr(num)` | ASCII code to character | N |
| STRIP | `strip(str[, chars])` | Strip specific characters | N |
| CAPLIST | `caplist(str[, delim])` | Capitalize each word | N |
| SPELLNUM | `spellnum(num)` | Number to English words | N |
| SOUNDEX | `soundex(str)` | Soundex phonetic code | N |
| SOUNDLIKE | `soundlike(str1, str2)` | Soundex comparison | N |
| GARBLE | `garble(str[, pct[, type]])` | Randomly garble text | N |
| SPELL | `spell(str[, dict])` | Spell check word | N |
| SPELLCHECK | `spellcheck(str[, dict])` | Spell check with suggestions | N |

### Examples

```
> think ljust(Hello, 20, .)
Hello...............
> think center(Title, 40, =)
=================Title==================
> think edit(Hello World, World, MUSH)
Hello MUSH
> think strmatch(Hello World, *World)
1
> think encode64(Hello)
SGVsbG8=
> think digest(sha256, test)
9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
```

---

## 3. Lists

Source: `lists.go`

### Core List Operations

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| WORDS | `words(list[, delim])` | Count words in list | C |
| FIRST | `first(list[, delim])` | First element | C |
| REST | `rest(list[, delim])` | All but first element | C |
| LAST | `last(list[, delim])` | Last element | C |
| EXTRACT | `extract(list, pos, count[, delim])` | Extract elements (1-based) | C |
| ELEMENTS | `elements(list, positions[, delim[, odelim]])` | Extract by position list | C |
| LNUM | `lnum(count)` or `lnum(start, end[, delim])` | Generate number list | C |
| MEMBER | `member(list, value[, delim])` | Find position of exact match (1-based, 0=not found) | C |
| REMOVE | `remove(list, value[, delim])` | Remove first occurrence | C |
| INSERT | `insert(list, pos, value[, delim])` | Insert at position | C |
| LDELETE | `ldelete(list, positions[, delim])` | Delete by position(s) | C |
| SORT | `sort(list[, type[, delim[, odelim]]])` | Sort list (auto-detect type: a/n/d/i/f) | C |
| REVWORDS | `revwords(list[, delim])` | Reverse element order | C |
| SHUFFLE | `shuffle(list[, delim])` | Randomly reorder | C |
| ITEMIZE | `itemize(list[, delim[, conj[, punc]]])` | English list (a, b, and c) | C |
| SPLICE | `splice(list1, list2, word[, delim])` | Replace matching words | C |
| GRAB | `grab(list, pattern[, delim])` | First wildcard match | C |
| GRABALL | `graball(list, pattern[, delim[, odelim]])` | All wildcard matches | C |
| SORTBY | `sortby(obj/attr, list[, delim])` | Custom sort function | C |
| LREPLACE | `lreplace(list, replacements, positions[, delim[, odelim]])` | Replace at positions | C |
| LEDIT | `ledit(list, from, to[, delim])` | Apply edit() to each element | C |
| ISORT | `isort(list[, type[, delim]])` | Returns sorted position indices | C |
| MERGE | `merge(str1, str2, char)` | Character-level merge | C |
| CHOOSE | `choose(list, weights[, delim])` | Weighted random selection | N |
| GROUP | `group(list, n[, delim[, odelim[, gdelim]]])` | Group elements column-wise | C |
| WILDGREP | `wildgrep(obj, attr-pat, search-pat)` | Grep attributes by wildcard | C |
| RANDEXTRACT | `randextract(list[, count[, delim]])` | Random element(s) | C |
| ELEMENTPOS | `elementpos(list, value[, delim])` | All positions of value | C |

### Set Operations

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| SETUNION / LUNION | `setunion(list1, list2[, delim[, odelim[, sort]]])` | Union of two lists | C |
| SETDIFF / LDIFF | `setdiff(list1, list2[, delim[, odelim[, sort]]])` | Difference (A - B) | C |
| SETINTER / LINTER | `setinter(list1, list2[, delim[, odelim[, sort]]])` | Intersection | C |
| SETSYMDIFF / LSYMDIFF | `setsymdiff(list1, list2[, delim[, odelim[, sort]]])` | Symmetric difference | C |

### List Math

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| LADD | `ladd(list[, delim])` | Sum of list elements | C |
| LMAX | `lmax(list[, delim])` | Maximum of list | C |
| LMIN | `lmin(list[, delim])` | Minimum of list | C |
| LAND | `land(list[, delim])` | Logical AND across list (numeric) | C |
| LOR | `lor(list[, delim])` | Logical OR across list | C |
| LANDBOOL | `landbool(list[, delim])` | Boolean AND across list | C |
| LORBOOL | `lorbool(list[, delim])` | Boolean OR across list | C |
| LAVG | `lavg(list[, delim])` | Average of list | N |
| LSUB | `lsub(list[, delim])` | First minus rest | N |
| LMUL | `lmul(list[, delim])` | Product of list | N |
| LDIV | `ldiv(list[, delim])` | First divided by rest | N |
| LISTMATCH | `listmatch(list, pattern[, delim])` | Filter by wildcard | N |
| NUMMATCH | `nummatch(list, pattern[, delim])` | Count wildcard matches | N |
| NUMMEMBER | `nummember(list, value[, delim])` | Count exact occurrences (case-insensitive) | N |

### Examples

```
> think sort(banana apple cherry)
apple banana cherry
> think setunion(a b c, b c d)
a b c d
> think itemize(apples oranges bananas)
apples, oranges, and bananas
> think ladd(1 2 3 4 5)
15
> think elements(red green blue yellow, 2 4)
green yellow
```

---

## 4. Objects

Source: `objects.go`

### Object Queries

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| NAME | `name(obj[, newname])` | Get/set object name | C |
| FULLNAME | `fullname(obj)` | Full name with aliases | C |
| NUM | `num(obj)` | Resolve to dbref | C |
| LOC | `loc(obj)` | Get location | C |
| OWNER | `owner(obj)` | Get owner dbref | C |
| TYPE | `type(obj)` | Object type string (ROOM/EXIT/THING/PLAYER) | C |
| OBJTYPE | `objtype(obj)` | Alias for type() | C |
| HASTYPE | `hastype(obj, type)` | Check type (returns 0/1) | C |
| FLAGS | `flags(obj)` | Get flag string | C |
| HASFLAG | `hasflag(obj, flag)` | Test for flag | C |
| HASFLAGS | `hasflags(obj, flaglist)` | Test for multiple flags | N |
| ANDFLAGS | `andflags(obj, flaglist)` | All flags present? | C |
| ORFLAGS | `orflags(obj, flaglist)` | Any flag present? | C |
| HASPOWER | `haspower(obj, power)` | Test for power | C |
| CON | `con(obj)` | First contents | C |
| EXIT | `exit(obj)` | First exit | C |
| NEXT | `next(obj)` | Next in chain | C |
| LCON | `lcon(obj[, type])` | List contents | C |
| LEXITS | `lexits(obj[, type])` | List exits | C |
| EXITS | `exits(obj[, type])` | Alias for lexits() | C |
| HOME | `home(obj)` | Get home/dropto | C |
| PARENT | `parent(obj[, depth])` | Get parent | C |
| ZONE | `zone(obj)` | Get zone | C |
| ZONES | `zones(obj)` | Get all zones | N |
| ROOM | `room(obj)` | Find containing room | C |
| CONTROLS | `controls(player, obj)` | Permission test | C |
| CHILDREN | `children(obj)` | List children (objects parented to obj) | C |
| LPARENT | `lparent(obj)` | Parent chain | C |
| ENTRANCES | `entrances(obj[, type[, low[, high]]])` | Objects linking to this object | C |
| LOCATE | `locate(looker, name, type)` | Advanced object matching | C |
| RLOC | `rloc(obj[, depth])` | Walk up to room | C |
| NEARBY | `nearby(obj1, obj2)` | Same location test | C |
| WHERE | `where(player)` | Location (bypasses unfindable) | C |
| XCON | `xcon(obj)` | Exhaustive contents | C |
| INZONE | `inzone(zone)` | Objects in zone | C |
| ZWHO | `zwho(zone)` | Players in zone | C |
| ZFUN | `zfun(zone, attr[, args...])` | Call zone function | C |
| FINDABLE | `findable(player, target)` | Can player find target? | C |
| SEES | `sees(player, target)` | Can player see target? | C |
| VISIBLE | `visible(player, target)` | Visibility check | C |
| HEARS | `hears(listener, speaker)` | Can listener hear speaker? | C |
| KNOWS | `knows(player, target)` | Does player know target? | C |
| MOVES | `moves(obj, dest)` | Can object move there? | C |
| ELOCK | `elock(obj, player)` | Evaluate lock | C |
| LOCK | `lock(obj)` | Get lock string | C |
| SCAN_ZONE | `scan_zone(zone[, type])` | Scan objects in zone | N |
| ISINSTANCE | `isinstance(obj)` | Is object an instance? | N |
| IROOMS | `irooms(obj)` | Instance rooms | N |
| IVEHICLE | `ivehicle(obj)` | Instance vehicle | N |

### Attribute Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| HASATTR | `hasattr(obj, attr)` | Test attribute exists | C |
| HASATTRP | `hasattrp(obj, attr)` | Test attr (checks parents) | C |
| GET | `get(obj/attr)` | Get attribute value | C |
| XGET | `xget(obj, attr)` | Get attribute (separate args) | C |
| V | `v(attr)` | Get own attribute value | C |
| U | `u(obj/attr[, arg1, ...])` | Evaluate user function | C |
| ULOCAL | `ulocal(obj/attr[, arg1, ...])` | U with localized registers | C |
| S | `s(str)` | Evaluate string substitutions | C |
| OBJEVAL | `objeval(obj, expr)` | Evaluate as another object | C |
| DEFAULT | `default(obj/attr, fallback)` | Get attr with fallback | C |
| EDEFAULT | `edefault(obj/attr, fallback)` | Get+eval attr with fallback | C |
| GET_EVAL | `get_eval(obj/attr)` | Get and evaluate attribute | C |
| LATTR | `lattr(obj[/pattern])` | List attributes | C |
| NATTR | `nattr(obj)` | Count attributes | C |
| GREP | `grep(obj, attr-pattern, text)` | Grep attributes (substring) | C |
| GREPI | `grepi(obj, attr-pattern, text)` | Case-insensitive grep | C |
| PGREP | `pgrep(obj, attr-pattern, text)` | Grep with parent search | C |
| PGREPI | `pgrepi(obj, attr-pattern, text)` | Case-insensitive parent grep | C |
| MONEY / PENNIES | `money(obj)` | Get object's money | C |
| EVAL | `eval(obj, attr)` | Evaluate attribute | C |
| ATTRCNT | `attrcnt(obj[/pattern])` | Count matching attributes | N |
| LCMDS | `lcmds(obj[, type])` | List $ commands on object | N |
| LROOMS | `lrooms(obj[, depth[, type]])` | List rooms reachable via exits | N |

### Pronoun Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| SUBJ | `subj(obj)` | Subjective pronoun (he/she/they/it) | C |
| OBJ | `obj(obj)` | Objective pronoun (him/her/them/it) | C |
| POSS | `poss(obj)` | Possessive pronoun (his/her/their/its) | C |
| APOSS | `aposs(obj)` | Absolute possessive (his/hers/theirs/its) | C |

### Timestamp Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| LASTACCESS | `lastaccess(obj)` | Last access time (epoch secs) | C |
| LASTMOD | `lastmod(obj)` | Last modification time | C |
| LASTCREATE | `lastcreate(obj[, type])` | Last created object of type | C |
| OBJMEM | `objmem(obj)` | Rough memory usage | C |
| PLAYMEM | `playmem(player)` | Player memory usage | N |
| OBJID | `objid(obj)` | Object ID (dbref:creation) | N |
| CREATETIME | `createtime(obj)` | Creation timestamp | N |
| ISOBJID | `isobjid(str)` | Test if valid objid format | N |
| SINGLETIME | `singletime(secs)` | Seconds to human-readable duration | N |

### Examples

```
> think name(me)
Moravel
> think type(me)
PLAYER
> think lcon(here)
#1234 #1235 #1236
> think u(me/FN_GREET, World)
Hello, World!
> think hasflag(me, WIZARD)
1
```

---

## 5. Conditionals

Source: `conditionals.go`

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| IF | `if(cond, true[, false])` | Boolean conditional (no-eval) | C |
| IFELSE | `ifelse(cond, true, false)` | Alias for if() | C |
| NONZERO | `nonzero(cond, true[, false])` | Alias for if() | C |
| SWITCH | `switch(expr, pat1, res1, ..., default)` | Wildcard pattern matching | C |
| SWITCHALL | `switchall(expr, pat1, res1, ..., default)` | Match ALL patterns | C |
| CASE | `case(expr, val1, res1, ..., default)` | Exact match (case-insensitive) | C |
| RESWITCH | `reswitch(expr, regex1, res1, ..., default)` | Regex pattern matching | C |
| RESWITCHALL | `reswitchall(expr, regex1, res1, ..., default)` | Regex match ALL | C |
| RESWITCHI | `reswitchi(expr, regex1, res1, ..., default)` | Case-insensitive regex match | C |
| RESWITCHALLI | `reswitchalli(expr, regex1, res1, ..., default)` | Case-insensitive regex match ALL | C |
| IFFALSE | `iffalse(cond, false[, true])` | Inverted if() | N |
| IFTRUE | `iftrue(cond, true[, false])` | Alias for if() | N |
| IFZERO | `ifzero(cond, zero-result[, nonzero])` | True when arg is exactly "0" | N |
| USETRUE | `usetrue(val, fallback)` | Return val if true, else fallback | N |
| USEFALSE | `usefalse(val, fallback)` | Return val if false, else fallback | N |
| ISFALSE | `isfalse(val)` | Returns 1 if val is false | N |
| ISTRUE | `istrue(val)` | Returns 1 if val is true | N |
| UDEFAULT | `udefault(obj/attr, fallback[, args...])` | u() with fallback if attr empty | N |

In switch/reswitch, `#$` refers to the matched expression.

### Examples

```
> think if(1, Yes, No)
Yes
> think switch(apple, a*, Fruit, b*, Veggie, Unknown)
Fruit
> think case(red, red, Stop, green, Go, Caution)
Stop
> think usetrue(Hello, default)
Hello
> think ifzero(0, was zero, was not zero)
was zero
```

---

## 6. Iteration

Source: `iter.go`

### Core Iteration

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| ITER | `iter(list, pattern[, idelim[, odelim]])` | Iterate list, `##`=element, `#@`=position | C |
| PARSE | `parse(list, pattern[, idelim[, odelim]])` | Alias for iter() | C |
| MAP | `map(obj/attr, list[, delim[, odelim]])` | Apply function to each element | C |
| FILTER | `filter(obj/attr, list[, delim[, odelim]])` | Keep elements where function returns true | C |
| FOLD | `fold(obj/attr, list[, base[, delim]])` | Reduce list through function | C |
| FOREACH | `foreach(obj/attr, string)` | Apply function to each character | C |
| STEP | `step(obj/attr, list, step[, delim[, odelim]])` | Process list in N-element chunks | C |
| MIX | `mix(obj/attr, list1, list2[, ...])` | Parallel iteration of multiple lists | C |
| MUNGE | `munge(obj/attr, list1, list2[, delim[, odelim]])` | Custom sort with parallel list | C |
| WHILE | `while(eval_fn, cond_fn, list, stop[, isep[, osep]])` | Iterate until condition matches stop value | C |
| SORTBY | `sortby(obj/attr, list[, delim])` | Sort by custom comparison function | C |

### Loop State Query

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| ILEV | `ilev()` | Current nesting level (0=outermost) | C |
| ITEXT | `itext(n)` | Loop token at nesting level N | C |
| INUM | `inum(n)` | Loop number at nesting level N | C |
| IBREAK | `ibreak([level])` | Break out of loop | C |

### Extended Iteration (New)

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| ITER2 | `iter2(list1, list2, pattern[, idelim[, odelim]])` | Dual-list iteration (`##`=list1, `#+`=list2) | N |
| ITEXT2 | `itext2(n)` | Second list token at nesting level N | N |
| WHENTRUE | `whentrue(list, pattern[, idelim[, odelim]])` | Iterate until pattern returns false | N |
| WHENFALSE | `whenfalse(list, pattern[, idelim[, odelim]])` | Iterate until pattern returns true | N |
| WHENTRUE2 | `whentrue2(list1, list2, cond[, idelim[, odelim]])` | Dual-list whentrue | N |
| WHENFALSE2 | `whenfalse2(list1, list2, cond[, idelim[, odelim]])` | Dual-list whenfalse | N |
| FILTERBOOL | `filterbool(obj/attr, list[, delim[, odelim]])` | Filter with boolean semantics | N |
| UNTIL | `until(condfn, bodyfn, initial[, delim])` | Loop until condition is true | N |
| LOOP | `loop(list, pattern[, idelim[, odelim]])` | Like iter() but sends output as notifications | N |
| LIST | `list(list, pattern[, idelim[, odelim]])` | Alias for loop() | N |
| LIST2 | `list2(list1, list2, pattern[, idelim])` | Dual-list loop (notifications) | N |

### Lambda Support

All function-reference arguments (map, filter, fold, etc.) support `#lambda/EXPRESSION` syntax for inline functions without needing a stored attribute.

### Examples

```
> think iter(red green blue, [ucstr(##)])
RED GREEN BLUE
> think filter(me/IS_EVEN, 1 2 3 4 5 6)
2 4 6
> think fold(me/FN_ADD, 1 2 3 4 5)
15
> think iter2(a b c, 1 2 3, ##=#+ )
a=1 b=2 c=3
> think map(#lambda/[ucstr(%0)], hello world)
HELLO WORLD
```

---

## 7. Connection

Source: `connection.go`

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| LWHO | `lwho([all])` | List connected player dbrefs | C |
| MWHO | `mwho()` | List visible connected players | C |
| CONN | `conn(player)` | Connection time in seconds | C |
| IDLE | `idle(player)` | Idle time in seconds | C |
| DOING | `doing(player)` | Player's @doing string | C |
| PMATCH | `pmatch(name)` | Match player name to dbref | C |
| CONNLOG | `connlog(player[, count])` | Connection log timestamps (wizard/self) | N |
| ADDRLOG | `addrlog(player[, count])` | Connection log IP addresses (wizard/self) | N |

### Formatting

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| WRAP | `wrap(text, width[, left[, hang]])` | Word-wrap text (ANSI-aware) | C |
| COLUMNS | `columns(list, colwidth[, delim[, linewidth]])` | Format into columns | C |
| TABLE | `table(list, fieldwidth[, linewidth[, listsep[, fieldsep[, pad]]]])` | Fixed-width table | C |
| TABLES | `tables(list, widths[, lead[, trail[, listsep[, fieldsep[, pad]]]]])` | Variable-width table (left-justified) | C |
| RTABLES | `rtables(...)` | Variable-width table (right-justified) | C |
| CTABLES | `ctables(...)` | Variable-width table (center-justified) | C |
| ALIGN | `align(template, col1[, col2, ...])` | Columnar alignment | C |

### Examples

```
> think lwho()
#1 #42 #100
> think conn(me)
3600
> think columns(alpha beta gamma delta epsilon, 15)
alpha          beta           gamma
delta          epsilon
```

---

## 8. Miscellaneous

Source: `misc.go`

### Registers

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| SETQ | `setq(reg, val[, reg, val, ...])` | Set Q-register (0-9, a-z, or named) | C |
| SETR | `setr(reg, val)` | Set and return value | C |
| R | `r(reg)` | Read Q-register | C |
| X | `x(name)` | Read named X-register | C |
| SETX | `setx(name, val)` | Set named X-register | C |
| LREGS | `lregs()` | List set registers | C |
| QVARS | `qvars(names[, delim])` | Read multiple Q vars | C |
| XVARS | `xvars(names[, delim])` | Read multiple X vars | C |
| CLEARVARS | `clearvars()` | Clear all registers | C |
| LET | `let(assignments, expr)` | Scoped register assignments | C |
| LOCALIZE | `localize(expr)` | Evaluate with localized registers | C |
| PRIVATE | `private(expr)` | Private register scope | C |
| UPRIVATE | `uprivate(obj/attr[, args...])` | U() with private registers | C |
| LVARS | `lvars()` | List variable names | C |

### Stack Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| PUSH | `push(val)` | Push onto stack | C |
| POP | `pop()` | Pop from stack | C |
| PEEK | `peek([depth])` | Read stack top | C |
| EMPTY | `empty()` | Clear stack | C |
| LSTACK | `lstack([delim])` | List stack contents | C |
| DUP | `dup()` | Duplicate stack top | C |
| SWAP | `swap()` | Swap top two stack elements | C |
| POPN | `popn([count])` | Pop N elements | C |
| TOSS | `toss()` | Discard stack top | C |

### Side-Effect Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| PEMIT | `pemit(player, msg)` | Send message to player | C |
| REMIT | `remit(room, msg)` | Emit to room | C |
| OEMIT | `oemit(obj, msg)` | Emit to room except obj | C |
| THINK | `think(msg)` | Send message to self | C |
| SET | `set(obj/attr, val)` or `set(obj, FLAG)` | Set attribute or flag | C |
| CREATE | `create(name[, type[, dest]])` | Create object (thing/room/exit) | C |
| TEL | `tel(obj, dest)` | Teleport object | C |
| LINK | `link(obj, dest)` | Set home/dropto/destination | C |
| TRIGGER | `trigger(obj/attr[, arg1, ...])` | Trigger attribute | C |
| WIPE | `wipe(obj[/pattern])` | Remove attributes | C |
| FORCE | `force(obj, command)` | Force command execution | C |
| WAIT | `wait(secs, command)` | Delayed command | C |
| CLONE | `clone(obj)` | Clone object | C |
| CEMIT | `cemit(channel, msg)` | Emit to channel | C |
| COMMAND | `command(cmd[, args...])` | Dispatch command | C |

### Time Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| TIME | `time()` | Current time string | C |
| SECS | `secs()` | Current Unix epoch seconds | C |
| CONVSECS | `convsecs(secs)` | Epoch to time string | C |
| CONVTIME | `convtime(timestr)` | Time string to epoch | C |
| TIMEFMT | `timefmt(format[, secs])` | Formatted time | C |
| STARTTIME | `starttime()` | Server start time | C |
| RESTARTTIME | `restarttime()` | Last restart time | C |
| UPTIME | `uptime()` | Seconds since startup | C |
| RESTARTS | `restarts()` | Number of restarts | C |

### Random Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| RAND | `rand(max)` | Random 0 to max-1 | C |
| DIE | `die(count, sides)` | Roll dice | C |
| LRAND | `lrand(min, max, count[, delim])` | List of random numbers | C |

### Regex Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| REGMATCH | `regmatch(str, regex[, regs])` | Regex match (capture to registers) | C |
| REGMATCHI | `regmatchi(str, regex[, regs])` | Case-insensitive regex match | C |
| REGEDIT | `regedit(str, regex, replace)` | Regex replace (first match) | C |
| REGEDITI | `regediti(str, regex, replace)` | Case-insensitive regex replace | C |
| REGEDITALL | `regeditall(str, regex, replace)` | Regex replace (all matches) | C |
| REGEDITALLI | `regeditalli(str, regex, replace)` | Case-insensitive replace all | C |
| REGRAB | `regrab(list, regex[, delim])` | First regex match in list | C |
| REGRABI | `regrabi(list, regex[, delim])` | Case-insensitive regrab | C |
| REGRABALL | `regraball(list, regex[, delim])` | All regex matches in list | C |
| REGRABALLI | `regraballi(list, regex[, delim])` | Case-insensitive regraball | C |
| REGREP | `regrep(obj, attrs, regex)` | Regex grep on attributes | C |
| REGREPI | `regrepi(obj, attrs, regex)` | Case-insensitive regrep | C |
| REGPARSE | `regparse(str, regex, template)` | Regex parse with template | C |
| REGPARSEI | `regparsei(str, regex, template)` | Case-insensitive regparse | C |
| WILDPARSE | `wildparse(str, pattern, template)` | Wildcard parse with template | C |

### Server Info

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| VERSION | `version()` | Server version string | C |
| MUDNAME | `mudname()` | MUD name | C |
| CONFIG | `config(param)` | Get config parameter | C |
| PORTS | `ports(player)` | Port numbers (wizard-only) | C |
| CONNRECORD | `connrecord()` | Peak connection count | C |
| FCOUNT | `fcount()` | Function invocation count | C |
| FDEPTH | `fdepth()` | Function nesting depth | C |
| CCOUNT | `ccount()` | Command count | C |
| CDEPTH | `cdepth()` | Command nesting depth | C |
| STATS | `stats([player])` | Database statistics | C |
| SEARCH / LSEARCH | `search(type=criteria)` | Search database | C |
| HASMODULE | `hasmodule(name)` | Module availability | C |
| VALID | `valid(type, name)` | Validate name for type | C |
| BEEP | `beep()` | Send beep (wizard-only) | C |
| NEXTDBREF | `nextdbref()` | Next available dbref | N |
| SESSION | `session(player)` | Session info | C |
| HELPTEXT | `helptext(topic, file)` | Retrieve help text | N |
| OBJCALL | `objcall(obj/attr[, args...])` | Call object function | N |
| CALLFN | `callfn(name[, args...])` | Call named function | N |
| PROGRAMMER | `programmer(obj)` | Get programmer | C |
| ELOCKSTR | `elockstr(obj, lock, player)` | Evaluate lock string | C |
| PFIND | `pfind(name)` | Find player | C |
| WRITABLE | `writable(player, obj)` | Write permission test | C |

### OOB / GMCP Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| OOB | `oob(player, package, data)` | Send Out-of-Band message | N |
| HASGMCP | `hasgmcp(player)` | Player supports GMCP? | N |
| GMCPPACKAGES | `gmcppackages(player)` | List GMCP packages | N |
| HASMSDP | `hasmsdp(player)` | Player supports MSDP? | N |
| HASAPIKEY / ISAPIKEY | `hasapikey(player)` | Player has API key? | N |

### Text Search / Logging

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| TEXTSEARCH | `textsearch(text, query)` | Full-text search | N |

### Utility

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| NULL / @@ | `null(expr)` | Evaluate and discard | C |
| LIT | `lit(text)` | Return text literally (no-eval) | C |
| SUBEVAL | `subeval(text)` | % substitutions only, no functions | C |

### Examples

```
> think setq(0, Hello)[r(0)]
Hello
> think die(3, 6)
11
> think regmatch(Hello World, (\w+)\s(\w+), 0 1)
1
> think timefmt($Y-$m-$d $H:$M:$S)
2026-03-14 15:30:00
> think oob(#42, beip.tilemap.data, {"tiles":"..."})
```

---

## 9. Navigation

Source: `nav.go`

All navigation functions are **new in GoTinyMUSH** (N). They provide a complete 3D grid-based navigation system with 32-point compass headings, topology/terrain generation, weather physics, and POI management.

### Heading System

Headings use a 32-point compass: 0=E, 4=NE, 8=N, 12=NW, 16=W, 20=SW, 24=S, 28=SE. Each step = 11.25 degrees.

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| HVEC | `hvec(heading)` | Heading to unit direction vector "dx dy" | N |
| HDELTA | `hdelta(h1, h2)` | Shortest turn between headings (-16 to +16) | N |
| HNAME | `hname(heading[, exact])` | Heading to compass name (16 or 32 point) | N |
| H2DEG | `h2deg(heading)` | Heading to degrees | N |
| DEG2H | `deg2h(degrees)` | Degrees to nearest heading | N |
| VEC2H | `vec2h(x, y)` | Direction vector to heading | N |

### Grid Coordinates

Grid system: 4 quadrants (NE/NW/SE/SW), letters AA-ZZ (676 positions), numbers 000-999, altitude -1000 to +1000. Format: `LL-NNN-QQ[:ALT]` (e.g., `EL-453-NE:500`).

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| GRIDABS | `gridabs(letters, number, quadrant)` or `gridabs(LL-NNN-QQ)` | Grid location to absolute "x y" | N |
| ABSGRID | `absgrid(x, y)` | Absolute coords to "LL-NNN-QQ" | N |
| GRIDDIST | `griddist(loc1, loc2)` | 2D distance between grid locations | N |
| GRIDDIST3D | `griddist3d(loc1, loc2)` | 3D distance between grid locations | N |
| GRIDCOURSE | `gridcourse(from, to)` | Heading and distance: "heading distance" | N |
| GRIDNAV | `gridnav(pos, heading, speed[, climb[, drift]])` | Project new position: "x y z" | N |
| GRIDLOCFULL | `gridlocfull(x, y, z)` | Absolute to "LL-NNN-QQ:ALT" | N |
| GRIDPARSEFULL | `gridparsefull(loc)` | Parse "LL-NNN-QQ:ALT" to "x y z" | N |
| GPS | `gps([obj])` | Get GPS position | N |

### Random Vector / Drift

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| VRAND | `vrand(max_mag[, dims])` | Random direction vector (uniform on sphere) | N |
| VRANDC | `vrandc(max_x max_y max_z)` | Per-component random (rectangular) | N |
| DRIFT | `drift(position, max_drift)` | Apply random drift to position | N |

### Tactical Navigation

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| BEARING | `bearing(pos1, pos2)` | Heading from pos1 to pos2 (2D) | N |
| PITCH | `pitch(pos1, pos2)` | Vertical angle in degrees | N |
| CLOSING | `closing(pos1, hdg1, spd1, pos2, hdg2, spd2)` | Closing rate between two objects | N |
| RELVEL | `relvel(hdg1, spd1, hdg2, spd2)` | Relative velocity vector | N |
| ETA | `eta(pos1, pos2, heading, speed)` | Estimated time of arrival | N |
| INTERCEPT | `intercept(my_pos, tgt_pos, tgt_hdg, tgt_spd, my_spd)` | Intercept heading calculation | N |
| ALTCLAMP | `altclamp(alt)` | Clamp altitude to valid range | N |

### Topology / Terrain

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| TOPOLOGY | `topology(zone, x, y)` | Get terrain elevation at position | N |
| TOPOFLUSH | `topoflush(zone)` | Clear topology cache (after attr changes) | N |
| DEPTH | `depth(zone[, x, y])` | Water depth at position | N |
| ISLANDCHK | `islandchk(zone[, x, y])` | Is position on an island? | N |
| TOPOMAP | `topomap(zone[, x, y, radius])` | ASCII terrain map | N |
| CURRENT | `current(zone[, x, y, tide_phase, storm, wind_hdg, wind_str, medium])` | Water/air current vector | N |
| TEMPERATURE | `temperature(zone[, x, y, depth])` | Temperature at position | N |

### Map Instance / POI

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| MAPINSTANCE | `mapinstance(zone[, args...])` | Create/query map instances | N |
| MAPPARSE | `mapparse(zone, data)` | Parse map data | N |
| POIFORMAT | `poiformat(poi[, format])` | Format POI for display | N |
| POIPARSE | `poiparse(field, data)` | Parse POI data | N |
| POIINRANGE | `poiinrange(poi, x y, radius)` | Is POI within range? | N |
| POIDIST | `poidist(poi1, poi2)` | Distance between POIs | N |
| POIBEARING | `poibearing(pos, poi)` | Bearing to POI | N |

### Examples

```
> think hname(8)
N
> think gridabs(EL, 453, NE)
114 453
> think absgrid(114, 453)
EL-453-NE
> think gridcourse(AA-000-NE, AZ-500-NE)
7 502.624824
> think bearing(0 0, 100 100)
4
> think topology(#1234, 50, 50)
-45
> think current(#1234, 50, 50, 0.5, 0.3, 8, 15, water)
2.3 -1.1
```

---

## 10. JSON

Source: `json.go`

All JSON functions are **new in GoTinyMUSH** (N).

### Core JSON Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| JSON | `json(type[, args...])` | Create JSON value (object, array, string, number, bool, null) | N |
| JSON_QUERY | `json_query(json, op[, path])` | Query JSON (get, exists, type, members/lkeys, count, isnull) | N |
| JSON_MOD | `json_mod(json, op, path[, value])` | Modify JSON (set, insert, replace, remove, push, merge) | N |
| JSON_PP | `json_pp(json)` | Pretty-print JSON (2-space indent) | N |
| JSON_TEST | `json_test(json)` | Validate JSON (returns 1 or error) | N |

### Conversion Functions

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| STRINGTOJSON | `stringtojson(str)` | MUSH string to JSON string literal (quoted+escaped) | N |
| LISTTOJSON | `listtojson(list[, delim[, type]])` | MUSH list to JSON array (type: string/number/auto) | N |
| JSONTOLIST | `jsontolist(json[, delim])` | JSON array to MUSH list | N |
| JSONESCAPE | `jsonescape(str)` | Escape for JSON embedding (no quotes) | N |

### JSON-Array Bridge

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| JSON_TO_ARRAY | `json_to_array(json, arrayname)` | Load JSON into named array | N |
| ARRAY_TO_JSON | `array_to_json(arrayname[, mode])` | Convert array to JSON (mode: array/object/nested) | N |

### Path Notation

JSON paths use dot notation: `key.subkey.0` for nested access. Array indices are 0-based numbers.

### json_query Operations

- **get** - Return value at path (strings unquoted, objects/arrays as JSON)
- **exists** - Returns 1/0
- **type** - Returns: object, array, string, number, boolean, null
- **members/lkeys/keys** - List keys (objects) or indices (arrays)
- **count/size** - Element count
- **isnull** - Returns 1 if value is null

### json_mod Operations

- **set** - Set value at path (creates intermediates)
- **insert** - Set only if key does NOT exist
- **replace** - Set only if key DOES exist
- **remove/delete** - Remove key/index
- **push/append** - Append to array
- **patch/merge** - Merge objects

### Examples

```
> think json(object, name, Moravel, level, 10)
{"level":10,"name":"Moravel"}
> think json_query({"hp":100,"mp":50}, get, hp)
100
> think json_query({"a":{"b":3}}, get, a.b)
3
> think json_mod({"hp":100}, set, mp, 50)
{"hp":100,"mp":50}
> think listtojson(1 2 3, , number)
[1,2,3]
> think jsontolist(["red","green","blue"])
red green blue
> think json_mod([1,2,3], push, , 4)
[1,2,3,4]
> think json_query({"items":["sword","shield"]}, count, items)
2
```

---

## 11. Comsys

Source: `comsys.go`

Channel/communication system functions.

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| CINFO | `cinfo(channel, field)` | Channel info (owner, description, flags, etc.) | C |
| COMLIST | `comlist([separator])` | List visible channels | C |
| CWHO | `cwho(channel)` | Connected listeners on channel | C |
| CWHOALL | `cwhoall(channel)` | All subscribers (incl. offline) | C |
| COMOWNER | `comowner(channel)` | Channel owner dbref | C |
| COMDESC | `comdesc(channel)` | Channel description | C |
| COMHEADER | `comheader(channel)` | Channel header | C |
| COMALIAS | `comalias(player)` | Player's channel aliases | C |
| COMINFO | `cominfo(player, alias)` | Channel name for alias | C |
| COMTITLE | `comtitle(player, alias)` | Player's title on channel | C |
| CEMIT | `cemit(channel, message)` | Emit to channel | C |

### Examples

```
> think comlist()
Public Staff Admin
> think cwho(Public)
#1 #42 #100
> think comowner(Public)
#1
```

---

## 12. Mail

Source: `mail.go`

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| MAIL | `mail()` or `mail(player)` | Message count or "total unread cleared" stats | C |
| MAILFROM | `mailfrom(num)` | Sender dbref of message N | C |
| MAILSUBJ | `mailsubj(num)` | Subject of message N | C |

### Examples

```
> think mail()
5
> think mail(me)
5 2 0
> think mailfrom(1)
#42
```

---

## 13. Event Bus

Source: `eventbus.go`

All event bus functions are **new in GoTinyMUSH** (N). They implement a publish/subscribe event system that runs independently of the master command queue.

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| PUBLISH | `publish(queue_name, data)` | Publish event to queue. Returns 1 on success. | N |
| SUBSCRIBE | `subscribe(obj/attr, queue_name[, bind])` | Subscribe attribute to queue. Requires control of object. | N |
| UNSUBSCRIBE | `unsubscribe(obj/attr, queue_name)` | Remove subscription. Requires control. | N |
| QUEUES | `queues()` or `queues(name[, mode])` | List queues, or query info/subs/stats for specific queue. | N |

### Event Bus Architecture

- Events published via `publish()` are delivered during the bus phase of each tick (after the master queue)
- Bus handlers are triggered with `%0` = event data, `%1` = queue name
- Generation counter prevents infinite loops (max depth 2)
- 50 handler budget per tick with round-robin fairness
- Optional `bind` parameter on subscribe links the subscription to an object that must be in the same location

### Examples

```
> think subscribe(#1234/ON_TICK, sled.tick)
1
> think publish(sled.tick, {"phase":"thrust"})
1
> think queues()
sled.tick weather.update
> think queues(sled.tick, subs)
#1234
> think unsubscribe(#1234/ON_TICK, sled.tick)
1
```

---

## 14. Database

Source: `database.go`

All database functions are **new in GoTinyMUSH** (N). They provide SQL access to the embedded database.

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| SQL | `sql(query[, row_delim[, field_delim]])` | Execute SQL query. Delimiters default to space. | N |
| SQLESCAPE | `sqlescape(string)` | Escape string for SQL (prevents injection) | N |

### Examples

```
> think sql(SELECT name FROM objects WHERE type='PLAYER', |, -)
Moravel|Alec|Nyki
> think sqlescape(O'Brien)
O''Brien
```

---

## 15. Arrays

Source: `arrays.go`

All array functions are **new in GoTinyMUSH** (N). Arrays are per-player, named, mutable, variable-length containers of string values. They persist across restarts via bbolt.

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| ARRAY | `array(name[, maxsize])` | Create named array. Returns 1 on success. | N |
| ADESTROY | `adestroy(name)` | Destroy array. Returns 1. | N |
| APUSH | `apush(name, val1[, val2, ...])` | Append value(s) to end. Returns new length. | N |
| APOP | `apop(name)` | Remove and return last element. | N |
| ASHIFT | `ashift(name)` | Remove and return first element. | N |
| AUNSHIFT | `aunshift(name, val1[, val2, ...])` | Prepend value(s). Returns new length. | N |
| AGET | `aget(name, index)` | Read element at 1-based index. | N |
| ASET | `aset(name, index, value)` | Write element at 1-based index. Returns 1. | N |
| ALEN | `alen(name)` | Current length. | N |
| ALIST | `alist(name[, delim])` | Convert to MUSH list. | N |
| ALOAD | `aload(name, list[, delim])` | Bulk load from list (replaces contents). Returns count. | N |
| LARRAYS | `larrays()` | List all caller's arrays. | N |

### Examples

```
> think array(inventory, 100)
1
> think apush(inventory, sword, shield, potion)
3
> think aget(inventory, 2)
shield
> think apop(inventory)
potion
> think alist(inventory)
sword shield
> think aload(inventory, axe bow dagger)
3
> think alen(inventory)
3
> think adestroy(inventory)
1
```

---

## 16. Structures

Source: `structs.go`

Structure/instance system matching TinyMUSH 3.x `funvars.c`. Per-player namespaced typed data structures that persist across restarts.

### Type Codes

- `a` = any value
- `c` = single character
- `d` = dbref (#number)
- `i` = integer
- `f` = float
- `s` = string (no spaces)

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| STRUCTURE | `structure(name, components, types[, defaults[, delim]])` | Define a structure template | C |
| CONSTRUCT | `construct(instance, structure[, components, values[, delim]])` | Create an instance | C |
| DESTRUCT | `destruct(instance)` | Destroy an instance | C |
| UNSTRUCTURE | `unstructure(name)` | Delete structure definition (requires 0 instances) | C |
| Z | `z(instance, component)` | Read component value | C |
| MODIFY | `modify(instance, components, values[, delim])` | Update component values | C |
| LOAD | `load(instance, structure, text[, delim])` | Create instance from delimited text | C |
| UNLOAD | `unload(instance[, delim])` | Serialize instance to delimited text | C |
| READ | `read(obj/attr, instance, structure)` | Load instance from attribute | C |
| WRITE | `write(obj/attr, instance)` | Save instance to attribute | C |
| DELIMIT | `delimit(obj/attr, new-delim[, input-delim])` | Convert delimiter in stored structure | C |
| LSTRUCTURES | `lstructures()` | List defined structures | C |
| LINSTANCES | `linstances()` | List active instances | C |
| STORE | `store(name, value)` | Set and return named variable (setx + x) | C |
| ITEMS | `items(structure)` | Number of components | C |

### Examples

```
> think structure(ship, name hp maxhp speed, s i i f, unnamed 100 100 0.0)
1
> think construct(myship, ship, name hp, Falcon 250)
1
> think z(myship, name)
Falcon
> think z(myship, hp)
250
> think modify(myship, speed, 5.5)
1
> think unload(myship)
Falcon 250 100 5.5
> think destruct(myship)
1
> think unstructure(ship)
1
```

---

## 17. Attribute Definitions

Source: `attrdef.go`

Functions for managing user-defined attribute definitions. All are **new in GoTinyMUSH** (N).

| Function | Syntax | Description | Compat |
|----------|--------|-------------|--------|
| LATTRDEF | `lattrdef([pattern[, type]])` | List user-defined attribute names. Type filter: player, thing, room, exit. | N |
| ATTRDEFFLAGS | `attrdefflags(attrname)` | Get flags for attribute definition (e.g., "IVwp") | N |
| HASATTRDEF | `hasattrdef(attrname)` | Returns 1 if attribute definition exists | N |
| SETATTRDEF | `setattrdef(attrname, flags)` | Set flags on attribute definition (wizard-only). Prefix with ! to clear. | N |

### Examples

```
> think lattrdef()
MYTIMER MYDATA MYLOCK
> think hasattrdef(MYTIMER)
1
> think attrdefflags(MYTIMER)
IV
> think setattrdef(MYTIMER, VISUAL)

```

---

## Appendix: Function Count by Category

| Category | Count | New (N) |
|----------|-------|---------|
| Math | 85 | 16 |
| Strings | 68 | 22 |
| Lists | 52 | 11 |
| Objects | 62 | 12 |
| Conditionals | 18 | 8 |
| Iteration | 25 | 12 |
| Connection | 16 | 2 |
| Miscellaneous | 91 | 16 |
| Navigation | 36 | 36 |
| JSON | 12 | 12 |
| Comsys | 11 | 0 |
| Mail | 3 | 0 |
| Event Bus | 4 | 4 |
| Database | 2 | 2 |
| Arrays | 12 | 12 |
| Structures | 15 | 0 |
| Attribute Defs | 4 | 4 |
| **Total** | **~516** | **~169** |

Note: Some functions are aliases (e.g., LUNION=SETUNION, EXITS=LEXITS, OBJTYPE=TYPE, PENNIES=MONEY, STRTRUNC=LEFT, ISAPIKEY=HASAPIKEY, NONZERO=IF, PARSE=ITER, LIST=LOOP). The total unique implementation count is approximately 516, with aliases bringing the registered name count above 556.
