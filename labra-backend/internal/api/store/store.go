package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrNotFound is what we return when a db lookup finds nothing
var ErrNotFound = errors.New("not found")

// Store wraps the database connection - all db access goes through here
type Store struct {
	db *sql.DB
}

// New creates a new store from an existing db connection
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// nullIfEmpty converts an empty string to nil for nullable db columns
// saves us from storing empty strings where null would be more appropriate
func nullIfEmpty(stringValue string) any {
	if strings.TrimSpace(stringValue) == "" {
		return nil
	}
	return stringValue
}

// boolToInt converts a bool to sqlite's 0/1 integer representation
// sqlite doesn't have a native bool type
func boolToInt(boolValue bool) int {
	if boolValue {
		return 1
	}
	return 0
}

// UnixNow returns the current unix timestamp - used for created_at/updated_at columns
func UnixNow() int64 {
	return time.Now().Unix()
}
