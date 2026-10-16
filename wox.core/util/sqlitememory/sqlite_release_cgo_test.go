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
	if _, err := db.Exec(`INSERT INTO items (name) VALUES ('retained')`); err != nil {
		t.Fatal(err)
	}

	Register(db, "test")
	defer Unregister(db)

	// The late hidden stage may trim a connection again after background writes.
	// It must preserve both the database and per-connection configuration.
	for range 2 {
		ReleaseIdleMemory(context.Background())
		var name string
		if err := db.QueryRow(`SELECT name FROM items WHERE id = 1`).Scan(&name); err != nil || name != "retained" {
			t.Fatalf("data after trim: name=%q err=%v", name, err)
		}
		var cacheSize int
		if err := db.QueryRow(`PRAGMA cache_size`).Scan(&cacheSize); err != nil || cacheSize != -1000 {
			t.Fatalf("connection configuration after trim: cache_size=%d err=%v", cacheSize, err)
		}
	}
}
