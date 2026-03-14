# Bot System Reference

> All features on this page are new in GoTinyMUSH and have no C TinyMUSH equivalent.

## Overview

GoTinyMUSH supports NPC bots via ROBOT-flagged player objects that authenticate
with API keys instead of passwords. Bots connect over WebSocket and are controlled
by external programs (LLM agents, automation scripts, MCP servers).

**Architecture** -- two-layer design:

1. **DARK wizard THING consoles** hold all privileged `$commands` (buy, sell,
   rent, restock). They are invisible to players and carry wizard-level locks.
2. **ROBOT PLAYER objects** are unprivileged personality shells. They have no
   wizard access; their only role is to speak and emote under external control.

This separation ensures an AI-controlled bot can never execute privileged game
commands directly.

---

## Commands

### @botcreate

Creates a bot player in one step: player object + ROBOT flag + API key.

```
@botcreate <name>
```

- **Permission**: Wizard only.
- Creates a PLAYER object at the caller's current location.
- Sets the ROBOT flag automatically.
- Sets the bot's home to the caller's location.
- No password is set -- the bot authenticates only via API key.
- Generates a 64-character hex API key (displayed once).
- The server stores only the SHA-256 hash of the key.

**Example:**

```
> @botcreate Cormac
Bot player Cormac created as #13364 with ROBOT flag.
API Key: a1b2c3d4e5f6...
Store this key securely - it will not be shown again.
Authenticate via: POST /api/v1/auth/apikey with {"key":"a1b2...","dbref":"#13364"}
```

### @pcreate

Creates a regular player (no ROBOT flag, no API key). Useful when you want to
set up a bot manually or create a normal player account.

```
@pcreate <name> = <password>
```

- **Permission**: Wizard only.
- Player is placed at the game's starting room.
- Player is NOT logged in automatically.
- Name must be at least 2 characters, no `"` or `;` characters.

**Example:**

```
> @pcreate Alec = secretpass
Player Alec created as #500.
```

To convert a `@pcreate`d player into a bot manually:

```
@set #500=ROBOT
@apikey generate #500
```

### @apikey

Generates or revokes API keys for players and things.

```
@apikey generate <object>
@apikey revoke <object>
```

- **Permission**: Wizard, or caller has the `bot` power and owns the target.
- Only PLAYER and THING type objects can have API keys.
- `generate` creates a new 64-character hex key (32 random bytes). The raw key
  is shown exactly once. The server stores the SHA-256 hash.
- `revoke` deletes the stored key hash, immediately invalidating any tokens
  issued from it.
- If the target is a PLAYER without the ROBOT flag, a warning is displayed.

**Examples:**

```
> @apikey generate #1234
API key generated for TestBot(#1234).
Key: a1b2c3d4e5f6...
Store this key securely - it will not be shown again.

> @apikey revoke #1234
API key revoked for TestBot(#1234).
```

### bot power

The `bot` power allows a non-wizard player to manage API keys for objects they
own via `@apikey`. Without this power, only wizards can generate and revoke keys.

```
@power <player>=bot       Grant bot power
@power <player>=!bot      Revoke bot power
```

---

## API Key Authentication

Bots authenticate through a two-step flow: API key exchange for a JWT, then
WebSocket connection with the JWT.

### Step 1 -- Obtain JWT

```
POST /api/v1/auth/apikey
Content-Type: application/json

{
  "key": "<64-char-hex-api-key>",
  "dbref": "#13364"
}
```

The server SHA-256 hashes the provided key and compares it (constant-time)
against the stored hash. On success it returns:

```json
{
  "token": "<jwt-token>"
}
```

The JWT includes `IsBot: true` for bot sessions.

### Step 2 -- Connect WebSocket

```
ws://host:8443/ws?token=<jwt-token>
```

Once connected, the bot receives game output and can send commands as the
authenticated player object.

### Key format

