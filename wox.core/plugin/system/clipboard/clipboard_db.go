package system

import (
	"context"
	"database/sql"
	"fmt"
	"path"
	"strings"
	"time"
	"wox/util"
	"wox/util/clipboard"

	_ "github.com/mattn/go-sqlite3"
)

// ClipboardDB handles all database operations for clipboard history
type ClipboardDB struct {
	db *sql.DB
}

// ClipboardRecord represents a clipboard history record in the database
type ClipboardRecord struct {
	ID         string
	Type       string
	Content    string  // For text content or metadata
	FilePath   string  // For image files
	IconData   *string // For storing icon data (base64 or file path), nullable
	Width      *int    // For image width, nullable
	Height     *int    // For image height, nullable
	FileSize   *int64  // For file size in bytes, nullable
	Alias      *string // For user-defined alias, nullable
	Timestamp  int64
	IsFavorite bool
	CreatedAt  time.Time
}

// NewClipboardDB creates a new clipboard database instance
func NewClipboardDB(ctx context.Context, pluginId string) (*ClipboardDB, error) {
	dbPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), pluginId+"_clipboard.db")

	// Configure SQLite with proper concurrency settings
	dsn := dbPath + "?" +
		"_journal_mode=DELETE&" + // Enable WAL mode for better concurrency
		"_synchronous=FULL&" + // Balance between safety and performance
		"_cache_size=1000&" + // Set cache size
		"_foreign_keys=true&" + // Enable foreign key constraints
		"_busy_timeout=5000" // Set busy timeout to 5 seconds

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings for better concurrency
	db.SetMaxOpenConns(10)           // Maximum number of open connections
	db.SetMaxIdleConns(5)            // Maximum number of idle connections
	db.SetConnMaxLifetime(time.Hour) // Maximum lifetime of a connection

	// Execute additional PRAGMA statements for optimal concurrency
	pragmas := []string{
		"PRAGMA journal_mode=DELETE", // Ensure WAL mode is enabled
		"PRAGMA synchronous=FULL",    // Balance safety and performance
		"PRAGMA cache_size=1000",     // Set cache size
		"PRAGMA foreign_keys=ON",     // Enable foreign key constraints
		"PRAGMA temp_store=memory",   // Store temporary tables in memory
		"PRAGMA mmap_size=268435456", // Set memory-mapped I/O size (256MB)
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to execute pragma %s: %v", pragma, err))
		}
	}

	clipboardDB := &ClipboardDB{db: db}
	if err := clipboardDB.initTables(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("clipboard database initialized at %s with WAL mode enabled", dbPath))
	return clipboardDB, nil
}

