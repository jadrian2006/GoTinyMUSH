# Event Bus Reference

The Event Bus is a publish/subscribe system for GoTinyMUSH that enables event-driven automation. Objects subscribe attributes to named queues; when events are published, subscriber attributes are triggered with the event data.

**Compat:** Entirely new system, not present in C TinyMUSH.

## Architecture

The bus runs as an independent phase **after** the master command queue each tick. Bus handlers never re-enter the master queue -- they execute in their own phase. This prevents feedback loops between regular commands and bus events.

### Loop Protection

- **Generation counter**: Each event carries a generation number. Original publishes are generation 0. If a bus handler publishes to another queue, that event is generation 1.
- **Max depth**: Generation 2+ events are rejected. Maximum chain: publish -> handler -> publish -> handler (two hops).
- **Deferred delivery**: Events published during bus phase are queued for the next tick's bus phase, never delivered in the same tick they were published.
- **Handler budget**: Maximum 50 handlers fire per tick across all queues (round-robin fairness). Remaining handlers are deferred to the next tick.

### Scopes

| Scope | Description |
|-------|-------------|
| `global` | All subscribers receive every event (default) |
| `object` | Subscribers only receive events from a bound publisher (set via `subscribe()` bind arg) |
| `parent` | Subscribers only receive events from publishers with the same parent object |

## @queue Command

All event bus administration uses `@queue/<switch>`. Most subcommands require wizard privilege.

### Queue Lifecycle

| Command | Description |
|---------|-------------|
| `@queue/create <name>=<options>` | Create a queue (wizard). Options: `rate:N scope:type max_subs:N enabled:yes\|no` |
| `@queue/set <name>=<option>:<value>` | Modify a queue property (wizard). Options: `rate`, `max_subs`, `enabled` |
| `@queue/destroy <name>` | Delete a queue and all subscriptions (wizard) |
| `@queue/alias <alias>=<queue_name>` | Create an alternate name for a queue (wizard) |

**Queue options:**

| Option | Default | Description |
|--------|---------|-------------|
| `rate` | 1 | Tick interval. 0 = event-driven only, N = fire every N ticks |
| `scope` | global | Delivery scope: `global`, `object`, or `parent` |
| `max_subs` | 50 | Maximum subscriber count |
| `enabled` | yes | Whether the queue accepts and delivers events |

### Locks

Control who can publish, subscribe, or administrate a queue. Empty lock = wizard-only.

```
@queue/lock/pub <name>=<lock_expression>
@queue/lock/sub <name>=<lock_expression>
@queue/lock/admin <name>=<lock_expression>
```

Clear a lock by omitting the expression:

```
@queue/lock/pub <name>=
```

### Information

| Command | Description |
|---------|-------------|
| `@queue/list` | List all queues with rate, scope, subscriber count, enabled status |
| `@queue/info <name>` | Detailed queue info: rate, scope, max_subs, locks, sub/pub/pending counts |
| `@queue/subs <name>` | List all subscribers (object/attr and bind target) |
| `@queue/pubs <name>` | List all unique publisher objects |

### Statistics

| Command | Description |
|---------|-------------|
| `@queue/stats <name>` | Comprehensive stats: volume, payload sizes, handler performance, budget |
| `@queue/stats/reset <name>` | Reset counters (preserves creation time) (wizard) |
| `@queue/bus` | Global bus stats: total ticks, handlers fired, budget exhaustion |

Stats include: publish count, fire count, delivery count, drops, lock rejects, payload size (min/avg/max), handler eval time (min/avg/max), budget exhaustion count, generation 1 events, generation 2 rejections.

### Maintenance

| Command | Description |
|---------|-------------|
| `@queue/drain <name>` | Force-process pending events on next tick (wizard) |

## Functions

### publish()

Publish an event to a queue.

```
publish(<queue_name>, <data>)
```

Returns `1` on success, `#-1 ERROR` on failure (queue not found, disabled, lock rejection, generation limit).

The `<data>` argument is typically JSON but can be any string. It is passed to subscriber handlers as `%0`.

### subscribe()

Subscribe an object's attribute to a queue.

```
subscribe(<object>/<attr>, <queue_name>)
subscribe(<object>/<attr>, <queue_name>, <bind>)
```

Returns `1` on success. Requires `Controls` permission on the target object.

The optional `<bind>` argument (a dbref) is used with `scope:object` queues -- the subscription only receives events published by that specific object.

### unsubscribe()

Remove a subscription.

```
unsubscribe(<object>/<attr>, <queue_name>)
```

Returns `1` on success. Requires `Controls` permission.

### queues()

Query queue information from softcode.

```
queues()                     -> space-separated list of all queue names
queues(<name>)               -> queue info (JSON or structured)
queues(<name>, subs)         -> subscriber dbrefs
queues(<name>, stats)        -> queue statistics
```

## Examples

### Basic Event-Driven Automation

Create a queue and wire up a subscriber:

```
@queue/create weather.update=rate:60 scope:global
@queue/lock/pub weather.update=FLAG^WIZARD
@queue/lock/sub weather.update=1
```

Subscribe an object's attribute:

```
&ON_WEATHER #1234=[setq(d,[json_get(%0,description)])]@pemit/list [lcon(here)]=Weather update: [r(d)]
```

From softcode:
```
think subscribe(#1234/ON_WEATHER, weather.update)
```

Publish an event:
```
think publish(weather.update, {"temp": 72, "description": "Clear skies"})
```

### Tick-Based Timer

A queue with `rate:10` fires every 10 ticks (approximately every second at default tick rate):

```
@queue/create heartbeat=rate:10 scope:global max_subs:20
```

### Object-Scoped Events

For events that should only reach subscribers bound to a specific publisher:

```
@queue/create vehicle.move=scope:object
```

Subscribe with a bind target:
```
think subscribe(#5678/ON_MOVE, vehicle.move, #1234)
```

Now `#5678/ON_MOVE` only fires when `#1234` publishes to `vehicle.move`.

### Monitoring

Check bus health:
```
@queue/bus
@queue/stats weather.update
```

If a queue is backing up:
```
@queue/drain weather.update
```
