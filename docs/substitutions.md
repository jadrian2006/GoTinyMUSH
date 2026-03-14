# Substitutions Reference

GoTinyMUSH substitution system documentation, sourced from `pkg/eval/exec.go`
(`handlePercent` and `exec` methods) and `pkg/eval/context.go` (eval flags,
register data).

## Overview

MUSH expressions are evaluated by the `Exec()` engine, which processes three
categories of substitutions:

1. **%-substitutions** -- replaced inline during evaluation
2. **#-substitutions** -- loop/switch iteration tokens
3. **[]-evaluation** -- function calls and nested expressions

## Standard %-Substitutions

### Object References

| Code | Description | Compat |
|------|-------------|--------|
| `%#` | Enactor/cause dbref (who triggered the action) | ✅ |
| `%!` | Executor dbref (object running the code) | ✅ |
| `%@` | Caller dbref (object that called `u()` to get here) | ✅ |
| `%l` / `%L` | Location of the enactor (dbref). Suppressed when `EvNoLocation` is set | ✅ |

### Enactor Name and Pronouns

| Code | Description | Compat |
|------|-------------|--------|
| `%n` | Enactor's name (lowercase) | ✅ |
| `%N` | Enactor's name (capitalized) | ✅ |
| `%s` | Subjective pronoun: he/she/it/they (based on SEX attr) | ✅ |
| `%S` | Subjective pronoun (capitalized) | ✅ |
| `%o` | Objective pronoun: him/her/it/them | ✅ |
| `%O` | Objective pronoun (capitalized) | ✅ |
| `%p` | Possessive pronoun: his/her/its/their | ✅ |
| `%P` | Possessive pronoun (capitalized) | ✅ |
| `%a` | Absolute possessive: his/hers/its/theirs | ✅ |
| `%A` | Absolute possessive (capitalized) | ✅ |

Pronouns are determined by the enactor's SEX attribute:

| SEX Value | Subjective | Objective | Possessive | Absolute |
|-----------|-----------|-----------|-----------|----------|
| Male (M) | he | him | his | his |
| Female (F/W) | she | her | her | hers |
| Plural (P) | they | them | their | theirs |
| Neuter (default) | it | it | its | its |
| Unset | (object name) | (object name) | (name)s | (name)s |

### Whitespace and Literals

| Code | Description | Compat |
|------|-------------|--------|
| `%r` / `%R` | Newline (CRLF) | ✅ |
| `%t` / `%T` | Tab character | ✅ |
| `%b` / `%B` | Space (literal, not compressed) | ✅ |
| `%%` | Literal `%` character | ✅ |

### Command Arguments (%0-%9)

| Code | Description | Compat |
|------|-------------|--------|
| `%0` through `%9` | Positional arguments passed via `u()`, `@trigger`, `@switch`, etc. | ✅ |
| `%+` | Number of function arguments (count of cargs) | ✅ |

Arguments are set by:
- `u(obj/attr, arg0, arg1, ...)` -- args become `%0`, `%1`, etc.
- `@trigger obj/attr = arg0, arg1, ...`
- `iter()`, `switch()`, and similar functions set `%0` for their body evaluations
- `$-command` pattern captures set `%0`-`%9`

### Q-Registers (%q0-%qz, Named)

| Code | Description | Compat |
|------|-------------|--------|
| `%q0` through `%q9` | Numbered registers (0-9) | ✅ |
| `%qa` through `%qz` | Lettered registers (a-z) | ✅ |
| `%q<name>` | Named register (arbitrary name in angle brackets) | ✅ |

Total of 36 positional registers (0-9 = indices 0-9, a-z = indices 10-35).

Set with `setq()` / `setr()`, read with `r()` or `%q` substitutions:
```
[setq(0, hello)]%q0          -> hello
[setq(a, world)]%qa          -> world
[setq(myvar, test, <myvar>)] -> (sets named register)
%q<myvar>                    -> test
```

