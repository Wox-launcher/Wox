//go:build cgo

package sqlitememory

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestReleaseIdleMemoryShrinksRegisteredConnections(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:sqlitememory-release?mode=memory&cache=shared")
	if err != nil {
		t.Skip(err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA cache_size=-1000`); err != nil {
		t.Skip(err)
	}
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	Register(db, "test")
	defer Unregister(db)

	ReleaseIdleMemory(context.Background())
}
