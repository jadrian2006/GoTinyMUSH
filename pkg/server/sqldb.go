package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	_ "modernc.org/sqlite"
)

// SQLStore manages a SQLite3 database connection for softcode SQL access.
type SQLStore struct {
	db         *sql.DB
	mu         sync.Mutex
	path       string
	queryLimit int
	timeout    time.Duration
}

// OpenSQLStore opens a SQLite3 database, sets WAL mode and busy timeout.
func OpenSQLStore(path string, queryLimit, timeoutSec int) (*SQLStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %s: %w", path, err)
	}
	// Set WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	// Set busy timeout (milliseconds)
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", timeoutSec*1000)); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}
	return &SQLStore{
		db:         db,
		path:       path,
		queryLimit: queryLimit,
		timeout:    time.Duration(timeoutSec) * time.Second,
	}, nil
}

// Close closes the SQLite3 database connection.
func (s *SQLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Path returns the filesystem path of the SQLite database.
func (s *SQLStore) Path() string { return s.path }

// Checkpoint forces a WAL checkpoint to flush all writes to the main database file.
func (s *SQLStore) Checkpoint() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("SQL NOT CONFIGURED")
	}
	_, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

// Query executes a SQL query and returns results as delimited text.
// SELECT queries return rows delimited by rowDelim with fields separated by fieldDelim.
// Non-SELECT queries return the number of affected rows.
func (s *SQLStore) Query(query, rowDelim, fieldDelim string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return "", fmt.Errorf("SQL NOT CONFIGURED")
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	trimmed := strings.TrimSpace(query)
	upper := strings.ToUpper(trimmed)

	// Non-SELECT statements (INSERT, UPDATE, DELETE, CREATE, DROP, ALTER, etc.)
	if !strings.HasPrefix(upper, "SELECT") {
		result, err := s.db.ExecContext(ctx, trimmed)
		if err != nil {
			return "", err
		}
		affected, _ := result.RowsAffected()
		return fmt.Sprintf("%d", affected), nil
	}

	// SELECT query
	rows, err := s.db.QueryContext(ctx, trimmed)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	numCols := len(cols)

	var resultRows []string
	rowCount := 0

	for rows.Next() {
		if rowCount >= s.queryLimit {
			break
		}
		// Create a slice of interface{} to scan into
		values := make([]interface{}, numCols)
		ptrs := make([]interface{}, numCols)
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", err
		}

		// Convert to strings
		fields := make([]string, numCols)
		for i, v := range values {
			if v == nil {
				fields[i] = ""
			} else {
				fields[i] = fmt.Sprintf("%v", v)
			}
		}
		resultRows = append(resultRows, strings.Join(fields, fieldDelim))
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	return strings.Join(resultRows, rowDelim), nil
}

// Escape doubles single quotes in the input string for safe SQL interpolation.
func (s *SQLStore) Escape(input string) string {
	return strings.ReplaceAll(input, "'", "''")
}

// --- Scrollback Storage ---

