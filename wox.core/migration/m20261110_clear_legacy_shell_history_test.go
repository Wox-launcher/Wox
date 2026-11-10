package migration

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestClearLegacyShellHistoryMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	migrate := func() {
		t.Helper()
		if err := db.Transaction(func(tx *gorm.DB) error {
			return (&clearLegacyShellHistoryMigration{}).Up(context.Background(), tx)
		}); err != nil {
			t.Fatal(err)
		}
	}

	migrate() // The Shell plugin has not created its history table on a fresh install.
	if err := db.Exec("CREATE TABLE shell_histories (id TEXT PRIMARY KEY, output_summary TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO shell_histories VALUES (?, ?), (?, ?), (?, ?)",
		"f3a20c29-e9b6-4ac4-8284-df38aef9d08d", "old output",
		"Interactive:legacy", "old output",
		"interactive:retained", "new output",
	).Error; err != nil {
		t.Fatal(err)
	}
	migrate()
	migrate()

	var rows []struct {
		ID            string
		OutputSummary string
	}
	if err := db.Table("shell_histories").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != "interactive:retained" || rows[0].OutputSummary != "new output" {
		t.Fatalf("retained history = %+v", rows)
	}
}
