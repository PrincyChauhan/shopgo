package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientEnabled(t *testing.T) {
	if NewClient("https://api.example.com", "", "gemini-2.5-flash").Enabled() {
		t.Fatal("expected remote client without API key to be disabled")
	}
	if !NewClient("https://api.example.com", "secret", "gemini-2.5-flash").Enabled() {
		t.Fatal("expected remote client with API key to be enabled")
	}
	if !NewClient("http://localhost:11434/v1", "", "llama3.2").Enabled() {
		t.Fatal("expected localhost client without API key to be enabled")
	}
}

func TestClientChatOpenAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want Bearer test-key", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "  Crisp wireless sound.  "}},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "llama-3.3-70b-versatile")
	got, err := c.Chat(context.Background(), "sys", "user", 100)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if got != "Crisp wireless sound." {
		t.Fatalf("Chat = %q, want %q", got, "Crisp wireless sound.")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClientChatGeminiNative(t *testing.T) {
	c := NewClient("https://generativelanguage.googleapis.com/v1beta/openai", "AQ.test_key", "gemini-2.5-flash")
	c.HTTP = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			wantURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"
			if r.URL.String() != wantURL {
				t.Fatalf("URL = %q, want %q", r.URL.String(), wantURL)
			}
			if got := r.Header.Get("x-goog-api-key"); got != "AQ.test_key" {
				t.Fatalf("x-goog-api-key = %q, want AQ.test_key", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"candidates": [{"content": {"parts": [{"text": "  Soft cotton tee. "}]}}]
				}`)),
			}, nil
		}),
	}

	got, err := c.Chat(context.Background(), "sys", "user", 100)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if got != "Soft cotton tee." {
		t.Fatalf("Chat = %q, want %q", got, "Soft cotton tee.")
	}
}

func TestClientChatUnconfigured(t *testing.T) {
	c := NewClient("https://api.example.com", "", "gemini-2.5-flash")
	_, err := c.Chat(context.Background(), "sys", "user", 100)
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}
