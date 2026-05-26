package notification

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"karasu/internal/store"
)

func TestNewTelegramAlertNotifierRequiresBotTokenAndChatID(t *testing.T) {
	t.Parallel()

	if _, err := NewTelegramAlertNotifier("", "123"); err == nil {
		t.Fatal("expected error when bot token is missing")
	}
	if _, err := NewTelegramAlertNotifier("token", ""); err == nil {
		t.Fatal("expected error when chat id is missing")
	}
}

func TestTelegramAlertNotifierNotifyAlert(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotContentType string
	var handlerErr error
	var payload struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			handlerErr = err
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			handlerErr = err
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier, err := NewTelegramAlertNotifier("token-123", "chat-456")
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}
	notifier.client = server.Client()
	notifier.baseURL = server.URL

	alert := store.AlertEvent{
		ID:        "al_1",
		Key:       "exchange:rate-limit-ban",
		Category:  "exchange",
		Severity:  store.AlertSeverityCritical,
		Message:   "rate limit active until 2026-05-26T22:00:00Z",
		Source:    "exchange",
		Symbol:    "BTC",
		Active:    true,
		Count:     2,
		FirstSeen: time.Date(2026, 5, 26, 21, 0, 0, 0, time.UTC),
		LastSeen:  time.Date(2026, 5, 26, 21, 5, 0, 0, time.UTC),
	}

	if err := notifier.NotifyAlert(alert); err != nil {
		t.Fatalf("NotifyAlert returned error: %v", err)
	}
	if handlerErr != nil {
		t.Fatalf("handler returned error: %v", handlerErr)
	}

	if gotPath != "/bottoken-123/sendMessage" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("unexpected content-type %q", gotContentType)
	}
	if payload.ChatID != "chat-456" {
		t.Fatalf("unexpected chat_id %q", payload.ChatID)
	}
	if !strings.Contains(payload.Text, "Karasu ALERTE ACTIVE") {
		t.Fatalf("expected active alert title, got %q", payload.Text)
	}
	if !strings.Contains(payload.Text, "Symbole: BTC") {
		t.Fatalf("expected symbol in message, got %q", payload.Text)
	}
	if !strings.Contains(payload.Text, "Occurrences: 2") {
		t.Fatalf("expected count in message, got %q", payload.Text)
	}
}
