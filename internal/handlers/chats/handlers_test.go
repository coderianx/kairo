package chats

import (
	"encoding/json"
	"fmt"
	"kairo/internal/crypto"
	"kairo/internal/store"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func setupTestDB(t *testing.T) {
	t.Helper()

	err := store.ConnectDB(filepath.Join(t.TempDir(), "test.db"))

	if err != nil {
		t.Fatalf("connect db: %v", err)
	}

	t.Cleanup(func() {
		store.CloseDB()
	})

	err = store.CreateTables()

	if err != nil {
		t.Fatalf("create tables: %v", err)
	}
}

// newFakeProvider registers a provider whose base_url points at a fake
// OpenAI-compatible server. The server records the request bodies it
// receives and replies with a fixed assistant message.
func newFakeProvider(t *testing.T, name, model string) (*httptest.Server, *[]map[string]any) {
	t.Helper()

	requests := &[]map[string]any{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}

		*requests = append(*requests, map[string]any{
			"model": body.Model,
			"count": len(body.Messages),
			"first": func() string {
				if len(body.Messages) == 0 {
					return ""
				}
				return body.Messages[0].Content
			}(),
		})

		w.Header().Set("Content-Type", "application/json")

		w.Write([]byte(`{"choices": [{"message": {"role": "assistant", "content": "assistant reply"}}]}`))
	}))

	t.Cleanup(server.Close)

	encryptedKey, err := crypto.Encrypt("fake-api-key")

	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	_, err = store.CreateProvider(name, server.URL, encryptedKey)

	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_ = model

	return server, requests
}

// getViaRouter performs a GET request through a real chi router.
func getViaRouter(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()

	router.Get("/chats", ListChats)
	router.Get("/chats/{id}/messages", GetMessages)

	req := httptest.NewRequest(http.MethodGet, path, nil)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	return rec
}

func TestCreateChat(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodPost, "/chats", strings.NewReader(`{"title": "My chat"}`))

	rec := httptest.NewRecorder()

	CreateChat(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"title":"My chat"`) {
		t.Fatalf("body must contain the title: %s", rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"created_at"`) {
		t.Fatalf("body must contain created_at: %s", rec.Body.String())
	}
}

func TestCreateChatDefaults(t *testing.T) {
	setupTestDB(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty body", "", "New chat"},
		{"empty title", `{}`, "New chat"},
		{"too long title", `{"title": "` + strings.Repeat("a", 101) + `"}`, ""},
	}

	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/chats", strings.NewReader(c.body))

		rec := httptest.NewRecorder()

		CreateChat(rec, req)

		if c.want == "" {
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s: status = %d, want %d", c.name, rec.Code, http.StatusBadRequest)
			}

			continue
		}

		if rec.Code != http.StatusCreated {
			t.Fatalf("%s: status = %d, want %d, body = %s", c.name, rec.Code, http.StatusCreated, rec.Body.String())
		}

		if !strings.Contains(rec.Body.String(), `"title":"`+c.want+`"`) {
			t.Fatalf("%s: body = %s, want title %q", c.name, rec.Body.String(), c.want)
		}
	}
}

// sendViaRouter posts a message through a real chi router so the URL param
// is populated the same way as in production.
func sendViaRouter(t *testing.T, id, body string) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()

	router.Post("/chats/{id}/messages", SendMessage)

	req := httptest.NewRequest(
		http.MethodPost,
		"/chats/"+id+"/messages",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	return rec
}