- 64 hexadecimal characters (32 random bytes, `crypto/rand`).
- Shown once at generation time; only the SHA-256 hash is persisted.
- Keys can be issued to both PLAYER and THING objects.

---

## Functions

### hasapikey()

```
hasapikey(<object>)
```

Returns `1` if `<object>` has an API key set, `0` otherwise. Only Players and
Things can have API keys.

```
> say hasapikey(#13364)
You say, "1"
> say hasapikey(me)
You say, "0"
```

### isapikey()

Alias for `hasapikey()`. Identical behavior.

---

## Bot Architecture Pattern

### Two-layer NPC design

| Layer | Object type | Purpose |
|-------|-------------|---------|
| Console | DARK wizard THING | Holds `$commands` for mechanical actions (commerce, rentals, repairs). Invisible to players. |
| Bot | ROBOT PLAYER | AI personality shell. Speaks and emotes via external LLM controller. No wizard access. |

**Key rules:**

- Never give wizard flags to AI-controlled bots.
- All mechanical power stays on DARK console objects.
- Bots authenticate via API key only, never interactive password.

### CONFORMAT integration

ROBOT players appear in rooms under a separate **"NPCs:"** section (cyan text).
This is controlled by functions on the Parent Room object:

| Function | Purpose |
|----------|---------|
| `FN_CF_ADDBOTS` | Finds disconnected ROBOT players via `lcon()` and adds them to the content list |
| `FN_CF_CATEGORIZE` | Routes ROBOT players to the `%qr` register |
| `FN_CF_NPCS` | Renders the "NPCs:" section from `%qr` |

NPCs are always visible in room descriptions regardless of connection status.
On the WHO list, they appear only when the external bot controller is connected.

---

## Examples

### Creating a bot from scratch

```
@botcreate Cormac
```

Save the API key from the output. Then set up the bot's description and
personality attributes:

```
@desc #13364=A weathered dockworker with calloused hands and a knowing grin.
&PERSONALITY #13364=Gruff but helpful dock master. Knows every boat in the harbor.
```

### Creating a companion console

```
@create Marina Dock Console
@set Marina Dock Console=DARK
@lock Marina Dock Console=me
&CMD_RENT Marina Dock Console=$+rent *:@pemit %#=You rent the %0.
```

### Setting up a bot controller connection

The bot controller authenticates and connects via two HTTP calls:

```bash
# 1. Exchange API key for JWT
TOKEN=$(curl -s -X POST http://localhost:8443/api/v1/auth/apikey \
  -H "Content-Type: application/json" \
  -d '{"key":"<64-char-hex>","dbref":"#13364"}' \
  | jq -r .token)

# 2. Connect WebSocket with JWT
wscat -c "ws://localhost:8443/ws?token=$TOKEN"
```

A production bot controller (TypeScript example at
`/mnt/f/mcp-servers/gotinymush-bot/`) handles reconnection, keepalive, and LLM
routing automatically. Configuration via `.env`:

```
MUSH_WS_URL=ws://localhost:8443/ws
AUTH_MODE=apikey
MUSH_DBREF=#13364
MUSH_API_KEY=<64-char-hex>
LLM_PROVIDER=groq
```

### Adding bot to CONFORMAT display

On the Parent Room (#100), ensure these attributes are set:

```
&FN_CF_ADDBOTS #100=<softcode to append disconnected ROBOT players to content>
&FIL_OFFBOT #100=<filter: true if object is ROBOT and disconnected>
&FN_CF_NPCS #100=<softcode to render NPCs: section in cyan>
```

When a player looks at a room, the CONFORMAT pipeline categorizes objects,
and ROBOT players are displayed under the "NPCs:" heading separately from
connected players and regular objects.

---

## See also

- [flags-and-powers.md](flags-and-powers.md) -- ROBOT flag, bot power
- [functions.md](functions.md) -- hasapikey(), isapikey()
- [commands.md](commands.md) -- @pcreate, @botcreate, @apikey
