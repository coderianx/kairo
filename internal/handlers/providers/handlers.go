package providers

import (
	"encoding/json"
	"kairo/internal/config"
	"kairo/internal/crypto"
	"kairo/internal/models"
	"kairo/internal/store"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func ConnectNewProvider(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	if store.DB == nil {
		w.WriteHeader(500)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "database not connected",
		})

		return
	}

	var req models.ConnectProviderReq

	err := json.NewDecoder(r.Body).Decode(&req)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid request",
		})

		return
	}

	if req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "name is required",
		})

		return
	}

	if len(req.Name) > 50 {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "name is too long",
		})

		return
	}

	switch req.Name {
	case "groq":
		req.BaseURL = config.GroqBaseURL
	case "openrouter":
		req.BaseURL = config.OpenrouterBaseURL
	}

	if req.BaseURL == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "base_url is required",
		})

		return
	}

	if req.APIKey == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "api_key is required",
		})

		return
	}

	encryptedAPIKey, err := crypto.Encrypt(req.APIKey)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "encrypt api_key failed",
		})

		return
	}

	exists, err := store.ProviderExistsByBaseURL(req.BaseURL)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "check provider failed",
		})

		return
	}

	if exists {
		w.WriteHeader(http.StatusConflict)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "provider with this base_url already exists",
		})

		return
	}

	id, err := store.CreateProvider(req.Name, req.BaseURL, encryptedAPIKey)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "save provider failed",
		})

		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(models.ConnectProviderRes{
		ID:      id,
		Name:    req.Name,
		BaseURL: req.BaseURL,
	})
}

func ListMyProviders(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	if store.DB == nil {
		w.WriteHeader(500)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "database not connected",
		})

		return
	}

	providers, err := store.ListProviders()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "list providers failed",
		})

		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(models.ListProvidersRes{
		Providers: providers,
	})
}

func DeleteProvider(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Content-Type", "application/json")

	if store.DB == nil {
		w.WriteHeader(500)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "database not connected",
		})

		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	if err != nil || id <= 0 {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid id",
		})

		return
	}

	deleted, err := store.DeleteProvider(id)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "delete provider failed",
		})

		return
	}

	if !deleted {
		w.WriteHeader(http.StatusNotFound)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "provider not found",
		})

		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]any{
		"message": "provider deleted",
		"id":      id,
	})
}
