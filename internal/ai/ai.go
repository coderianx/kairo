// Package ai provides a minimal OpenAI-compatible chat completion client used
// to talk to the connected providers (Groq, OpenRouter, ...).
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Roles used in chat conversations.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is a single message exchanged with the model.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client talks to an OpenAI-compatible /chat/completions endpoint.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New returns a client for the given provider base URL and api key.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

type completionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	// Stream enables SSE token streaming; omitted for the plain request.
	Stream bool `json:"stream,omitempty"`
}

// streamEvent is one OpenAI-compatible SSE data frame.
type streamEvent struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type completionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// ListModels fetches the model catalog from the provider's /models endpoint
// and returns the model ids.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/models",
		nil,
	)

	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	res, err := c.http.Do(req)

	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))

		return nil, fmt.Errorf("unexpected status %d: %s", res.StatusCode, string(body))
	}

	var data struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]string, 0, len(data.Data))

	for _, model := range data.Data {
		models = append(models, model.ID)
	}

	return models, nil
}

// StreamComplete sends the conversation history to the model and streams the
// reply through onChunk as SSE deltas arrive, returning the full reply once
// the stream ends.
func (c *Client) StreamComplete(
	ctx context.Context,
	model string,
	history []Message,
	onChunk func(string),
) (string, error) {
	payload, err := json.Marshal(completionRequest{
		Model:    model,
		Messages: history,
		Stream:   true,
	})

	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)

	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	res, err := c.http.Do(req)

	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))

		return "", fmt.Errorf("unexpected status %d: %s", res.StatusCode, string(body))
	}

	// SSE frames are line based; allow long data lines.
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var full strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

		if data == "[DONE]" {
			break
		}

		var event streamEvent

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if len(event.Choices) == 0 || event.Choices[0].Delta.Content == "" {
			continue
		}

		full.WriteString(event.Choices[0].Delta.Content)

		if onChunk != nil {
			onChunk(event.Choices[0].Delta.Content)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read stream: %w", err)
	}

	return full.String(), nil
}

// Complete sends the conversation history to the model and returns the
// assistant's reply.
func (c *Client) Complete(ctx context.Context, model string, history []Message) (string, error) {
	payload, err := json.Marshal(completionRequest{
		Model:    model,
		Messages: history,
	})

	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)

	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	res, err := c.http.Do(req)

	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))

		return "", fmt.Errorf("unexpected status %d: %s", res.StatusCode, string(body))
	}

	var data completionResponse

	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(data.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return data.Choices[0].Message.Content, nil
}