// InitScrollbackTables creates the scrollback tables if they don't exist.
func (s *SQLStore) InitScrollbackTables() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("SQL NOT CONFIGURED")
	}

	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS channel_scrollback (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel TEXT NOT NULL,
			sender_ref INTEGER,
			sender_name TEXT,
			message TEXT,
			timestamp INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_scrollback_time ON channel_scrollback(channel, timestamp);

		CREATE TABLE IF NOT EXISTS personal_scrollback (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			player_ref INTEGER NOT NULL,
			encrypted_data BLOB NOT NULL,
			iv BLOB NOT NULL,
			timestamp INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_personal_time ON personal_scrollback(player_ref, timestamp);
	`)
	if err != nil {
		return fmt.Errorf("creating scrollback tables: %w", err)
	}
	log.Printf("sqldb: scrollback tables initialized")
	return nil
}

// ScrollbackMessage represents a stored channel message.
type ScrollbackMessage struct {
	ID         int64  `json:"id"`
	Channel    string `json:"channel"`
	SenderRef  int    `json:"sender_ref"`
	SenderName string `json:"sender_name"`
	Message    string `json:"message"`
	Timestamp  int64  `json:"timestamp"`
}

// InsertScrollback stores a channel message.
func (s *SQLStore) InsertScrollback(channel string, senderRef gamedb.DBRef, senderName, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("SQL NOT CONFIGURED")
	}
	_, err := s.db.Exec(
		`INSERT INTO channel_scrollback (channel, sender_ref, sender_name, message, timestamp)
		 VALUES (?, ?, ?, ?, ?)`,
		channel, int(senderRef), senderName, message, time.Now().Unix(),
	)
	return err
}

// GetScrollback retrieves channel messages since a given time.
func (s *SQLStore) GetScrollback(channel string, since time.Time, limit int) ([]ScrollbackMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil, fmt.Errorf("SQL NOT CONFIGURED")
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, channel, sender_ref, sender_name, message, timestamp
		 FROM channel_scrollback WHERE channel = ? AND timestamp >= ?
		 ORDER BY timestamp ASC LIMIT ?`,
		channel, since.Unix(), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []ScrollbackMessage
	for rows.Next() {
		var m ScrollbackMessage
		if err := rows.Scan(&m.ID, &m.Channel, &m.SenderRef, &m.SenderName, &m.Message, &m.Timestamp); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// PurgeOldScrollback deletes channel scrollback entries older than the given duration.
func (s *SQLStore) PurgeOldScrollback(retention time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return 0, fmt.Errorf("SQL NOT CONFIGURED")
	}
	cutoff := time.Now().Add(-retention).Unix()
	result, err := s.db.Exec(`DELETE FROM channel_scrollback WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// PersonalScrollbackEntry represents an encrypted personal scrollback blob.
type PersonalScrollbackEntry struct {
	ID            int64  `json:"id"`
	EncryptedData []byte `json:"encrypted_data"`
	IV            []byte `json:"iv"`
	Timestamp     int64  `json:"timestamp"`
}

// InsertPersonalScrollback stores an encrypted scrollback blob for a player.
func (s *SQLStore) InsertPersonalScrollback(player gamedb.DBRef, encData, iv []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("SQL NOT CONFIGURED")
	}
	_, err := s.db.Exec(
		`INSERT INTO personal_scrollback (player_ref, encrypted_data, iv, timestamp)
		 VALUES (?, ?, ?, ?)`,
		int(player), encData, iv, time.Now().Unix(),
	)
	return err
}

// GetPersonalScrollback retrieves encrypted scrollback entries for a player since a given time.
func (s *SQLStore) GetPersonalScrollback(player gamedb.DBRef, since time.Time, limit int) ([]PersonalScrollbackEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil, fmt.Errorf("SQL NOT CONFIGURED")
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, encrypted_data, iv, timestamp
		 FROM personal_scrollback WHERE player_ref = ? AND timestamp >= ?
		 ORDER BY timestamp ASC LIMIT ?`,
		int(player), since.Unix(), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []PersonalScrollbackEntry
	for rows.Next() {
		var e PersonalScrollbackEntry
		if err := rows.Scan(&e.ID, &e.EncryptedData, &e.IV, &e.Timestamp); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// PurgeOldPersonalScrollback deletes personal scrollback entries older than the given duration.
func (s *SQLStore) PurgeOldPersonalScrollback(retention time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return 0, fmt.Errorf("SQL NOT CONFIGURED")
	}
	cutoff := time.Now().Add(-retention).Unix()
	result, err := s.db.Exec(`DELETE FROM personal_scrollback WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Reconnect closes and reopens the database connection.
func (s *SQLStore) Reconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		s.db.Close()
	}

	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		s.db = nil
		return fmt.Errorf("reconnecting sqlite %s: %w", s.path, err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		s.db = nil
		return fmt.Errorf("setting WAL mode on reconnect: %w", err)
	}
	timeoutMs := int(s.timeout.Milliseconds())
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", timeoutMs)); err != nil {
		db.Close()
		s.db = nil
		return fmt.Errorf("setting busy timeout on reconnect: %w", err)
	}

	s.db = db
	return nil
}
