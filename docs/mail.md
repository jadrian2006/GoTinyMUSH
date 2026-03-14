# Mail System Reference

GoTinyMUSH implements the full TinyMUSH 3.x `@mail` system with compose/send/receive workflow, folder organization, mail aliases, and softcode-accessible functions.

**Compat:** Full C parity (31/31 switches). Go correctly refuses `@mail/clear` on safe messages; C has a bug that allows it.

## @mail Command

### Reading & Listing

| Command | Description |
|---------|-------------|
| `@mail` | List inbox (same as `@mail/list`) |
| `@mail <num>` | Read message number (same as `@mail/read`) |
| `@mail/read <num>` | Read a specific message |
| `@mail/list` | List all inbox messages with flags, sender, date, subject |
| `@mail/review [<player>]` | Show your sent mail history (optionally filtered to one recipient) |

### Composing

The compose workflow uses a draft system. Start a draft, add body text, then send.

| Command | Description |
|---------|-------------|
| `@mail/to <player list>` | Set draft recipients (starts or updates draft) |
| `@mail/cc <player list>` | Set CC recipients |
| `@mail/bcc <player list>` | Set BCC recipients (hidden from other recipients) |
| `@mail/subject <text>` | Set draft subject (alias: `/sub`) |
| `- <text>` | Append text to draft body |
| `~ <text>` | Prepend text to draft body |
| `@mail/edit <old>=<new>` | Search/replace in draft body (first occurrence) |
| `@mail/proof` | Preview the current draft |
| `@mail/send` | Send the draft and clear it |
| `@mail/abort` | Discard the current draft |

### Quick Send

| Command | Description |
|---------|-------------|
| `@mail <player>=<subject>/<body>` | One-line send (Go syntax) |
| `@mail <player>=<subject>` | Start compose mode with recipient and subject set (C compat) |
| `@mail/quick <player>/<subject>=<body>` | C-compatible one-line send |

### Reply & Forward

| Command | Description |
|---------|-------------|
| `@mail/reply <num>` | Start reply to sender (auto-prefixes "Re:") |
| `@mail/replyall <num>` | Reply to sender + all To/CC recipients |
| `@mail/quote <num>` | Reply with original message quoted in body (`> ` prefix) |
| `@mail/forward <num>=<player list>` | Forward message (auto-prefixes "Fwd:"), alias: `/fwd` |
| `@mail/retract <player>=<num>` | Retract an unread message from a recipient's inbox |

### Message Flags

| Command | Description |
|---------|-------------|
| `@mail/clear <num>` | Mark message for deletion (refuses if safe) |
| `@mail/unclear <num>` | Remove cleared flag |
| `@mail/safe <num>` | Protect message from clearing and purge |
| `@mail/tag <num>` | Set user tag flag |
| `@mail/untag <num>` | Remove tag flag |
| `@mail/urgent <num>` | Mark message as urgent |

Flag display codes in listings: `N` = new/unread, `C` = cleared, `U` = urgent, `S` = safe, `T` = tagged, `F` = forwarded, `R` = replied.

### Folders

Messages can be filed into folders 0-14.

| Command | Description |
|---------|-------------|
| `@mail/folder` | List folders with message counts |
| `@mail/folder <num>=<name>` | Name a folder |
| `@mail/file <msg>=<folder>` | Move a message to a folder |

### Purge & Nuke

| Command | Description |
|---------|-------------|
| `@mail/purge` | Permanently delete all cleared messages |
| `@mail/nuke` | Destroy ALL mail in the system (wizard only) |

### Statistics

| Command | Description |
|---------|-------------|
| `@mail/stats [<player>]` | Basic counts: total, unread, cleared |
| `@mail/dstats [<player>]` | Detailed: total, unread, cleared, tagged, urgent, safe |
| `@mail/fstats [<player>]` | Per-folder message counts |
| `@mail/debug` | Database sanity check: orphaned messages (wizard only) |

Querying another player's stats requires wizard privilege.

## @malias Command

Mail aliases let you send to groups of players using `*aliasname` in recipient lists. Each player can create their own aliases; God-owned aliases (#1) are visible to everyone.

### Creating & Managing

| Command | Description |
|---------|-------------|
| `@malias *name=<player list>` | Create a new alias with members |
| `@malias *name` | Show alias details (owner, description, members) |
| `@malias` | List your aliases (wizard: list all) |
| `@malias/list` | Same as bare `@malias` |
| `@malias/delete *name` | Delete an alias you own |
| `@malias/rename *old=*new` | Rename an alias (1-31 characters) |

### Membership

| Command | Description |
|---------|-------------|
| `@malias/add *name=<player>` | Add a player to an alias |
| `@malias/remove *name=<player>` | Remove a player from an alias |

Maximum members per alias: 100 (matching C).

### Properties

| Command | Description |
|---------|-------------|
| `@malias/desc *name=<text>` | Set alias description |
| `@malias/chown *name=<player>` | Transfer ownership (wizard only) |
| `@malias/status` | Show total alias count (wizard only) |

### Using Aliases

Include `*aliasname` anywhere in a recipient list:

```
@mail/to *staff
@mail *builders,*coders=Meeting tomorrow/Don't forget.
```

Alias resolution: player-owned aliases take priority, then God-owned.

## Mail Functions

| Function | Description |
|----------|-------------|
| `mail()` | Returns total message count for the caller |
| `mail(<player>)` | Returns `total unread cleared` (wizard only for other players) |
| `mailfrom(<num>)` | Returns sender dbref (`#123`) of message number |
| `mailsubj(<num>)` | Returns subject line of message number |

## Examples

**Quick send:**
```
@mail Alec=Hello/This is a test message.
```

**Compose workflow:**
```
@mail/to Alec Nyki
@mail/subject Important Update
- First line of the message body.
- Second line appended.
@mail/proof
@mail/send
```

**Reply with quote:**
```
@mail/quote 3
- I agree with your proposal.
@mail/send
```

**Forward to a group:**
```
@mail/fwd 5=*staff
```

**Retract unread mail:**
```
@mail/retract Nyki=7
```

**File and organize:**
```
@mail/folder 1=Important
@mail/file 3=1
@mail/safe 3
```

**Softcode mail check:**
```
&STARTUP obj=@pemit %#=You have [mail()] messages.
```
