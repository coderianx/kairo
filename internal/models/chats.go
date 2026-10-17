package models

type ChatRequest struct {
	Provider string `json:"name"`
	Model    string `json:"model"`
	Content  string `json:"content"`
}

type CreateChatReq struct {
	Title string `json:"title"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

type Message struct {
	ID      int64  `json:"id"`
	ChatID  int64  `json:"chat_id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CreateChatRes struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

type SendMessageRes struct {
	ChatID           int64   `json:"chat_id"`
	UserMessage      Message `json:"user_message"`
	AssistantMessage Message `json:"assistant_message"`
}

type ListChatsRes struct {
	Chats []Chat `json:"chats"`
}

type ListMessagesRes struct {
	Messages []Message `json:"messages"`
}
