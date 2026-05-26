package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"karasu/internal/exchange"
	"karasu/internal/market"
	"karasu/internal/store"
)

const (
	defaultTelegramTimeout          = 35 * time.Second
	defaultTelegramLongPollTimeout  = 30 * time.Second
	defaultTelegramCommandListLimit = 5
)

type TelegramAlertNotifier struct {
	botToken      string
	chatID        string
	baseURL       string
	client        *http.Client
	exchangeClient exchange.ExchangeClient
	candleStore    store.CandleStore
	nextUpdateID   int64
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

func (n *TelegramAlertNotifier) SetCommandSources(exchangeClient exchange.ExchangeClient, candleStore store.CandleStore) {
	if n == nil {
		return
	}
	n.exchangeClient = exchangeClient
	n.candleStore = candleStore
}

func (n *TelegramAlertNotifier) NotifyAlert(alert store.AlertEvent) error {
	if n == nil {
		return fmt.Errorf("telegram notifier is nil")
	}

	return n.sendText(context.Background(), n.chatID, formatTelegramAlert(alert))
}

func (n *TelegramAlertNotifier) Run(ctx context.Context) {
	if n == nil || n.exchangeClient == nil || n.candleStore == nil {
		return
	}

	for {
		if err := n.pollOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("telegram command polling failed", "err", err)

			timer := time.NewTimer(3 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (n *TelegramAlertNotifier) pollOnce(ctx context.Context) error {
	updates, err := n.getUpdates(ctx)
	if err != nil {
		return err
	}

	for _, update := range updates {
		if update.UpdateID >= n.nextUpdateID {
			n.nextUpdateID = update.UpdateID + 1
		}
		if update.Message == nil {
			continue
		}
		if strings.TrimSpace(strconv.FormatInt(update.Message.Chat.ID, 10)) != n.chatID {
			continue
		}

		response, shouldReply, err := n.commandResponse(update.Message.Text)
		if err != nil {
			response = fmt.Sprintf("Erreur Karasu: %v", err)
			shouldReply = true
		}
		if !shouldReply {
			continue
		}
		if err := n.sendText(ctx, n.chatID, response); err != nil {
			return err
		}
	}

	return nil
}

func (n *TelegramAlertNotifier) commandResponse(message string) (string, bool, error) {
	fields := strings.Fields(strings.TrimSpace(message))
	if len(fields) == 0 {
		return "", false, nil
	}

	command := strings.ToLower(fields[0])
	if idx := strings.Index(command, "@"); idx >= 0 {
		command = command[:idx]
	}

	switch command {
	case "/wallet":
		response, err := n.buildWalletResponse()
		return response, true, err
	case "/opportunities":
		response, err := n.buildOpportunitiesResponse()
		return response, true, err
	case "/decision":
		response, err := n.buildDecisionResponse()
		return response, true, err
	case "/start", "/help":
		return telegramHelpMessage(), true, nil
	default:
		if strings.HasPrefix(command, "/") {
			return telegramHelpMessage(), true, nil
		}
		return "", false, nil
	}
}

func (n *TelegramAlertNotifier) buildWalletResponse() (string, error) {
	wallet, err := n.exchangeClient.Wallet()
	if err != nil {
		return "", fmt.Errorf("wallet indisponible: %w", err)
	}

	assets := append([]exchange.WalletAsset(nil), wallet.Assets...)
	sort.SliceStable(assets, func(i, j int) bool {
		return assets[i].Value > assets[j].Value
	})

	lines := []string{
		"💼 Portefeuille",
		fmt.Sprintf("Valeur totale: %s EUR", formatTelegramNumber(wallet.TotalValue)),
		fmt.Sprintf("Cash: %s EUR", formatTelegramNumber(wallet.CashValue)),
		fmt.Sprintf("Actifs: %s EUR", formatTelegramNumber(wallet.AssetValue)),
		fmt.Sprintf("PnL global: %s EUR (%s)", formatSignedTelegramNumber(wallet.PnLValue), formatSignedTelegramPercent(wallet.PnLPercent)),
	}

	visibleCount := 0
	for _, asset := range assets {
		if asset.Value < 0.01 {
			continue
		}
		if visibleCount == 0 {
			lines = append(lines, "Top positions:")
		}
		lines = append(lines, fmt.Sprintf("- %s: %s EUR (%s)", strings.ToUpper(asset.Symbol), formatTelegramNumber(asset.Value), formatSignedTelegramPercent(asset.PnLPercent)))
		visibleCount++
		if visibleCount >= defaultTelegramCommandListLimit {
			break
		}
	}

	if visibleCount == 0 {
		lines = append(lines, "Aucune position significative.")
	}

	return strings.Join(lines, "\n"), nil
}

func (n *TelegramAlertNotifier) buildOpportunitiesResponse() (string, error) {
	opportunities, err := market.TopOpportunities(n.exchangeClient, n.candleStore, defaultTelegramCommandListLimit)
	if err != nil {
		return "", fmt.Errorf("opportunites indisponibles: %w", err)
	}
	if len(opportunities) == 0 {
		return "🎯 Opportunités\nAucune opportunité disponible pour le moment.", nil
	}

	lines := []string{"🎯 Opportunités prioritaires"}
	for i, opportunity := range opportunities {
		lines = append(lines,
			fmt.Sprintf("%d. %s — %s — %s — score %s", i+1, opportunity.Symbol, translateTelegramPrimaryAction(opportunity.PrimaryAction), translateTelegramPriorityBand(opportunity.PriorityBand), formatTelegramNumber(opportunity.PriorityScore)),
			fmt.Sprintf("   %s | leader %s en %s", translateTelegramSummary(opportunity.Summary), translateTelegramProfileLabel(opportunity.Leader.Label), translateTelegramState(opportunity.Leader.State)),
		)
	}

	return strings.Join(lines, "\n"), nil
}

func (n *TelegramAlertNotifier) buildDecisionResponse() (string, error) {
	wallet, err := n.exchangeClient.Wallet()
	if err != nil {
		return "", fmt.Errorf("wallet indisponible: %w", err)
	}
	opportunities, err := market.TopOpportunities(n.exchangeClient, n.candleStore, 40)
	if err != nil {
		return "", fmt.Errorf("scanner indisponible: %w", err)
	}

	decision := market.BuildWalletDecision(wallet, opportunities)
	lines := []string{
		"🧭 Décision portefeuille",
		fmt.Sprintf("Vente immédiate: %s", ternaryString(decision.HasSellSignal, "oui, vérification recommandée", "non, aucun signal défensif fort")),
		fmt.Sprintf("À alléger: %d", len(decision.Reduce)),
		fmt.Sprintf("À surveiller: %d", len(decision.Watch)),
		fmt.Sprintf("À renforcer: %d", len(decision.Reinforce)),
	}

	appendDecisionItems := func(title string, items []market.WalletDecisionItem) {
		if len(items) == 0 {
			return
		}
		lines = append(lines, title)
		for _, item := range items[:min(len(items), defaultTelegramCommandListLimit)] {
			lines = append(lines, fmt.Sprintf("- %s (%s EUR): %s", item.Symbol, formatTelegramNumber(item.Value), item.Reason))
		}
	}

	appendDecisionItems("Priorité allègement:", decision.Reduce)
	appendDecisionItems("Surveillance:", decision.Watch)
	appendDecisionItems("Renforcement potentiel:", decision.Reinforce)

	return strings.Join(lines, "\n"), nil
}

func (n *TelegramAlertNotifier) sendText(ctx context.Context, chatID, text string) error {
	body, err := json.Marshal(map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.sendMessageEndpoint(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return fmt.Errorf("telegram send failed: status=%d read_body_err=%w", resp.StatusCode, readErr)
		}
		return fmt.Errorf("telegram send failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

func (n *TelegramAlertNotifier) getUpdates(ctx context.Context) ([]telegramUpdate, error) {
	query := url.Values{}
	query.Set("timeout", strconv.FormatInt(int64(defaultTelegramLongPollTimeout/time.Second), 10))
	if n.nextUpdateID > 0 {
		query.Set("offset", strconv.FormatInt(n.nextUpdateID, 10))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.getUpdatesEndpoint()+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram getUpdates request: %w", err)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to poll telegram updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return nil, fmt.Errorf("telegram poll failed: status=%d read_body_err=%w", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("telegram poll failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload telegramGetUpdatesResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode telegram updates: %w", err)
	}
	return payload.Result, nil
}

func (n *TelegramAlertNotifier) sendMessageEndpoint() string {
	return fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(n.baseURL, "/"), n.botToken)
}

func (n *TelegramAlertNotifier) getUpdatesEndpoint() string {
	return fmt.Sprintf("%s/bot%s/getUpdates", strings.TrimRight(n.baseURL, "/"), n.botToken)
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

type telegramGetUpdatesResponse struct {
	Result []telegramUpdate `json:"result"`
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	Text string       `json:"text"`
	Chat telegramChat `json:"chat"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

func telegramHelpMessage() string {
	return "Commandes disponibles:\n/wallet\n/opportunities\n/decision"
}

func translateTelegramPrimaryAction(action string) string {
	switch action {
	case "act-now":
		return "agir maintenant"
	case "watch-closely":
		return "surveiller de près"
	case "prepare":
		return "préparer"
	case "avoid":
		return "éviter"
	default:
		return action
	}
}

func translateTelegramPriorityBand(band string) string {
	switch band {
	case "actionable":
		return "actionnable"
	case "strong-watch":
		return "surveillance forte"
	case "watchlist":
		return "liste de surveillance"
	case "defensive":
		return "défensif"
	default:
		return band
	}
}

func translateTelegramState(state string) string {
	switch state {
	case "entry":
		return "entrée"
	case "hold":
		return "maintien"
	case "watch":
		return "surveillance"
	case "exit":
		return "sortie"
	case "avoid":
		return "à éviter"
	default:
		return state
	}
}

func translateTelegramProfileLabel(label string) string {
	switch label {
	case "Intraday Momentum":
		return "momentum intraday"
	case "Swing Balance":
		return "swing équilibre"
	case "Trend Follow":
		return "suivi de tendance"
	default:
		return label
	}
}

func translateTelegramSummary(summary string) string {
	switch summary {
	case "Fresh multi-profile entry detected":
		return "entrée fraîche multi-profils"
	case "Fresh entry led by the strongest profile":
		return "entrée fraîche portée par le profil leader"
	case "Recent exit transition requires caution":
		return "sortie récente, prudence"
	case "High-conviction alignment remains active":
		return "alignement fort toujours actif"
	case "Constructive setup worth close monitoring":
		return "configuration constructive à surveiller"
	case "Setup is building but not fully confirmed":
		return "configuration en construction"
	case "Context remains defensive or low conviction":
		return "contexte défensif ou peu convaincant"
	default:
		return summary
	}
}

func formatTelegramNumber(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

func formatSignedTelegramNumber(value float64) string {
	if value > 0 {
		return "+" + formatTelegramNumber(value)
	}
	return formatTelegramNumber(value)
}

func formatSignedTelegramPercent(value float64) string {
	if value > 0 {
		return fmt.Sprintf("+%.2f%%", value)
	}
	return fmt.Sprintf("%.2f%%", value)
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func ternaryString(condition bool, whenTrue, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}
