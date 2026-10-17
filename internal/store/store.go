package store

import (
	"database/sql"
	"errors"
	"fmt"

	"kairo/internal/models"

	_ "modernc.org/sqlite"
)

// DB is the global database handle used by the store package.
var DB *sql.DB

// ConnectDB opens the sqlite database at dbPath and verifies the connection.
// The connection is stored in the global DB variable.
func ConnectDB(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)

	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode and foreign keys for better concurrency and integrity.
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return fmt.Errorf("exec pragma %q: %w", pragma, err)
		}
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	DB = db

	return nil
}

// CloseDB closes the global database connection and resets it to nil.
func CloseDB() error {
	if DB == nil {
		return nil
	}

	if err := DB.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	DB = nil

	return nil
}

// CreateTables creates the tables required by Kairo if they do not exist yet.
func CreateTables() error {
	if DB == nil {
		return errors.New("database not connected")
	}

	const schema = `
CREATE TABLE IF NOT EXISTS providers (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	base_url TEXT NOT NULL UNIQUE,
	api_key TEXT NOT NULL
);
`

	if _, err := DB.Exec(schema); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	return nil
}

// ProviderExistsByBaseURL reports whether a provider with the given base URL
// already exists.
func ProviderExistsByBaseURL(baseURL string) (bool, error) {
	if DB == nil {
		return false, errors.New("database not connected")
	}

	var exists bool

	err := DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM providers WHERE base_url = ?)",
		baseURL,
	).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check provider exists: %w", err)
	}

	return exists, nil
}

// CreateProvider inserts a new provider row and returns its generated id.
// The apiKey argument must already be encrypted by the caller.
func CreateProvider(name, baseURL, apiKey string) (int64, error) {
	if DB == nil {
		return 0, errors.New("database not connected")
	}

	result, err := DB.Exec(
		"INSERT INTO providers (name, base_url, api_key) VALUES (?, ?, ?)",
		name,
		baseURL,
		apiKey,
	)

	if err != nil {
		return 0, fmt.Errorf("insert provider: %w", err)
	}

	id, err := result.LastInsertId()

	if err != nil {
		return 0, fmt.Errorf("get provider id: %w", err)
	}

	return id, nil
}

// ListProviders returns all providers ordered by id ascending.
// The api key column is intentionally not selected because stored keys are
// encrypted and must never be exposed by this query.
func ListProviders() ([]models.ProviderItem, error) {
	if DB == nil {
		return nil, errors.New("database not connected")
	}

	providers := []models.ProviderItem{}

	rows, err := DB.Query("SELECT id, name, base_url FROM providers ORDER BY id")

	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var provider models.ProviderItem

		err := rows.Scan(&provider.ID, &provider.Name, &provider.BaseURL)

		if err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}

		providers = append(providers, provider)
	}

	err = rows.Err()

	if err != nil {
		return nil, fmt.Errorf("iterate providers: %w", err)
	}

	return providers, nil
}

// DeleteProvider deletes the provider with the given id and reports whether a
// row was deleted.
func DeleteProvider(id int64) (bool, error) {
	if DB == nil {
		return false, errors.New("database not connected")
	}

	result, err := DB.Exec("DELETE FROM providers WHERE id = ?", id)

	if err != nil {
		return false, fmt.Errorf("delete provider: %w", err)
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return false, fmt.Errorf("get rows affected: %w", err)
	}

	return rowsAffected > 0, nil
}
