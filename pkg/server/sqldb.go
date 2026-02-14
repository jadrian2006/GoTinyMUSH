package server

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

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