### Attribute Shortcuts

| Code | Description | Compat |
|------|-------------|--------|
| `%va` through `%vz` | Value of attribute VA-VZ on the executor (`%!`) | ✅ |
| `%=<attrname>` | Value of any attribute on the executor (shorthand for `get(%!/attr)`) | ✅ |

Examples:
```
%va         -> contents of VA attribute on executor
%=<DESC>    -> contents of DESC attribute on executor
```

### X-Variables

| Code | Description | Compat |
|------|-------------|--------|
| `%_x` | Single-char x-variable (shared with `setx()`/`store()`) | ✅ |
| `%_<name>` | Named x-variable (long form) | ✅ |

X-variables share the same store as named registers (`XRegs`). They are set
with `setx()`, `store()`, and `xvars()`.

### Command and Pipe

| Code | Description | Compat |
|------|-------------|--------|
| `%m` / `%M` | The current command being executed (always available) | ✅ |
| `%c` / `%C` | Conditional: current command only when `c_is_command` is set | ✅ |
| `%\|` | Piped output (output from the previous command in a pipe chain) | ✅ |

### ANSI Color (%x)

| Code | Description | Compat |
|------|-------------|--------|
| `%xn` | Reset / normal | ✅ |
| `%xh` | Hilite / bold | ✅ |
| `%xu` | Underline | ✅ |
| `%xf` | Flash / blink | ✅ |
| `%xi` | Inverse / reverse | ✅ |
| `%xx` | Black foreground | ✅ |
| `%xr` | Red foreground | ✅ |
| `%xg` | Green foreground | ✅ |
| `%xy` | Yellow foreground | ✅ |
| `%xb` | Blue foreground | ✅ |
| `%xm` | Magenta foreground | ✅ |
| `%xc` | Cyan foreground | ✅ |
| `%xw` | White foreground | ✅ |
| `%x<208>` | Extended 256-color (foreground) | ✅ |
| `%x<#FF5733>` | RGB hex color (foreground) | ✅ |
| `%x/<208>` | Extended 256-color (background) | ✅ |
| `%x/<#FF5733>` | RGB hex color (background) | ✅ |

Uppercase variants (`%xR`, `%xG`, etc.) are equivalent to lowercase.
ANSI auto-terminates with `\033[0m` at the end of evaluation if any color
codes were emitted.

### Iterator Text (%i, %j)

| Code | Description | Compat |
|------|-------------|--------|
| `%i0` | Current iter() token (innermost loop) | ✅ |
| `%i1` | Parent iter() token (one level up) | ✅ |
| `%i-0` | Absolute iter() token at nesting level 0 | ✅ |
| `%i-1` | Absolute iter() token at nesting level 1 | ✅ |
| `%j0` | Current iter2() second token (for ilev()-based iteration) | ✅ |
| `%j-0` | Absolute iter2() second token at level 0 | ✅ |

`%i0` is equivalent to `itext(0)` and `##`.
`%i1` is equivalent to `itext(1)`.

## #-Substitutions

Active only inside `iter()`, `parse()`, `switch()`, and related functions.

| Code | Description | Compat |
|------|-------------|--------|
| `##` | Current iteration token (equivalent to `%i0` / `itext(0)`) | ✅ |
| `#@` | Current iteration count (0-based, equivalent to `inum(0)`) | ✅ |
| `#+` | Current iter2() second token | ✅ |
| `#$` | Switch token (the value being matched in `switch()`) | ✅ |
| `#!` | Current nesting level (for nested iter/switch) | ✅ |

These tokens are only substituted when the evaluator is inside a loop
(`ctx.Loop.InLoop > 0`) or switch (`ctx.Loop.InSwitch > 0`). Outside of
loops, `#` is treated as a literal character.

## Function Evaluation

### Bracket Evaluation

| Syntax | Description |
|--------|-------------|
| `[function(args)]` | Evaluates function and substitutes the result inline |
| `[expr1][expr2]` | Multiple expressions concatenated |

