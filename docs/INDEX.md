# GoTinyMUSH Reference Documentation

> Complete reference for GoTinyMUSH — a Go reimplementation of TinyMUSH 3.3
>
> **TinyMUSH Compatibility**: 696/696 function tests pass (100%), 164/164 command tests pass
> **Version**: GoTinyMUSH (Go) — based on TinyMUSH 3.3 specification

## Quick Navigation

### Core Reference
- [Commands Reference](commands.md) — All @ commands, building, admin, movement
- [Functions Reference](functions.md) — All 556+ softcode functions by category
- [Flags & Powers](flags-and-powers.md) — Object flags, attribute flags, powers
- [Locks](locks.md) — Lock types, evaluation locks, indirect locks
- [Substitutions](substitutions.md) — %-substitutions and #-substitutions

### Subsystems
- [Mail System](mail.md) — @mail switches, @malias, mail functions
- [Channel System (Comsys)](comsys.md) — Channels, @clist, addcom, channel flags
- [Event Bus](eventbus.md) — Pub/sub queues, @queue management
- [SQL Integration](SQL.md) — SQLite3 embedded database
- [Navigation System](FLIGHT.md) — Grid coordinates, topology, weather, POI
- [Bot System](bots.md) — @botcreate, API keys, NPC architecture
- [Array System](arrays.md) — Per-player mutable arrays
- [Structure/Instance System](structs.md) — Typed data structures

### New & Extended Features
- [New Features](FEATURES.md) — Features beyond TinyMUSH 3.3 baseline
- [New Functions](new-functions.md) — Functions added or extended beyond C TinyMUSH
- [Behavioral Changes](behavioral-changes.md) — Differences from C TinyMUSH behavior

### Administration
- [Configuration](configuration.md) — YAML config, environment variables, CLI flags
- [Building Guide](building.md) — Compiling from source, Docker deployment
- [Migration Guide](migration.md) — Migrating from C TinyMUSH

### Compatibility
- [TinyMUSH Compatibility Report](compatibility.md) — Full audit results, test coverage

---

## Documentation Conventions

- **Syntax**: `<required>` `[optional]` `<opt1|opt2>`
- **Permissions**: Each entry notes required permissions (any, builder, wizard, god)
- **Compatibility**: ✅ = tested compatible, ⚠️ = behavioral difference, 🆕 = new in Go
- **Examples**: All entries include at least one usage example
