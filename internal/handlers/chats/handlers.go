package chats

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kairo/internal/ai"
	"kairo/internal/crypto"
	"kairo/internal/models"
	"kairo/internal/store"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// historyLimit is the number of recent messages sent to the model with every
// request.
const historyLimit = 15

// maxChatTitle bounds the chat title length.
const maxChatTitle = 100

// derivedTitleLength is the length of a chat title generated from the first
// user message, before the trailing ellipsis.
const derivedTitleLength = 48

// deriveChatTitle collapses the first user message into a chat title,
// truncated with an ellipsis when it is too long.
func deriveChatTitle(content string) string {
	content = strings.Join(strings.Fields(content), " ")

	runes := []rune(content)

	if len(runes) == 0 {
		return ""
	}

	if len(runes) > derivedTitleLength {
		return string(runes[:derivedTitleLength]) + "…"
	}

	return string(runes)
}

func CreateChat(
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

	var req models.CreateChatReq

	// An empty body is allowed and creates a chat with the default title.
	err := json.NewDecoder(r.Body).Decode(&req)

	if err != nil && !errors.Is(err, io.EOF) {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid request",
		})

		return
	}

	if len(req.Title) > maxChatTitle {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "title is too long",
		})

		return
	}

	if req.Title == "" {
		req.Title = store.DefaultChatTitle
	}

	id, createdAt, err := store.CreateChat(req.Title)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "create chat failed",
		})

		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(models.CreateChatRes{
		ID:        id,
		Title:     req.Title,
		CreatedAt: createdAt,
	})
}

func ListChats(
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

	chats, err := store.ListChats()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "list chats failed",
		})

		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(models.ListChatsRes{
		Chats: chats,
	})
}

func GetMessages(
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

	chatID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	if err != nil || chatID <= 0 {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid chat id",
		})

		return
	}

	exists, err := store.ChatExists(chatID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "check chat failed",
		})

		return
	}

	if !exists {
		w.WriteHeader(http.StatusNotFound)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "chat not found",
		})

		return
	}

	messages, err := store.ListMessages(chatID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "list messages failed",
		})

		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(models.ListMessagesRes{
		Messages: messages,
	})
}

// preparedChat holds everything a completion needs after the request has
// been validated and the user message has been saved.
type preparedChat struct {
	chatID        int64
	req           models.ChatRequest
	baseURL       string
	apiKey        string
	history       []ai.Message
	userMessageID int64
}

// prepareChatRequest validates a send-message request, saves the user
// message and loads the model history. On any failure the error response has
// already been written and ok is false.
func prepareChatRequest(
	w http.ResponseWriter,
	r *http.Request,
) (preparedChat, bool) {
	w.Header().Set("Content-Type", "application/json")

	if store.DB == nil {
		w.WriteHeader(500)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "database not connected",
		})

		return preparedChat{}, false
	}

	chatID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	if err != nil || chatID <= 0 {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid chat id",
		})

		return preparedChat{}, false
	}

	exists, err := store.ChatExists(chatID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "check chat failed",
		})

		return preparedChat{}, false
	}

	if !exists {
		w.WriteHeader(http.StatusNotFound)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "chat not found",
		})

		return preparedChat{}, false
	}

	var req models.ChatRequest

	err = json.NewDecoder(r.Body).Decode(&req)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "invalid request",
		})

		return preparedChat{}, false
	}

	if req.Provider == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "name is required",
		})

		return preparedChat{}, false
	}

	if req.Model == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "model is required",
		})

		return preparedChat{}, false
	}

	if req.Content == "" {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "content is required",
		})

		return preparedChat{}, false
	}

	baseURL, encryptedAPIKey, err := store.GetProviderByName(req.Provider)

	if errors.Is(err, store.ErrProviderNotFound) {
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "provider not connected",
		})

		return preparedChat{}, false
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "get provider failed",
		})

		return preparedChat{}, false
	}

	apiKey, err := crypto.Decrypt(encryptedAPIKey)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "decrypt api_key failed",
		})

		return preparedChat{}, false
	}

	userMessageID, err := store.CreateMessage(chatID, ai.RoleUser, req.Content)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "save message failed",
		})

		return preparedChat{}, false
	}

	// The first message names the chat: the default placeholder title is
	// replaced with the message text. A failed rename is cosmetic and never
	// fails the message.
	chat, err := store.GetChat(chatID)

	if err == nil && chat.Title == store.DefaultChatTitle {
		derived := deriveChatTitle(req.Content)

		if derived != "" {
			_ = store.RenameChat(chatID, derived)
		}
	}

	history, err := store.GetLastMessages(chatID, historyLimit)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "load history failed",
		})

		return preparedChat{}, false
	}

	messages := make([]ai.Message, 0, len(history))

	for _, message := range history {
		messages = append(messages, ai.Message{
			Role:    message.Role,
			Content: message.Content,
		})
	}

	return preparedChat{
		chatID:        chatID,
		req:           req,
		baseURL:       baseURL,
		apiKey:        apiKey,
		history:       messages,
		userMessageID: userMessageID,
	}, true
}