func TestSendMessage(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("ab", 32))

	setupTestDB(t)
	newFakeProvider(t, "fake", "test-model")

	// Create a chat.
	req := httptest.NewRequest(http.MethodPost, "/chats", strings.NewReader(``))

	rec := httptest.NewRecorder()

	CreateChat(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create chat status = %d", rec.Code)
	}

	rec = sendViaRouter(
		t,
		"1",
		`{"name": "fake", "model": "test-model", "content": "hi there"}`,
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"assistant reply"`) {
		t.Fatalf("body must contain the assistant reply: %s", rec.Body.String())
	}

	var count int

	err := store.DB.QueryRow(
		"SELECT COUNT(*) FROM messages WHERE chat_id = 1",
	).Scan(&count)

	if err != nil {
		t.Fatalf("count messages: %v", err)
	}

	if count != 2 {
		t.Fatalf("message count = %d, want 2", count)
	}
}

func TestSendMessageHistoryWindow(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("cd", 32))

	setupTestDB(t)
	_, requests := newFakeProvider(t, "fake2", "test-model")

	req := httptest.NewRequest(http.MethodPost, "/chats", strings.NewReader(``))

	rec := httptest.NewRecorder()

	CreateChat(rec, req)

	// Seed 20 messages in the chat.
	for i := 1; i <= 20; i++ {
		role := "assistant"

		if i%2 == 1 {
			role = "user"
		}

		_, err := store.CreateMessage(1, role, fmt.Sprintf("msg-%d", i))

		if err != nil {
			t.Fatalf("seed message %d: %v", i, err)
		}
	}

	rec = sendViaRouter(
		t,
		"1",
		`{"name": "fake2", "model": "test-model", "content": "new message"}`,
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if len(*requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(*requests))
	}

	seen := (*requests)[0]

	if seen["count"] != historyLimit {
		t.Fatalf("history count = %v, want %d", seen["count"], historyLimit)
	}

	// 21 total messages; the window must start at msg-7.
	if seen["first"] != "msg-7" {
		t.Fatalf("first history message = %v, want msg-7", seen["first"])
	}
}

func TestSendMessageValidation(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("ef", 32))

	setupTestDB(t)

	cases := []struct {
		name   string
		chatID string
		body   string
		want   int
	}{
		{"invalid chat id", "abc", `{}`, http.StatusBadRequest},
		{"unknown chat", "42", `{}`, http.StatusNotFound},
		{"missing fields", "1", `{}`, http.StatusBadRequest},
	}

	req := httptest.NewRequest(http.MethodPost, "/chats", strings.NewReader(``))

	rec := httptest.NewRecorder()

	CreateChat(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create chat status = %d", rec.Code)
	}

	for _, c := range cases {
		rec := sendViaRouter(t, c.chatID, c.body)

		if rec.Code != c.want {
			t.Fatalf("%s: status = %d, want %d, body = %s", c.name, rec.Code, c.want, rec.Body.String())
		}
	}
}

func TestListChats(t *testing.T) {
	setupTestDB(t)

	for i, title := range []string{"first", "second"} {
		req := httptest.NewRequest(
			http.MethodPost,
			"/chats",
			strings.NewReader(fmt.Sprintf(`{"title": %q}`, title)),
		)

		rec := httptest.NewRecorder()

		CreateChat(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("create chat %d: status = %d", i, rec.Code)
		}
	}

	rec := getViaRouter(t, "/chats")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Chats []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"chats"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(res.Chats) != 2 {
		t.Fatalf("chat count = %d, want 2", len(res.Chats))
	}

	// Newest first.
	if res.Chats[0].ID != 2 || res.Chats[0].Title != "second" {
		t.Fatalf("first chat = %+v, want id 2 titled second", res.Chats[0])
	}
}

func TestGetMessages(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodPost, "/chats", strings.NewReader(``))

	rec := httptest.NewRecorder()

	CreateChat(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create chat status = %d", rec.Code)
	}

	for i := 1; i <= 3; i++ {
		role := "assistant"

		if i%2 == 1 {
			role = "user"
		}

		if _, err := store.CreateMessage(1, role, fmt.Sprintf("msg-%d", i)); err != nil {
			t.Fatalf("seed message: %v", err)
		}
	}

	rec = getViaRouter(t, "/chats/1/messages")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(res.Messages) != 3 {
		t.Fatalf("message count = %d, want 3", len(res.Messages))
	}

	// Ascending order.
	if res.Messages[0].Content != "msg-1" || res.Messages[2].Content != "msg-3" {
		t.Fatalf("messages not ordered: %+v", res.Messages)
	}

	// Unknown chat.
	rec = getViaRouter(t, "/chats/99/messages")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown chat status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestSendMessageAutoTitle(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("ab", 32))

	setupTestDB(t)
	newFakeProvider(t, "fake", "test-model")

	req := httptest.NewRequest(http.MethodPost, "/chats", strings.NewReader(``))

	rec := httptest.NewRecorder()

	CreateChat(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create chat status = %d", rec.Code)
	}

	// First message replaces the default title.
	rec = sendViaRouter(
		t,
		"1",
		`{"name": "fake", "model": "test-model", "content": "merhaba dünya"}`,
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("send status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var title string

	if err := store.DB.QueryRow("SELECT title FROM chats WHERE id = 1").Scan(&title); err != nil {
		t.Fatalf("get title: %v", err)
	}

	if title != "merhaba dünya" {
		t.Fatalf("title = %q, want %q (whitespace collapsed)", title, "merhaba dünya")
	}

	// The second message must not rename the chat.
	rec = sendViaRouter(
		t,
		"1",
		`{"name": "fake", "model": "test-model", "content": "ikinci mesaj"}`,
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("second send status = %d", rec.Code)
	}

	if err := store.DB.QueryRow("SELECT title FROM chats WHERE id = 1").Scan(&title); err != nil {
		t.Fatalf("get title: %v", err)
	}

	if title != "merhaba dünya" {
		t.Fatalf("title must not change: %q", title)
	}
}

func TestSendMessageLongTitleTruncated(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("ab", 32))

	setupTestDB(t)
	newFakeProvider(t, "fake", "test-model")

	req := httptest.NewRequest(http.MethodPost, "/chats", strings.NewReader(``))

	rec := httptest.NewRecorder()

	CreateChat(rec, req)

	want := strings.Repeat("x", derivedTitleLength) + "…"
	content := strings.Repeat("x", derivedTitleLength+20)

	rec = sendViaRouter(
		t,
		"1",
		`{"name": "fake", "model": "test-model", "content": "`+content+`"}`,
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("send status = %d", rec.Code)
	}

	var title string

	if err := store.DB.QueryRow("SELECT title FROM chats WHERE id = 1").Scan(&title); err != nil {
		t.Fatalf("get title: %v", err)
	}

	if title != want {
		t.Fatalf("title = %q (len %d), want %q (len %d)", title, len([]rune(title)), want, len([]rune(want)))
	}
}

func TestSendMessageUnknownProvider(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodPost, "/chats", strings.NewReader(``))

	rec := httptest.NewRecorder()

	CreateChat(rec, req)

	rec = sendViaRouter(
		t,
		"1",
		`{"name": "missing", "model": "m", "content": "hi"}`,
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if !strings.Contains(rec.Body.String(), "provider not connected") {
		t.Fatalf("body = %s, want provider not connected", rec.Body.String())
	}
}
