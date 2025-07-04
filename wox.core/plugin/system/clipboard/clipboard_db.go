package system

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"wox/util"
	"wox/util/clipboard"

	"github.com/disintegration/imaging"
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
	Timestamp  int64
	IsFavorite bool
	CreatedAt  time.Time
}

// NewClipboardDB creates a new clipboard database instance
func NewClipboardDB(ctx context.Context, pluginId string) (*ClipboardDB, error) {
	dbPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), pluginId+"_clipboard.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	clipboardDB := &ClipboardDB{db: db}
	if err := clipboardDB.initTables(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

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
	INSERT INTO clipboard_history (id, type, content, file_path, icon_data, width, height, file_size, timestamp, is_favorite, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := c.db.ExecContext(ctx, insertSQL,
		record.ID, record.Type, record.Content, record.FilePath, record.IconData,
		record.Width, record.Height, record.FileSize,
		record.Timestamp, record.IsFavorite, record.CreatedAt)

	return err
}

// Update modifies an existing clipboard record
func (c *ClipboardDB) Update(ctx context.Context, record ClipboardRecord) error {
	updateSQL := `
	UPDATE clipboard_history
	SET type = ?, content = ?, file_path = ?, icon_data = ?, width = ?, height = ?, file_size = ?, timestamp = ?, is_favorite = ?
	WHERE id = ?
	`

	_, err := c.db.ExecContext(ctx, updateSQL,
		record.Type, record.Content, record.FilePath, record.IconData,
		record.Width, record.Height, record.FileSize,
		record.Timestamp, record.IsFavorite, record.ID)

	return err
}

// SetFavorite updates the favorite status of a record
func (c *ClipboardDB) SetFavorite(ctx context.Context, id string, isFavorite bool) error {
	updateSQL := `UPDATE clipboard_history SET is_favorite = ? WHERE id = ?`
	_, err := c.db.ExecContext(ctx, updateSQL, isFavorite, id)
	return err
}

// UpdateTimestamp updates the timestamp of a record (for moving to top)
func (c *ClipboardDB) UpdateTimestamp(ctx context.Context, id string, timestamp int64) error {
	updateSQL := `UPDATE clipboard_history SET timestamp = ? WHERE id = ?`
	_, err := c.db.ExecContext(ctx, updateSQL, timestamp, id)
	return err
}

// GetRecent retrieves recent clipboard records with pagination
func (c *ClipboardDB) GetRecent(ctx context.Context, limit, offset int) ([]ClipboardRecord, error) {
	querySQL := `
	SELECT id, type, content, file_path, icon_data, width, height, file_size, timestamp, is_favorite, created_at
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

// GetFavorites retrieves all favorite records
func (c *ClipboardDB) GetFavorites(ctx context.Context) ([]ClipboardRecord, error) {
	querySQL := `
	SELECT id, type, content, file_path, icon_data, width, height, file_size, timestamp, is_favorite, created_at
	FROM clipboard_history
	WHERE is_favorite = TRUE
	ORDER BY timestamp DESC
	`

	rows, err := c.db.QueryContext(ctx, querySQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return c.scanRecords(rows)
}

// SearchText searches for text content in clipboard history
func (c *ClipboardDB) SearchText(ctx context.Context, searchTerm string, limit int) ([]ClipboardRecord, error) {
	querySQL := `
	SELECT id, type, content, file_path, icon_data, width, height, file_size, timestamp, is_favorite, created_at
	FROM clipboard_history
	WHERE type = ? AND content LIKE ?
	ORDER BY timestamp DESC
	LIMIT ?
	`

	rows, err := c.db.QueryContext(ctx, querySQL, string(clipboard.ClipboardTypeText), "%"+searchTerm+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return c.scanRecords(rows)
}

// GetByID retrieves a specific record by ID
func (c *ClipboardDB) GetByID(ctx context.Context, id string) (*ClipboardRecord, error) {
	querySQL := `
	SELECT id, type, content, file_path, timestamp, is_favorite, created_at
	FROM clipboard_history
	WHERE id = ?
	`

	row := c.db.QueryRowContext(ctx, querySQL, id)
	record := &ClipboardRecord{}

	err := row.Scan(&record.ID, &record.Type, &record.Content,
		&record.FilePath, &record.Timestamp, &record.IsFavorite, &record.CreatedAt)

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

// migrateLegacyItem migrates a single legacy clipboard item to the database
func (db *ClipboardDB) migrateLegacyItem(ctx context.Context, item ClipboardHistory) error {
	// Check if item already exists
	exists, err := db.itemExists(ctx, item.ID)
	if err != nil {
		return fmt.Errorf("failed to check if item exists: %w", err)
	}
	if exists {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Item %s already exists, skipping", item.ID))
		return nil
	}

	// Convert legacy type to new type
	var itemType clipboard.Type
	switch item.Type {
	case "text":
		itemType = clipboard.ClipboardTypeText
	case "image":
		itemType = clipboard.ClipboardTypeImage
	default:
		itemType = clipboard.ClipboardTypeText
	}

	// Handle image migration
	var imagePath string
	if itemType == clipboard.ClipboardTypeImage && item.ImagePath != "" {
		// Check if old image file exists
		if _, err := os.Stat(item.ImagePath); err == nil {
			// Copy image to new location
			newImagePath, err := db.copyImageToNewLocation(ctx, item.ImagePath, item.ID)
			if err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to copy image for item %s: %v", item.ID, err))
				// Continue with text content if image copy fails
				itemType = clipboard.ClipboardTypeText
			} else {
				imagePath = newImagePath
			}
		} else {
			util.GetLogger().Warn(ctx, fmt.Sprintf("Legacy image file not found: %s", item.ImagePath))
			// Continue with text content if image file is missing
			itemType = clipboard.ClipboardTypeText
		}
	}

	// Insert into database with is_favorite set to true (since we only migrate favorites)
	query := `INSERT INTO clipboard_history (id, content, type, file_path, timestamp, is_favorite) VALUES (?, ?, ?, ?, ?, ?)`
	_, err = db.db.ExecContext(ctx, query, item.ID, item.Text, string(itemType), imagePath, item.Timestamp, true)
	if err != nil {
		return fmt.Errorf("failed to insert migrated item: %w", err)
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("Successfully migrated item %s", item.ID))
	return nil
}

// itemExists checks if an item with the given ID already exists in the database
func (db *ClipboardDB) itemExists(ctx context.Context, id string) (bool, error) {
	query := `SELECT COUNT(*) FROM clipboard_history WHERE id = ?`
	var count int
	err := db.db.QueryRowContext(ctx, query, id).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// copyImageToNewLocation copies an image from the old location to the new temp directory structure
func (db *ClipboardDB) copyImageToNewLocation(ctx context.Context, oldPath, itemID string) (string, error) {
	// Create temp directory if it doesn't exist
	tempDir := path.Join(os.TempDir(), "wox_clipboard")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate new filename
	newFilename := fmt.Sprintf("%s.png", itemID)
	newPath := path.Join(tempDir, newFilename)

	// Open source image
	srcImg, err := imaging.Open(oldPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source image: %w", err)
	}

	// Save to new location
	if err := imaging.Save(srcImg, newPath); err != nil {
		return "", fmt.Errorf("failed to save image to new location: %w", err)
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("Copied image from %s to %s", oldPath, newPath))
	return newPath, nil
}

// scanRecords is a helper function to scan multiple records from query results
func (c *ClipboardDB) scanRecords(rows *sql.Rows) ([]ClipboardRecord, error) {
	var records []ClipboardRecord

	for rows.Next() {
		var record ClipboardRecord
		err := rows.Scan(&record.ID, &record.Type, &record.Content,
			&record.FilePath, &record.IconData, &record.Width, &record.Height, &record.FileSize,
			&record.Timestamp, &record.IsFavorite, &record.CreatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}