// initTables creates the necessary tables if they don't exist
func (c *ClipboardDB) initTables(ctx context.Context) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS clipboard_history (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		content TEXT,
		file_path TEXT,
		icon_data TEXT,
		width INTEGER,
		height INTEGER,
		file_size INTEGER,
		timestamp INTEGER NOT NULL,
		is_favorite BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_timestamp ON clipboard_history(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_favorite ON clipboard_history(is_favorite);
	CREATE INDEX IF NOT EXISTS idx_type ON clipboard_history(type);
	CREATE INDEX IF NOT EXISTS idx_content ON clipboard_history(content);
	`

	_, err := c.db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return err
	}

	// Add new columns if they don't exist (for migration from older versions)
	alterTableSQLs := []string{
		`ALTER TABLE clipboard_history ADD COLUMN icon_data TEXT`,
		`ALTER TABLE clipboard_history ADD COLUMN width INTEGER`,
		`ALTER TABLE clipboard_history ADD COLUMN height INTEGER`,
		`ALTER TABLE clipboard_history ADD COLUMN file_size INTEGER`,
		`ALTER TABLE clipboard_history ADD COLUMN alias TEXT`,
	}

	for _, alterSQL := range alterTableSQLs {
		_, alterErr := c.db.ExecContext(ctx, alterSQL)
		// Ignore error if column already exists
		if alterErr != nil && !strings.Contains(alterErr.Error(), "duplicate column name") {
			// Log the error but don't fail initialization
			util.GetLogger().Info(ctx, fmt.Sprintf("Failed to add column (likely already exists): %s", alterErr.Error()))
		}
	}

	return nil
}

// Insert adds a new clipboard record to the database
func (c *ClipboardDB) Insert(ctx context.Context, record ClipboardRecord) error {
	insertSQL := `
	INSERT INTO clipboard_history (id, type, content, file_path, icon_data, width, height, file_size, alias, timestamp, is_favorite, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := c.db.ExecContext(ctx, insertSQL,
		record.ID, record.Type, record.Content, record.FilePath, record.IconData,
		record.Width, record.Height, record.FileSize, record.Alias,
		record.Timestamp, record.IsFavorite, record.CreatedAt)

	return err
}

// Update modifies an existing clipboard record
func (c *ClipboardDB) Update(ctx context.Context, record ClipboardRecord) error {
	updateSQL := `
	UPDATE clipboard_history
	SET type = ?, content = ?, file_path = ?, icon_data = ?, width = ?, height = ?, file_size = ?, alias = ?, timestamp = ?, is_favorite = ?
	WHERE id = ?
	`

	_, err := c.db.ExecContext(ctx, updateSQL,
		record.Type, record.Content, record.FilePath, record.IconData,
		record.Width, record.Height, record.FileSize, record.Alias,
		record.Timestamp, record.IsFavorite, record.ID)

	return err
}

// UpdateTimestamp updates the timestamp of a record (for moving to top)
func (c *ClipboardDB) UpdateTimestamp(ctx context.Context, id string, timestamp int64) error {
	updateSQL := `UPDATE clipboard_history SET timestamp = ? WHERE id = ?`
	_, err := c.db.ExecContext(ctx, updateSQL, timestamp, id)
	return err
}

// UpdateContent updates the content of a record
func (c *ClipboardDB) UpdateContent(ctx context.Context, id string, content string) error {
	updateSQL := `UPDATE clipboard_history SET content = ? WHERE id = ?`
	_, err := c.db.ExecContext(ctx, updateSQL, content, id)
	return err
}

// UpdateAlias updates the alias of a record
func (c *ClipboardDB) UpdateAlias(ctx context.Context, id string, alias *string) error {
	updateSQL := `UPDATE clipboard_history SET alias = ? WHERE id = ?`
	_, err := c.db.ExecContext(ctx, updateSQL, alias, id)
	return err
}

// Delete removes a record by ID
func (c *ClipboardDB) Delete(ctx context.Context, id string) error {
	deleteSQL := `DELETE FROM clipboard_history WHERE id = ?`
	_, err := c.db.ExecContext(ctx, deleteSQL, id)
	return err
}

// GetRecent retrieves recent clipboard records with pagination
func (c *ClipboardDB) GetRecent(ctx context.Context, limit, offset int) ([]ClipboardRecord, error) {
	querySQL := `
	SELECT id, type, content, file_path, icon_data, width, height, file_size, alias, timestamp, is_favorite, created_at
	FROM clipboard_history
	ORDER BY timestamp DESC
	LIMIT ? OFFSET ?
	`

	rows, err := c.db.QueryContext(ctx, querySQL, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return c.scanRecords(rows)
}

// SearchText searches for text content in clipboard history
func (c *ClipboardDB) SearchText(ctx context.Context, searchTerm string, limit int) ([]ClipboardRecord, error) {
	querySQL := `
	SELECT id, type, content, file_path, icon_data, width, height, file_size, alias, timestamp, is_favorite, created_at
	FROM clipboard_history
	WHERE type = ? AND (content LIKE ? OR alias LIKE ?)
	ORDER BY timestamp DESC
	LIMIT ?
	`

	searchPattern := "%" + searchTerm + "%"
	rows, err := c.db.QueryContext(ctx, querySQL, string(clipboard.ClipboardTypeText), searchPattern, searchPattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return c.scanRecords(rows)
}

// GetByID retrieves a specific record by ID
func (c *ClipboardDB) GetByID(ctx context.Context, id string) (*ClipboardRecord, error) {
	querySQL := `
	SELECT id, type, content, file_path, alias, timestamp, is_favorite, created_at
	FROM clipboard_history
	WHERE id = ?
	`

	row := c.db.QueryRowContext(ctx, querySQL, id)
	record := &ClipboardRecord{}

	err := row.Scan(&record.ID, &record.Type, &record.Content,
		&record.FilePath, &record.Alias, &record.Timestamp, &record.IsFavorite, &record.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return record, nil
}

// DeleteExpired removes records older than the specified days
func (c *ClipboardDB) DeleteExpired(ctx context.Context, textDays, imageDays int) (int64, error) {
	currentTime := util.GetSystemTimestamp()
	textCutoff := currentTime - int64(textDays)*24*60*60*1000
	imageCutoff := currentTime - int64(imageDays)*24*60*60*1000

	deleteSQL := `
	DELETE FROM clipboard_history 
	WHERE is_favorite = FALSE AND (
		(type = ? AND timestamp < ?) OR
		(type = ? AND timestamp < ?)
	)
	`

	result, err := c.db.ExecContext(ctx, deleteSQL,
		string(clipboard.ClipboardTypeText), textCutoff,
		string(clipboard.ClipboardTypeImage), imageCutoff)

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// EnforceMaxCount ensures the total number of records doesn't exceed maxCount
func (c *ClipboardDB) EnforceMaxCount(ctx context.Context, maxCount int) (int64, error) {
	// First, count total records
	countSQL := `SELECT COUNT(*) FROM clipboard_history`
	var totalCount int
	err := c.db.QueryRowContext(ctx, countSQL).Scan(&totalCount)
	if err != nil {
		return 0, err
	}

	if totalCount <= maxCount {
		return 0, nil // No need to delete anything
	}

	// Delete oldest non-favorite records
	deleteSQL := `
	DELETE FROM clipboard_history 
	WHERE id IN (
		SELECT id FROM clipboard_history 
		WHERE is_favorite = FALSE 
		ORDER BY timestamp ASC 
		LIMIT ?
	)
	`

	deleteCount := totalCount - maxCount
	result, err := c.db.ExecContext(ctx, deleteSQL, deleteCount)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetStats returns statistics about the clipboard database
func (c *ClipboardDB) GetStats(ctx context.Context) (map[string]int, error) {
	stats := make(map[string]int)

	// Total count
	var total int
	err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clipboard_history`).Scan(&total)
	if err != nil {
		return nil, err
	}
	stats["total"] = total

	// Favorite count
	var favorites int
	err = c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clipboard_history WHERE is_favorite = TRUE`).Scan(&favorites)
	if err != nil {
		return nil, err
	}
	stats["favorites"] = favorites

	// Text count
	var textCount int
	err = c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clipboard_history WHERE type = ?`, string(clipboard.ClipboardTypeText)).Scan(&textCount)
	if err != nil {
		return nil, err
	}
	stats["text"] = textCount

	// Image count
	var imageCount int
	err = c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clipboard_history WHERE type = ?`, string(clipboard.ClipboardTypeImage)).Scan(&imageCount)
	if err != nil {
		return nil, err
	}
	stats["images"] = imageCount

	return stats, nil
}

// Close closes the database connection
func (c *ClipboardDB) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// ClipboardHistory represents the old clipboard history structure from plugin settings
type ClipboardHistory struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Type       string `json:"type"`
	Timestamp  int64  `json:"timestamp"`
	ImagePath  string `json:"imagePath,omitempty"`
	IsFavorite bool   `json:"isFavorite,omitempty"`
}

// scanRecords is a helper function to scan multiple records from query results
func (c *ClipboardDB) scanRecords(rows *sql.Rows) ([]ClipboardRecord, error) {
	var records []ClipboardRecord

	for rows.Next() {
		var record ClipboardRecord
		err := rows.Scan(&record.ID, &record.Type, &record.Content,
			&record.FilePath, &record.IconData, &record.Width, &record.Height, &record.FileSize, &record.Alias,
			&record.Timestamp, &record.IsFavorite, &record.CreatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}
