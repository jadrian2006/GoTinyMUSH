# Structure/Instance System Reference

GoTinyMUSH implements the TinyMUSH 3.2+ structure system, which provides typed, named data structures with defined components. Structures are defined with a schema (names, types, defaults), then instantiated as live objects with per-component values.

**Compat:** Matches TinyMUSH 3.x `funvars.c` structure system.

## Concepts

- **Structure definition**: A named template specifying component names, types, and default values. Like a `struct` type declaration.
- **Instance**: A live copy of a structure with its own values. Like a variable of that struct type.
- Both structures and instances are scoped per-player.
- Instances persist across reboots (stored in bbolt).
- The generic structure delimiter is form feed (`\f`, 0x0C), matching TinyMUSH convention for attribute storage.

## Type Codes

Each component in a structure has a type that is checked on construction and modification.

| Code | Type | Validation |
|------|------|------------|
| `a` | Any | No validation |
| `c` | Character | Exactly 1 character |
| `d` | Dbref | Must match `#<integer>` |
| `i` | Integer | Must parse as integer |
| `f` | Float | Must parse as float |
| `s` | String | No spaces, tabs, or newlines |

## Functions

### structure() -- Define a Structure

```
structure(<name>, <components>, <types>[, <defaults>[, <output-delim>]])
```

Define a named structure template. Returns `1` on success, `0` on failure.

- `<name>`: Structure name (case-insensitive, no dots allowed).
- `<components>`: Space-separated component names.
- `<types>`: Space-separated type codes (must match component count).
- `<defaults>`: Default values (parsed by output-delim, default space).
- `<output-delim>`: Delimiter for `unload()` output (default space).

Cannot redefine an existing structure.

### unstructure() -- Remove a Structure Definition

```
unstructure(<name>)
```

Delete a structure definition. Returns `1` on success, `0` if not found or instances still exist. All instances must be destroyed first.

### construct() -- Create an Instance

```
construct(<instance>, <structure>[, <components>, <values>[, <input-delim>]])
```

Create a named instance of a structure. Returns `1` on success, `0` on failure.

- Values start as the structure's defaults.
- Optional `<components>` and `<values>` override specific defaults at creation time.
- Type checking is enforced on all provided values.

### destruct() -- Destroy an Instance

```
destruct(<instance>)
```

Destroy an instance and decrement the structure's reference count. Returns `1` on success, `0` if not found.

### z() -- Read a Component

```
z(<instance>, <component>)
```

Read a component value from an instance. Returns empty string if instance or component not found.

### modify() -- Update Components

```
modify(<instance>, <components>, <values>[, <input-delim>])
```

Update one or more component values. Returns the number of components successfully modified. Type checking is enforced; invalid values are silently skipped.

### load() -- Create Instance from Text

```
load(<instance>, <structure>, <text>[, <input-delim>])
```

Parse delimited text and create a new instance. The number of delimited parts must exactly match the component count. All values are type-checked. Returns `1` on success, `0` on failure.

Default input delimiter is the structure's output-delim.

### unload() -- Serialize Instance to Text

```
unload(<instance>[, <output-delim>])
```

Serialize an instance's values to a delimited string. Default delimiter is the structure's configured output-delim.

### read() -- Load Instance from Attribute

```
read(<obj>/<attr>, <instance>, <structure>)
```

Read an attribute's value and create an instance from it, using form feed (`\f`) as the input delimiter. This is the counterpart to `write()`.

### write() -- Save Instance to Attribute

```
write(<obj>/<attr>, <instance>)
```

Serialize an instance using form feed delimiter and store it in an attribute. Requires `Controls` permission on the target object.

### delimit() -- Re-delimit Attribute Data

```
delimit(<obj>/<attr>, <new-delim>[, <input-delim>])
```

Read a structure-formatted attribute and output it with a different delimiter. Default input delimiter is form feed. Useful for displaying stored structure data in a human-readable format.

### lstructures() -- List Defined Structures

```
lstructures()
```

Return a space-separated list of the caller's defined structure names.

### linstances() -- List Active Instances

```
linstances()
```

Return a space-separated list of the caller's active instance names.

### items() -- Component Count

```
items(<structure>)
```

Return the number of components in a structure definition. Returns `0` if not found.

### store() -- Set and Return Named Variable

```
store(<name>, <value>)
```

Set a named register variable (XReg) and return the value. This is a convenience function that combines `setx()` and `x()` -- it stores the value and returns it in one call.

## Examples

### Define and Use a Character Sheet

```
think structure(charsheet, name class level hp maxhp, s s i i i, Unknown Fighter 1 10 10)
```

This creates a structure with 5 components: `name` (string), `class` (string), `level` (integer), `hp` (integer), `maxhp` (integer), with defaults.

### Create an Instance

```
think construct(hero, charsheet, name level hp maxhp, Aldric 5 45 45)
```

### Read and Modify

```
think z(hero, name)            -> Aldric
think z(hero, hp)              -> 45
think modify(hero, hp, 30)     -> 1
think z(hero, hp)              -> 30
```

### Serialize and Store

```
think write(me/CHARDATA, hero)
```

This stores the form-feed-delimited data in the attribute `CHARDATA`.

### Load from Attribute

```
think read(me/CHARDATA, restored_hero, charsheet)
think z(restored_hero, name)   -> Aldric
```

### Display Stored Data

```
think delimit(me/CHARDATA, |)  -> Aldric|Fighter|5|30|45
```

### Bulk Create from Text

```
think load(enemy, charsheet, Goblin Rogue 3 15 15)
think z(enemy, class)          -> Rogue
```

### Cleanup

```
think destruct(hero)
think destruct(enemy)
think destruct(restored_hero)
think unstructure(charsheet)
```

### Inspect

```
think lstructures()            -> charsheet
think linstances()             -> hero enemy
think items(charsheet)         -> 5
```

## Notes

- Structure names cannot contain dots.
- You cannot redefine a structure -- `unstructure()` it first (requires zero active instances).
- Instance names must be unique per player.
- The `read()`/`write()` pair uses form feed (`\f`) as the storage delimiter, which is invisible in most displays but safe for attribute storage.
- Both definitions and instances persist across restarts.
- The `store()` function operates on named registers (XRegs), not structure instances -- it is included in this module for historical reasons matching C TinyMUSH's `funvars.c`.
