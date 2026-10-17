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

CREATE TABLE IF NOT EXISTS chats (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	chat_id INTEGER NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
)
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

// Sentinel errors returned by the store queries that can miss rows.
var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrChatNotFound     = errors.New("chat not found")
)

// DefaultChatTitle is the placeholder title of freshly created chats; it is
// replaced by the first user message once one arrives.
const DefaultChatTitle = "New chat"

// CreateChat inserts a new chat and returns its id and created_at timestamp.
func CreateChat(title string) (int64, string, error) {
	if DB == nil {
		return 0, "", errors.New("database not connected")
	}

	result, err := DB.Exec("INSERT INTO chats (title) VALUES (?)", title)

	if err != nil {
		return 0, "", fmt.Errorf("insert chat: %w", err)
	}

	id, err := result.LastInsertId()

	if err != nil {
		return 0, "", fmt.Errorf("get chat id: %w", err)
	}

	var createdAt string

	err = DB.QueryRow("SELECT created_at FROM chats WHERE id = ?", id).
		Scan(&createdAt)

	if err != nil {
		return 0, "", fmt.Errorf("get chat timestamp: %w", err)
	}

	return id, createdAt, nil
}

// ChatExists reports whether a chat with the given id exists.
func ChatExists(id int64) (bool, error) {
	if DB == nil {
		return false, errors.New("database not connected")
	}

	var exists bool

	err := DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM chats WHERE id = ?)",
		id,
	).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check chat exists: %w", err)
	}

	return exists, nil
}

// GetChat returns a single chat by id.
func GetChat(id int64) (models.Chat, error) {
	if DB == nil {
		return models.Chat{}, errors.New("database not connected")
	}

	var chat models.Chat

	err := DB.QueryRow(
		"SELECT id, title, created_at FROM chats WHERE id = ?",
		id,
	).Scan(&chat.ID, &chat.Title, &chat.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return models.Chat{}, ErrChatNotFound
	}

	if err != nil {
		return models.Chat{}, fmt.Errorf("get chat: %w", err)
	}

	return chat, nil
}

// RenameChat updates the title of a chat.
func RenameChat(id int64, title string) error {
	if DB == nil {
		return errors.New("database not connected")
	}

	_, err := DB.Exec("UPDATE chats SET title = ? WHERE id = ?", title, id)

	if err != nil {
		return fmt.Errorf("rename chat: %w", err)
	}

	return nil
}

// ListChats returns all chats, newest first.
func ListChats() ([]models.Chat, error) {
	if DB == nil {
		return nil, errors.New("database not connected")
	}

	rows, err := DB.Query("SELECT id, title, created_at FROM chats ORDER BY id DESC")

	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}

	defer rows.Close()

	chats := []models.Chat{}

	for rows.Next() {
		var chat models.Chat

		err := rows.Scan(&chat.ID, &chat.Title, &chat.CreatedAt)

		if err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}

		chats = append(chats, chat)
	}

	err = rows.Err()

	if err != nil {
		return nil, fmt.Errorf("iterate chats: %w", err)
	}

	return chats, nil
}

// ListMessages returns every message of a chat ordered oldest first.
func ListMessages(chatID int64) ([]models.Message, error) {
	if DB == nil {
		return nil, errors.New("database not connected")
	}

	rows, err := DB.Query(
		`SELECT id, chat_id, role, content
		FROM messages
		WHERE chat_id = ?
		ORDER BY id ASC`,
		chatID,
	)

	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	defer rows.Close()

	messages := []models.Message{}

	for rows.Next() {
		var message models.Message

		err := rows.Scan(&message.ID, &message.ChatID, &message.Role, &message.Content)

		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}

		messages = append(messages, message)
	}

	err = rows.Err()

	if err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	return messages, nil
}

// GetProviderByName returns the base URL and the encrypted api key of the
// provider with the given name. The caller is responsible for decrypting the
// api key.
func GetProviderByName(name string) (baseURL, apiKey string, err error) {
	if DB == nil {
		return "", "", errors.New("database not connected")
	}

	err = DB.QueryRow(
		"SELECT base_url, api_key FROM providers WHERE name = ?",
		name,
	).Scan(&baseURL, &apiKey)

	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrProviderNotFound
	}

	if err != nil {
		return "", "", fmt.Errorf("get provider: %w", err)
	}

	return baseURL, apiKey, nil
}

// CreateMessage inserts a message into a chat and returns its generated id.
func CreateMessage(chatID int64, role, content string) (int64, error) {
	if DB == nil {
		return 0, errors.New("database not connected")
	}

	result, err := DB.Exec(
		"INSERT INTO messages (chat_id, role, content) VALUES (?, ?, ?)",
		chatID,
		role,
		content,
	)

	if err != nil {
		return 0, fmt.Errorf("insert message: %w", err)
	}

	id, err := result.LastInsertId()

	if err != nil {
		return 0, fmt.Errorf("get message id: %w", err)
	}

	return id, nil
}

// GetLastMessages returns the most recent messages of a chat, oldest first.
func GetLastMessages(chatID int64, limit int) ([]models.Message, error) {
	if DB == nil {
		return nil, errors.New("database not connected")
	}

	rows, err := DB.Query(
		`SELECT id, chat_id, role, content
		FROM messages
		WHERE chat_id = ?
		ORDER BY id DESC
		LIMIT ?`,
		chatID,
		limit,
	)

	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	defer rows.Close()

	messages := []models.Message{}

	for rows.Next() {
		var message models.Message

		err := rows.Scan(&message.ID, &message.ChatID, &message.Role, &message.Content)

		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}

		messages = append(messages, message)
	}

	err = rows.Err()

	if err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	// The query returns newest first; reverse so the history is ordered
	// oldest first for the model.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
