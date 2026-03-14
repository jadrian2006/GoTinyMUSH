# Channel System (Comsys) Reference

GoTinyMUSH implements the TinyMUSH 3.x channel communication system (comsys). Players subscribe to channels via personal aliases and can talk, listen, set titles, and manage channel properties.

**Compat:** Full C parity.

## Channel Management (Wizard/Owner)

### @ccreate

Create a new channel. Wizard only.

```
@ccreate <channel name>
```

New channels default to all six type permission flags enabled (P_Join, P_Trans, P_Recv, O_Join, O_Trans, O_Recv).

### @cdestroy

Destroy a channel and remove all subscriptions. Requires channel owner or Comm_All power.

```
@cdestroy <channel name>
```

### @cset

Set channel properties. Requires channel owner or Comm_All power.

```
@cset <channel>=<option>
```

**Options:**

| Option | Description |
|--------|-------------|
| `description <text>` | Set channel description |
| `header <text>` | Set channel message header (default: `[ChannelName]`) |
| `public` | Anyone can join |
| `private` | Only players passing the join lock can join |
| `loud` | Show connect/disconnect messages |
| `quiet` | Suppress connect/disconnect messages |
| `spoof` / `!spoof` | Allow/disallow spoofing on the channel |
| `mogrifier <obj>` | Set mogrifier object for message transformation |
| `nomogrifier` | Clear mogrifier |

**Per-type permission flags** (set/clear with `!` prefix):

| Flag | Description |
|------|-------------|
| `p_join` / `!p_join` | Players can join |
| `p_trans` / `!p_trans` | Players can transmit |
| `p_recv` / `!p_recv` | Players can receive |
| `o_join` / `!o_join` | Objects can join |
| `o_trans` / `!o_trans` | Objects can transmit |
| `o_recv` / `!o_recv` | Objects can receive |

**Lock expressions:**

| Option | Description |
|--------|-------------|
| `joinlock <expr>` | Set join lock expression |
| `joinlock` | Clear join lock |
| `translock <expr>` | Set transmit lock expression |
| `translock` | Clear transmit lock |
| `recvlock <expr>` | Set receive lock expression |
| `recvlock` | Clear receive lock |

### @cinfo

Show detailed channel configuration. Requires channel owner or wizard.

```
@cinfo <channel>
```

Displays: owner, description, header, message count, flags, locks, mogrifier, charge, subscriber count.

### @clist

List all channels with message counts, owners, and descriptions.

```
@clist
```

### @cwho

Show connected, listening players on a channel. Requires channel owner or Comm_All.

```
@cwho <channel>
```

### @cboot

Remove a player from a channel (deletes all their aliases for it). Requires channel owner or Comm_All.

```
@cboot <channel>=<player>
```

### @cemit

Emit a raw message to a channel (bypasses normal `Name says` format). Requires channel owner or Comm_All.

```
@cemit <channel>=<message>
```

## Player Commands

### addcom

Subscribe to a channel with a personal alias.

```
addcom <alias>=<channel>
```

Example: `addcom pub=Public` -- now typing `pub Hello` sends to Public.

### delcom

Remove a channel alias (unsubscribe from that alias).

```
delcom <alias>
```

### clearcom

Remove all your channel aliases at once.

```
clearcom
```

### comlist

List your channel aliases with status and titles.

```
comlist
```

Displays: alias, channel name, on/off status, title.

### comtitle

Set your display title on a channel.

```
comtitle <alias>=<title>
comtitle <alias>=           (clear title)
```

### allcom

Toggle all your channels on or off, or show who's on each.

```
allcom on
allcom off
allcom who
```

### Talking on Channels

Type your alias followed by a space and your message:

```
pub Hello everyone!
```

This sends: `[Public] YourName says, "Hello everyone!"`

## Channel Flags

| Flag | Hex | Description |
|------|-----|-------------|
| ChanPublic | 0x10 | Anyone can join |
| ChanLoud | 0x20 | Show connect/disconnect |
| ChanPJoin | 0x40 | Players can join (flag-based) |
| ChanPTrans | 0x80 | Players can transmit |
| ChanPRecv | 0x100 | Players can receive |
| ChanOJoin | 0x200 | Objects can join |
| ChanOTrans | 0x400 | Objects can transmit |
| ChanORecv | 0x800 | Objects can receive |
| ChanSpoof | 0x1000 | Allow spoofing |
| ChanNoTitles | 0x10000 | Suppress titles (Go extension) |

## Functions

| Function | Description |
|----------|-------------|
| `cinfo(<channel>, <field>)` | Return a field value. Fields: owner, description, header, flags, numsent, subscribers, joinlock, translock, recvlock, charge. Requires channel owner or wizard. |
| `comlist()` | Space-separated list of visible channel names |
| `comlist(<sep>)` | Custom-separated list of visible channel names |
| `cwho(<channel>)` | Space-separated dbrefs of connected, listening players |
| `cwhoall(<channel>)` | Space-separated dbrefs of all subscribers (including offline) |
| `comowner(<channel>)` | Owner dbref of channel |
| `comdesc(<channel>)` | Channel description |
| `comheader(<channel>)` | Channel header string |
| `comalias(<player>)` | Space-separated alias names for a player (requires Controls or Comm_All) |
| `cominfo(<player>, <alias>)` | Channel name for a player's alias (requires Controls or Comm_All) |
| `comtitle(<player>, <alias>)` | Title for a player's alias (requires Controls or Comm_All) |
| `cemit(<channel>, <message>)` | Send message to channel (function form, requires Comm_All or owner) |

## Examples

**Create and configure a channel:**
```
@ccreate Gossip
@cset Gossip=description The town gossip channel
@cset Gossip=header [Gossip]
@cset Gossip=public
@cset Gossip=loud
```

**Join and use:**
```
addcom gos=Gossip
gos Did you hear about the dragon?
```

**Set a title:**
```
comtitle gos=Town Crier
gos Hear ye, hear ye!
```
Output: `[Gossip] Town Crier YourName says, "Hear ye, hear ye!"`

**Lock a channel to a flag:**
```
@cset Secret=private
@cset Secret=joinlock FLAG^STAFF
```

**Softcode channel query:**
```
say The Public channel has [words(cwho(Public))] listeners.
```