func SendMessage(
	w http.ResponseWriter,
	r *http.Request,
) {
	prep, ok := prepareChatRequest(w, r)

	if !ok {
		return
	}

	reply, err := ai.New(prep.baseURL, prep.apiKey).Complete(
		r.Context(),
		prep.req.Model,
		prep.history,
	)

	if err != nil {
		w.WriteHeader(http.StatusBadGateway)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "provider request failed",
		})

		return
	}

	assistantMessageID, err := store.CreateMessage(prep.chatID, ai.RoleAssistant, reply)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "save message failed",
		})

		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(models.SendMessageRes{
		ChatID: prep.chatID,
		UserMessage: models.Message{
			ID:      prep.userMessageID,
			ChatID:  prep.chatID,
			Role:    ai.RoleUser,
			Content: prep.req.Content,
		},
		AssistantMessage: models.Message{
			ID:      assistantMessageID,
			ChatID:  prep.chatID,
			Role:    ai.RoleAssistant,
			Content: reply,
		},
	})
}

// writeSSE writes one data event and flushes it immediately.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, payload models.StreamEvent) {
	data, err := json.Marshal(payload)

	if err != nil {
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", data)

	flusher.Flush()
}

// StreamSendMessage behaves like SendMessage but streams the assistant reply
// to the client as SSE events while the model generates it:
//
//	data: {"type":"chunk","chat_id":1,"content":"..."}   (many)
//	data: {"type":"done","chat_id":1,"user_message":{...},"assistant_message":{...}}
//	data: [DONE]
//
// Failures before the first event keep plain JSON error responses; failures
// mid-stream arrive as a {"type":"error"} event. Nothing partial is saved:
// either the full reply lands in the database or none of it does.
func StreamSendMessage(
	w http.ResponseWriter,
	r *http.Request,
) {
	prep, ok := prepareChatRequest(w, r)

	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)

	if !ok {
		w.WriteHeader(http.StatusInternalServerError)

		json.NewEncoder(w).Encode(map[string]any{
			"error": "streaming unsupported",
		})

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	var full strings.Builder

	reply, err := ai.New(prep.baseURL, prep.apiKey).StreamComplete(
		r.Context(),
		prep.req.Model,
		prep.history,
		func(delta string) {
			full.WriteString(delta)

			writeSSE(w, flusher, models.StreamEvent{
				Type:    "chunk",
				ChatID:  prep.chatID,
				Content: delta,
			})
		},
	)

	if err != nil {
		writeSSE(w, flusher, models.StreamEvent{
			Type:  "error",
			Error: "provider request failed",
		})

		return
	}

	assistantMessageID, err := store.CreateMessage(prep.chatID, ai.RoleAssistant, reply)

	if err != nil {
		writeSSE(w, flusher, models.StreamEvent{
			Type:  "error",
			Error: "save message failed",
		})

		return
	}

	writeSSE(w, flusher, models.StreamEvent{
		Type:   "done",
		ChatID: prep.chatID,
		UserMessage: &models.Message{
			ID:      prep.userMessageID,
			ChatID:  prep.chatID,
			Role:    ai.RoleUser,
			Content: prep.req.Content,
		},
		AssistantMessage: &models.Message{
			ID:      assistantMessageID,
			ChatID:  prep.chatID,
			Role:    ai.RoleAssistant,
			Content: reply,
		},
	})

	fmt.Fprint(w, "data: [DONE]\n\n")

	flusher.Flush()
}
