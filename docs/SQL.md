# SQL Module

GoTinyMUSH includes a built-in SQLite3 database for softcode SQL access. This replaces the external SQL database module from TinyMUSH C with an embedded, zero-configuration SQLite3 engine (via `modernc.org/sqlite`, pure Go, no CGo).

## Setup

### Config File (YAML)

```yaml
sql_enabled: true
sql_database: /path/to/game.sqlite3
sql_query_limit: 100
sql_timeout: 5
sql_reconnect: true
```

### Environment Variables

```bash
export MUSH_SQL=true
export MUSH_SQLDB=/path/to/game.sqlite3
```

### Command-Line Flags

```bash
gotinymush -sqldb /path/to/game.sqlite3 ...
```

The `-sqldb` flag and `MUSH_SQLDB` env override the config file's `sql_database`. `MUSH_SQL=true` or `sql_enabled: true` in config enables the module.

## Configuration Reference

| Parameter | Default | Description |
|---|---|---|
| `sql_enabled` | `false` | Master enable/disable for SQL module |
| `sql_database` | `""` | Path to SQLite3 database file |
| `sql_query_limit` | `100` | Maximum rows returned from a SELECT query |
| `sql_timeout` | `5` | Query timeout in seconds |
| `sql_reconnect` | `true` | Auto-reconnect on failure |

## Permissions

SQL access requires the `use_sql` power, which can only be granted by God:

```
@power player=use_sql
```

Wizards do **not** automatically get SQL access. Only players with the `use_sql` power or God (#1) can use the `sql()` function.

To revoke access:
```
@power player=!use_sql
```

## Softcode Functions

### sql(\<query\>[, \<row delim\>[, \<field delim\>]])

Executes a SQL query against the SQLite3 database.

- **SELECT** queries return results as delimited text
- **Non-SELECT** queries (INSERT, UPDATE, DELETE, CREATE, etc.) return the number of affected rows
- Default row delimiter: space
- Default field delimiter: same as row delimiter

Returns `#-1 SQL NOT CONFIGURED` if SQL is disabled, `#-1 PERMISSION DENIED` if the player lacks the `use_sql` power.

**Examples:**
```
> say [sql(SELECT name\, score FROM players)]
You say "Alice 100 Bob 85"

> say [sql(SELECT name\, score FROM players, |, /)]
You say "Alice/100|Bob/85"

> say [sql(INSERT INTO log VALUES('event'\, secs()))]
You say "1"
```

### sqlescape(\<string\>)

Doubles single quotes in a string for safe SQL interpolation.

**Example:**
```
> say [sqlescape(O'Brien)]
You say "O''Brien"

> @sql INSERT INTO players (name) VALUES('[sqlescape(%n)]')
```

## Admin Commands

### @sql \<query\>

**Wizard-only.** Executes a SQL query interactively. SELECT results display in "Row N, Field N: value" format. Non-SELECT shows affected row count.

```
> @sql CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)
SQL query touched 0 row(s).

> @sql INSERT INTO test VALUES(1, 'Hello')
SQL query touched 1 row(s).

> @sql SELECT * FROM test
Row 1, Field 1: 1
Row 1, Field 2: Hello
1 row(s) returned.
```

### @sqlinit

**God-only.** Re-opens the SQLite3 database connection.

### @sqldisconnect

**God-only.** Closes the SQLite3 database connection. After disconnecting, `sql()` calls return `#-1 SQL NOT CONFIGURED`.

## Example Use Cases

### Player Statistics / Leaderboard

Track kills, deaths, scores in a table; query top-10 with `sql(SELECT ...)`:

```
&CMD_SCORE obj=$+score:think [sql(INSERT OR REPLACE INTO scores (dbref\,name\,kills) VALUES(%#\,'[sqlescape(%n)]'\,[add(sql(SELECT kills FROM scores WHERE dbref=%#)\,1)]))]
&CMD_TOP10 obj=$+top10:@pemit %#=[sql(SELECT name\,kills FROM scores ORDER BY kills DESC LIMIT 10,|,%b)]
```

### Web Portal Integration

A shared SQLite DB readable by a web app for character sheets, forums, or maps.

### Event Logging / Audit Trail

Log IC events to SQL for admin review or RP history:

```
&LOG_EVENT obj=$+log *:think [sql(INSERT INTO events (time\,player\,event) VALUES([secs()]\,'[sqlescape(%n)]'\,'[sqlescape(%0)]'))]
```

### Economy / Market System

Item listings, bids, transactions in SQL with proper indexing; far more efficient than attribute iteration for large datasets.

### BBS / Mail Archive

Store message boards in SQL for efficient search, pagination, and web access.

## Safety Notes

- **Row limit** prevents runaway queries (default 100 rows max)
- **Timeout** prevents long-running locks (default 5 seconds)
- **SQLite is file-based** -- no network exposure, no separate server
- **WAL mode** allows concurrent reads while writing
- **Mutex serialization** prevents concurrent write conflicts
- **use_sql power** restricts access to authorized players only
