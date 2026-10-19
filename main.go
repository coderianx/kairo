package main

import (
	"kairo/internal/config"
	"kairo/internal/handlers/chats"
	"kairo/internal/handlers/providers"
	"kairo/internal/store"
	"log"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

func main() {
	// init project
	err := config.Init()

	if err != nil {
		log.Fatal(err)
	}

	kairoPath := config.GetKairoPath()

	// connect database
	err = store.ConnectDB(
		filepath.Join(kairoPath, "kairo.db"),
	)

	if err != nil {
		log.Fatal(err)
	}

	defer store.CloseDB()

	// create tables
	err = store.CreateTables()

	if err != nil {
		log.Fatal(err)
	}

	router := chi.NewRouter()

	router.Route("/providers", func(r chi.Router) {
		r.Post("/", providers.ConnectNewProvider)
		r.Get("/", providers.ListMyProviders)
		r.Get("/{name}/models", providers.ListModels)
		r.Delete("/{id}", providers.DeleteProvider)
	})

	router.Route("/chats", func(r chi.Router) {
		r.Post("/", chats.CreateChat)
		r.Get("/", chats.ListChats)
		r.Post("/{id}/messages", chats.SendMessage)
		r.Post("/{id}/messages/stream", chats.StreamSendMessage)
		r.Get("/{id}/messages", chats.GetMessages)
	})

	err = http.ListenAndServe(":8080", router)

	if err != nil {
		log.Fatal(err)
	}
}
