# Array System Reference

GoTinyMUSH provides per-player named mutable arrays as an alternative to MUSH list manipulation. Arrays are flat (1D), store string values, support stack/queue operations, and persist across reboots.

**Compat:** Entirely new system, not present in C TinyMUSH.

## Functions

### array() -- Create an Array

```
array(<name>[, <maxsize>])
```

Create a named array. Returns `1` on success, `0` if the name is already taken or invalid.

- Names are case-insensitive (stored lowercase).
- `maxsize` of 0 (default) means unlimited.
- Arrays are scoped per-player -- each player has their own namespace.

### adestroy() -- Destroy an Array

```
adestroy(<name>)
```

Delete an array and all its contents. Returns `1` on success, `0` if not found.

### apush() -- Append (Push to End)

```
apush(<name>, <value>[, <value2>, ...])
```

Append one or more values to the end of the array. Stops adding if `maxsize` is reached. Returns the new length.

### apop() -- Pop from End

```
apop(<name>)
```

Remove and return the last element. Returns empty string if the array is empty or not found.

### ashift() -- Shift from Front

```
ashift(<name>)
```

Remove and return the first element. Returns empty string if the array is empty or not found.

### aunshift() -- Prepend (Unshift to Front)

```
aunshift(<name>, <value>[, <value2>, ...])
```

Prepend one or more values to the beginning of the array. Values are inserted in order: `aunshift(a, x, y)` results in `[x, y, ...existing...]`. Stops adding if `maxsize` is reached. Returns the new length.

### aget() -- Read by Index

```
aget(<name>, <index>)
```

Read element at 1-based index. Returns empty string if index is out of range or array not found.

### aset() -- Write by Index

```
aset(<name>, <index>, <value>)
```

Write element at 1-based index. Returns `1` on success, `0` if index is out of range or array not found.

### alen() -- Length

```
alen(<name>)
```

Return the number of elements. Returns `0` if array not found.

### alist() -- Convert to List

```
alist(<name>[, <delim>])
```

Return all elements joined by delimiter (default: space). Returns empty string if array not found.

### aload() -- Bulk Load from List

```
aload(<name>, <list>[, <delim>])
```

Replace array contents with elements parsed from a MUSH list. Default delimiter is space (uses `Fields` splitting, so multiple spaces collapse). Truncates to `maxsize` if set. Returns the new length.

### larrays() -- List All Arrays

```
larrays()
```

Return a space-separated, alphabetically sorted list of the caller's array names.

## Examples

### Stack (LIFO)

```
think array(mystack)
think apush(mystack, first)
think apush(mystack, second)
think apush(mystack, third)
think apop(mystack)            -> third
think apop(mystack)            -> second
```

### Queue (FIFO)

```
think array(myqueue)
think apush(myqueue, first)
think apush(myqueue, second)
think ashift(myqueue)          -> first
think ashift(myqueue)          -> second
```

### Random Access

```
think array(inventory, 10)
think apush(inventory, sword)
think apush(inventory, shield)
think apush(inventory, potion)
think aget(inventory, 2)       -> shield
think aset(inventory, 2, armor)
think aget(inventory, 2)       -> armor
```

### Bulk Load and Export

```
think array(colors)
think aload(colors, red green blue yellow)
think alen(colors)             -> 4
think alist(colors, |)         -> red|green|blue|yellow
```

### Custom Delimiter Load

```
think aload(colors, red|green|blue, |)
think alist(colors)            -> red green blue
```

### List All Arrays

```
think larrays()                -> colors inventory myqueue mystack
```

### Cleanup

```
think adestroy(mystack)
think adestroy(myqueue)
```

## Notes

- Arrays persist across `@restart` and `@shutdown` (stored in bbolt).
- Each player's arrays are independent -- `array(foo)` for player A and player B are separate.
- `maxsize` is enforced on `apush`, `aunshift`, and `aload` but not on `aset` (which only writes to existing indices).
- All indices are 1-based, matching MUSH conventions.