Brackets invoke the function evaluator with `EvFCheck | EvFMand`, meaning
function names are mandatory -- an unknown function inside brackets returns
`#-1 FUNCTION (NAME) NOT FOUND`.

### Bare Function Calls

Outside of brackets, function calls are recognized only at the START of an
evaluation unit (one-shot `EvFCheck`). After any non-function-name character,
bare function checking is disabled:

```
min(3,7,1)        -> 1           (bare function at start)
hello min(3,7,1)  -> hello min(3,7,1)  (literal after space)
[min(3,7,1)]      -> 1           (always evaluated inside brackets)
```

### Curly Brace Grouping

| Syntax | Description |
|--------|-------------|
| `{text}` | In normal mode: protects content from function evaluation |
| `{text}` | In EvStrip mode (inside `switch()`/`iter()`): grouping syntax, braces stripped, content re-evaluated |

In normal evaluation, `{[add(1,2)]}` preserves the function call literally.
Inside `switch()` or `iter()` body branches, `{[add(1,2)]}` strips braces
and evaluates to `3`.

### Backslash Escaping

| Syntax | Description | Compat |
|--------|-------------|--------|
| `\x` | Produces literal `x` (backslash is always stripped) | ⚠️ |

GoTinyMUSH always strips the backslash (matching `EV_STRIP_ESC` behavior).
C TinyMUSH's default preserves the backslash in some contexts. This is a known
behavioral difference -- see the `validate` package's `PercentChecker` for
automated detection of `\\%` patterns that need adjustment.

### Space Compression

By default, consecutive spaces are compressed to a single space (matching
C TinyMUSH behavior). Use `%b` to insert guaranteed literal spaces. Space
compression can be disabled with `EvNoCompress`.

### Argument Handling

Function arguments are split on commas respecting nesting of `[]`, `()`, and
`{}`. Leading and trailing spaces are trimmed from each argument (matching
C TinyMUSH's `EV_STRIP_LS | EV_STRIP_TS`):

```
add( 3 , 7 )    -> 10    (spaces trimmed from args)
```

### @function (User-Defined Functions)

User-defined functions created with `@function` are invoked the same way as
built-in functions. They support two special flags:

| Flag | Effect |
|------|--------|
| PRIVILEGED | Evaluates as the object owner, not the caller |
| PRESERVE | Saves and restores q-registers around the call |

### Evaluation Limits

| Limit | Description |
|-------|-------------|
| Function nesting | Configurable (`FuncNestLim`), returns `#-1 FUNCTION RECURSION LIMIT EXCEEDED` |
| Function invocations | Configurable (`FuncInvkLim`), returns `#-1 FUNCTION INVOCATION LIMIT EXCEEDED` |
| CPU time | Deadline checked every 100 invocations, returns `#-1 CPU TIME LIMIT EXCEEDED` |
| Output size | Configurable limit, returns `#-1 OUTPUT LIMIT EXCEEDED` |

## Quick Reference

```
%#    enactor dbref          %!    executor dbref
%@    caller dbref           %l    enactor location
%n    enactor name           %N    enactor Name (capitalized)
%s/%o/%p/%a  pronouns        %S/%O/%P/%A  Pronouns (capitalized)

%0-%9  command arguments     %+    arg count
%q0-%q9, %qa-%qz  registers %q<name>  named register
%va-%vz  VA-VZ attributes   %=<attr>  any attribute
%_x / %_<name>  x-variables

%r    newline                %t    tab
%b    space                  %%    literal %
%m    current command        %c    conditional command
%|    piped output

%xn   ANSI reset             %xr   red  %xg  green  %xb  blue
%x<N> 256-color              %x<#RRGGBB>  RGB color

##    iter token             #@    iter count
#+    iter2 token            #$    switch token
#!    nesting level

[function(args)]             {literal grouping}
\x   escape next char
```
