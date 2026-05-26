package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"karasu/internal/store"
)

const defaultTelegramTimeout = 10 * time.Second

type TelegramAlertNotifier struct {
	botToken string
	chatID   string
	baseURL  string
	client   *http.Client
}

func NewTelegramAlertNotifier(botToken, chatID string) (*TelegramAlertNotifier, error) {
	botToken = strings.TrimSpace(botToken)
	chatID = strings.TrimSpace(chatID)

	if botToken == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}
	if chatID == "" {
		return nil, fmt.Errorf("telegram chat id is required")
	}

	return &TelegramAlertNotifier{
		botToken: botToken,
		chatID:   chatID,
		baseURL:  "https://api.telegram.org",
		client:   &http.Client{Timeout: defaultTelegramTimeout},
	}, nil
}

func (n *TelegramAlertNotifier) NotifyAlert(alert store.AlertEvent) error {
	if n == nil {
		return fmt.Errorf("telegram notifier is nil")
	}

	body, err := json.Marshal(map[string]any{
		"chat_id": n.chatID,
		"text":    formatTelegramAlert(alert),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, n.endpoint(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return fmt.Errorf("telegram send failed: status=%d read_body_err=%w", resp.StatusCode, readErr)
		}
		return fmt.Errorf("telegram send failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

func (n *TelegramAlertNotifier) endpoint() string {
	return fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(n.baseURL, "/"), n.botToken)
}

func formatTelegramAlert(alert store.AlertEvent) string {
	lines := []string{
		fmt.Sprintf("%s Karasu %s", telegramAlertPrefix(alert), telegramAlertState(alert)),
		fmt.Sprintf("Catégorie: %s", alert.Category),
		fmt.Sprintf("Sévérité: %s", alert.Severity),
		fmt.Sprintf("Source: %s", alert.Source),
	}

	if symbol := strings.TrimSpace(alert.Symbol); symbol != "" {
		lines = append(lines, fmt.Sprintf("Symbole: %s", symbol))
	}

	lines = append(
		lines,
		fmt.Sprintf("Message: %s", alert.Message),
		fmt.Sprintf("Occurrences: %d", alert.Count),
		fmt.Sprintf("Première apparition: %s", alert.FirstSeen.UTC().Format(time.RFC3339)),
		fmt.Sprintf("Dernière apparition: %s", alert.LastSeen.UTC().Format(time.RFC3339)),
	)

	return strings.Join(lines, "\n")
}

func telegramAlertPrefix(alert store.AlertEvent) string {
	if !alert.Active {
		return "✅"
	}

	switch alert.Severity {
	case store.AlertSeverityCritical:
		return "🚨"
	case store.AlertSeverityWarning:
		return "⚠️"
	default:
		return "ℹ️"
	}
}

func telegramAlertState(alert store.AlertEvent) string {
	if alert.Active {
		return "ALERTE ACTIVE"
	}
	return "ALERTE RESOLUE"
}
