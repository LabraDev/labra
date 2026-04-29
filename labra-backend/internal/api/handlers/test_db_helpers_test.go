package handlers

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openInMemorySQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func openInMemorySQLiteSingleConn(t *testing.T) *sql.DB {
	t.Helper()
	db := openInMemorySQLite(t)
	db.SetMaxOpenConns(1)
	return db
}

func applySchema(t *testing.T, db *sql.DB, schema string) {
	t.Helper()
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
}
