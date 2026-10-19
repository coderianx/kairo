package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompleteRoundTrip(t *testing.T) {
	var receivedBody string

	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}

		receivedAuth = r.Header.Get("Authorization")
		receivedBody = r.PostFormValue("")
	}))

	// PostFormValue does not read JSON bodies; capture the body manually.
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)

		if _, err := r.Body.Read(buf); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("read body: %v", err)
		}

		receivedBody = string(buf)
		receivedAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")

		w.Write([]byte(`{"choices": [{"message": {"role": "assistant", "content": "hello!"}}]}`))
	})

	defer server.Close()

	client := New(server.URL, "test-key")

	reply, err := client.Complete(context.Background(), "test-model", []Message{
		{Role: RoleUser, Content: "hi"},
	})

	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if reply != "hello!" {
		t.Fatalf("reply = %q, want %q", reply, "hello!")
	}

	if receivedAuth != "Bearer test-key" {
		t.Fatalf("authorization = %q, want %q", receivedAuth, "Bearer test-key")
	}

	if !strings.Contains(receivedBody, `"model":"test-model"`) {
		t.Fatalf("body must contain the model: %s", receivedBody)
	}

	if !strings.Contains(receivedBody, `"content":"hi"`) {
		t.Fatalf("body must contain the message: %s", receivedBody)
	}
}

func TestCompleteUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)

		w.Write([]byte(`{"error": "bad key"}`))
	}))

	defer server.Close()

	client := New(server.URL, "bad-key")

	if _, err := client.Complete(context.Background(), "m", []Message{{Role: RoleUser, Content: "hi"}}); err == nil {
		t.Fatal("complete must fail for unexpected status")
	}
}

func TestCompleteEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices": []}`))
	}))

	defer server.Close()

	client := New(server.URL, "key")

	if _, err := client.Complete(context.Background(), "m", []Message{{Role: RoleUser, Content: "hi"}}); err == nil {
		t.Fatal("complete must fail when no choices are returned")
	}
}

func TestStreamCompleteRoundTrip(t *testing.T) {
	var receivedBody string

	var receivedAccept string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)

		if _, err := r.Body.Read(buf); err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("read body: %v", err)
		}

		receivedBody = string(buf)
		receivedAccept = r.Header.Get("Accept")

		w.Header().Set("Content-Type", "text/event-stream")

		fmt.Fprint(w,
			"data: {\"choices\": [{\"delta\": {\"content\": \"Hel\"}}]}\n\n"+
				"data: {\"choices\": [{\"delta\": {\"content\": \"lo!\"}}]}\n\n"+
				// Empty deltas (role-only frames) must be skipped.
				"data: {\"choices\": [{\"delta\": {}}]}\n\n"+
				"data: [DONE]\n\n",
		)
	}))

	defer server.Close()

	var chunks []string

	reply, err := New(server.URL, "test-key").StreamComplete(
		context.Background(),
		"test-model",
		[]Message{{Role: RoleUser, Content: "hi"}},
		func(delta string) {
			chunks = append(chunks, delta)
		},
	)

	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	if reply != "Hello!" {
		t.Fatalf("reply = %q, want %q", reply, "Hello!")
	}

	if strings.Join(chunks, "") != "Hello!" {
		t.Fatalf("chunks = %v, want to join into Hello!", chunks)
	}

	if !strings.Contains(receivedBody, `"stream":true`) {
		t.Fatalf("body must enable streaming: %s", receivedBody)
	}

	if receivedAccept != "text/event-stream" {
		t.Fatalf("accept = %q, want text/event-stream", receivedAccept)
	}
}

func TestStreamCompleteUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)

		w.Write([]byte(`{"error": "bad key"}`))
	}))

	defer server.Close()

	client := New(server.URL, "bad-key")

	if _, err := client.StreamComplete(
		context.Background(),
		"m",
		[]Message{{Role: RoleUser, Content: "hi"}},
		func(string) {},
	); err == nil {
		t.Fatal("stream must fail for unexpected status")
	}
}
