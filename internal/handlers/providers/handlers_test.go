package providers

import (
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

func TestConnectNewProvider(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("ab", 32))

	setupTestDB(t)

	body := `{"name": "groq", "api_key": "gsk_test_secret_key"}`

	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))

	rec := httptest.NewRecorder()

	ConnectNewProvider(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"id":1`) {
		t.Fatalf("response body must contain the provider id: %s", rec.Body.String())
	}

	if strings.Contains(rec.Body.String(), "gsk_test_secret_key") {
		t.Fatal("response body must not contain the api key")
	}

	var (
		apiKey string
		name   string
	)

	err := store.DB.QueryRow("SELECT name, api_key FROM providers WHERE id = 1").Scan(&name, &apiKey)

	if err != nil {
		t.Fatalf("query provider: %v", err)
	}

	if name != "groq" {
		t.Fatalf("name = %q, want %q", name, "groq")
	}

	if apiKey == "gsk_test_secret_key" {
		t.Fatal("api_key must be stored encrypted")
	}

	decrypted, err := crypto.Decrypt(apiKey)

	if err != nil {
		t.Fatalf("decrypt stored api key: %v", err)
	}

	if decrypted != "gsk_test_secret_key" {
		t.Fatalf("decrypted api key = %q, want %q", decrypted, "gsk_test_secret_key")
	}
}

// modelsViaRouter performs a GET models request through a real chi router.
func modelsViaRouter(t *testing.T, name string) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()

	router.Get("/providers/{name}/models", ListModels)

	req := httptest.NewRequest(
		http.MethodGet,
		"/providers/"+name+"/models",
		nil,
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	return rec
}

func TestListModels(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("ab", 32))

	setupTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %q, want /models", r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer fake-api-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}

		w.Write([]byte(`{"data": [{"id": "model-a"}, {"id": "model-b"}]}`))
	}))

	defer server.Close()

	encryptedKey, err := crypto.Encrypt("fake-api-key")

	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	if _, err = store.CreateProvider("fake", server.URL, encryptedKey); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	rec := modelsViaRouter(t, "fake")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"models":["model-a","model-b"]`) {
		t.Fatalf("body = %s", rec.Body.String())
	}

	// Unknown provider.
	rec = modelsViaRouter(t, "missing")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown provider status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestConnectNewProviderDuplicateBaseURL(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("cd", 32))

	setupTestDB(t)

	body := `{"name": "groq", "api_key": "gsk_first_key"}`

	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))

	rec := httptest.NewRecorder()

	ConnectNewProvider(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("first connect status = %d, want %d", rec.Code, http.StatusCreated)
	}

	req = httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))

	rec = httptest.NewRecorder()

	ConnectNewProvider(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate connect status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestConnectNewProviderValidation(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("ef", 32))

	setupTestDB(t)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"invalid json", "{", http.StatusBadRequest},
		{"missing name", `{"api_key": "key"}`, http.StatusBadRequest},
		{"name too long", `{"name": "` + strings.Repeat("a", 51) + `", "api_key": "key"}`, http.StatusBadRequest},
		{"missing base_url", `{"name": "unknown", "api_key": "key"}`, http.StatusBadRequest},
		{"missing api_key", `{"name": "groq"}`, http.StatusBadRequest},
	}

	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(c.body))

		rec := httptest.NewRecorder()

		ConnectNewProvider(rec, req)

		if rec.Code != c.want {
			t.Fatalf("%s: status = %d, want %d", c.name, rec.Code, c.want)
		}
	}
}

func TestListMyProviders(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("12", 32))

	setupTestDB(t)

	bodies := []string{
		`{"name": "groq", "api_key": "gsk_first_key"}`,
		`{"name": "openrouter", "api_key": "sk-or_second_key"}`,
	}

	for _, body := range bodies {
		req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))

		rec := httptest.NewRecorder()

		ConnectNewProvider(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("connect status = %d, want %d", rec.Code, http.StatusCreated)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/providers", nil)

	rec := httptest.NewRecorder()

	ListMyProviders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	response := rec.Body.String()

	if !strings.Contains(response, `"providers":[`) {
		t.Fatalf("response body must contain the providers array: %s", response)
	}

	if !strings.Contains(response, `"name":"groq"`) || !strings.Contains(response, `"name":"openrouter"`) {
		t.Fatalf("response body must contain both providers: %s", response)
	}

	// Groq was connected first, so it must come first (ordered by id).
	if strings.Index(response, `"name":"groq"`) > strings.Index(response, `"name":"openrouter"`) {
		t.Fatalf("providers must be ordered by id: %s", response)
	}

	if strings.Contains(response, "gsk_first_key") || strings.Contains(response, "sk-or_second_key") {
		t.Fatal("response body must not contain any api key")
	}
}

func TestListMyProvidersEmpty(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("34", 32))

	setupTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/providers", nil)

	rec := httptest.NewRecorder()

	ListMyProviders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != `{"providers":[]}`+"\n" {
		t.Fatalf("empty list body = %q, want %q", rec.Body.String(), `{"providers":[]}`+"\n")
	}
}

func deleteViaRouter(t *testing.T, id string) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()

	router.Delete("/providers/{id}", DeleteProvider)

	req := httptest.NewRequest(http.MethodDelete, "/providers/"+id, nil)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	return rec
}

func TestDeleteProvider(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("56", 32))

	setupTestDB(t)

	body := `{"name": "groq", "api_key": "gsk_delete_key"}`

	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))

	rec := httptest.NewRecorder()

	ConnectNewProvider(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("connect status = %d, want %d", rec.Code, http.StatusCreated)
	}

	rec = deleteViaRouter(t, "1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"id":1`) {
		t.Fatalf("response body must contain the deleted id: %s", rec.Body.String())
	}

	var count int

	err := store.DB.QueryRow("SELECT COUNT(*) FROM providers").Scan(&count)

	if err != nil {
		t.Fatalf("count providers: %v", err)
	}

	if count != 0 {
		t.Fatalf("providers count = %d, want 0", count)
	}
}

func TestDeleteProviderNotFound(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("78", 32))

	setupTestDB(t)

	rec := deleteViaRouter(t, "42")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestDeleteProviderInvalidID(t *testing.T) {
	t.Setenv(crypto.KeyEnvVar, strings.Repeat("9a", 32))

	setupTestDB(t)

	for _, id := range []string{"abc", "0", "-1"} {
		rec := deleteViaRouter(t, id)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("id %q: status = %d, want %d", id, rec.Code, http.StatusBadRequest)
		}
	}
}
