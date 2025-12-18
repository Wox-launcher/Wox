# Migration (one file per migration)

Wox uses an application-level migration runner (separate from Gorm `AutoMigrate`) for **data / settings compatibility upgrades**.

## How it works

- Each migration is a Go file under `wox.core/migration/` (same package: `migration`).
- Each file registers itself via `init()` + `migration.Register(...)`.
- Migrations run in **lexicographic order** of `ID()`.
- Applied/skipped migrations are persisted in SQLite table `migration_records` (model: `database.MigrationRecord`).

## Create a new migration

1. Add a file like `wox.core/migration/mYYYYMMDD_short_name.go`
2. Implement the interface:

```go
type Migration interface {
    ID() string
    Description() string
    Up(ctx context.Context, tx *gorm.DB) error
}
```

3. Register it:

```go
func init() { Register(&myMigration{}) }
```

## Optional hooks

- If a migration only applies under certain conditions, implement:

```go
type ConditionalMigration interface {
    Migration
    IsNeeded(ctx context.Context, db *gorm.DB) (bool, error)
}
```

- If a migration needs filesystem actions **after** DB commit (e.g. rename legacy files), implement:

```go
type PostCommitMigration interface {
    Migration
    AfterCommit(ctx context.Context) error
}
```

