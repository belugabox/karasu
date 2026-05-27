package notification

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"karasu/internal/exchange"
	"karasu/internal/market"
	"karasu/internal/store"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	defaultTelegramTimeout          = 35 * time.Second
	defaultTelegramLongPollTimeout  = 30 * time.Second
	defaultTelegramCommandListLimit = 5
)

const (
	telegramCommandWallet        = "/wallet"
	telegramCommandOpportunities = "/opportunities"
	telegramCommandDecision      = "/decision"
	telegramCommandHealth        = "/health"
	telegramCommandAlerts        = "/alerts"
	telegramCommandHelp          = "/help"
)

type TelegramAlertNotifier struct {
	botToken       string
	chatID         string
	bot            *telego.Bot
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

	bot, err := telego.NewBot(
		botToken,
		telego.WithHTTPClient(&http.Client{Timeout: defaultTelegramTimeout}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize telegram bot: %w", err)
	}

	return &TelegramAlertNotifier{
		botToken: botToken,
		chatID:   chatID,
		bot:      bot,
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
		updateID := int64(update.UpdateID)
		if updateID >= n.nextUpdateID {
			n.nextUpdateID = updateID + 1
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
	command, ok := normalizeTelegramCommand(message)
	if !ok {
		return telegramQuickRepliesMessage(), true, nil
	}

	switch command {
	case telegramCommandWallet:
		response, err := n.buildWalletResponse()
		return response, true, err
	case telegramCommandOpportunities:
		response, err := n.buildOpportunitiesResponse()
		return response, true, err
	case telegramCommandDecision:
		response, err := n.buildDecisionResponse()
		return response, true, err
	case telegramCommandHealth:
		response, err := n.buildHealthResponse()
		return response, true, err
	case telegramCommandAlerts:
		response, err := n.buildAlertsResponse()
		return response, true, err
	case telegramCommandHelp:
		return telegramHelpMessage(), true, nil
	default:
		return telegramQuickRepliesMessage(), true, nil
	}
}

func normalizeTelegramCommand(message string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(strings.ToLower(message)))
	if len(fields) == 0 {
		return "", false
	}

	command := fields[0]
	if idx := strings.Index(command, "@"); idx >= 0 {
		command = command[:idx]
	}

	alias := map[string]string{
		"/start":                     telegramCommandHelp,
		telegramCommandHelp:          telegramCommandHelp,
		telegramCommandWallet:        telegramCommandWallet,
		telegramCommandOpportunities: telegramCommandOpportunities,
		telegramCommandDecision:      telegramCommandDecision,
		telegramCommandHealth:        telegramCommandHealth,
		telegramCommandAlerts:        telegramCommandAlerts,
		"wallet":                     telegramCommandWallet,
		"portefeuille":               telegramCommandWallet,
		"opportunites":               telegramCommandOpportunities,
		"opportunities":              telegramCommandOpportunities,
		"scanner":                    telegramCommandOpportunities,
		"decision":                   telegramCommandDecision,
		"vendre":                     telegramCommandDecision,
		"health":                     telegramCommandHealth,
		"sante":                      telegramCommandHealth,
		"alerts":                     telegramCommandAlerts,
		"alertes":                    telegramCommandAlerts,
	}

	normalized, ok := alias[command]
	return normalized, ok
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

	lines := []string{
		fmt.Sprintf("🎯 Opportunités prioritaires (top %d)", len(opportunities)),
		"Lecture rapide: action | profil leader | contexte",
	}
	for i, opportunity := range opportunities {
		leaderLabel := translateTelegramProfileLabel(opportunity.Leader.Label)
		if icon := strings.TrimSpace(opportunity.Leader.Icon); icon != "" {
			leaderLabel = icon + " " + leaderLabel
		}
		profileHint := compactTelegramProfileHint(opportunity.Leader.Label, opportunity.Leader.Description)

		lines = append(lines,
			fmt.Sprintf("%d) %s | score %s", i+1, opportunity.Symbol, formatTelegramNumber(opportunity.PriorityScore)),
			fmt.Sprintf("   action: %s (%s)", translateTelegramPrimaryAction(opportunity.PrimaryAction), translateTelegramPriorityBand(opportunity.PriorityBand)),
			fmt.Sprintf("   leader: %s - %s", leaderLabel, translateTelegramState(opportunity.Leader.State)),
			fmt.Sprintf("   contexte: %s", translateTelegramSummary(opportunity.Summary)),
			fmt.Sprintf("   profil: %s", profileHint),
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

func (n *TelegramAlertNotifier) buildHealthResponse() (string, error) {
	now := time.Now().UTC()
	lines := []string{"🩺 Santé rapide"}

	status := "ok"
	if _, err := n.exchangeClient.Wallet(); err != nil {
		status = "degrade"
		lines = append(lines, "Exchange: indisponible")
	} else {
		lines = append(lines, "Exchange: disponible")
	}

	opportunities, oppErr := market.TopOpportunities(n.exchangeClient, n.candleStore, 20)
	if oppErr != nil {
		status = "degrade"
		lines = append(lines, "Scanner: indisponible")
	} else {
		actionable := 0
		for _, opportunity := range opportunities {
			if opportunity.PriorityBand == "actionable" {
				actionable++
			}
		}
		lines = append(lines, fmt.Sprintf("Scanner: %d opportunites (%d actionnables)", len(opportunities), actionable))
	}

	activeAlerts := 0
	if alertStore, ok := n.candleStore.(store.AlertStore); ok {
		alerts, total, err := alertStore.ListAlerts(5, 0, true)
		if err != nil {
			status = "degrade"
			lines = append(lines, "Alertes: lecture indisponible")
		} else {
			activeAlerts = total
			if len(alerts) > 0 {
				lines = append(lines, fmt.Sprintf("Alertes actives: %d (ex: %s)", total, alerts[0].Key))
			} else {
				lines = append(lines, "Alertes actives: 0")
			}
		}
	}

	if activeAlerts > 0 {
		status = "degrade"
	}

	lines = append(lines, fmt.Sprintf("Etat global: %s", strings.ToUpper(status)))
	lines = append(lines, fmt.Sprintf("Horodatage: %s", now.Format("2006-01-02 15:04:05 UTC")))

	return strings.Join(lines, "\n"), nil
}

func (n *TelegramAlertNotifier) buildAlertsResponse() (string, error) {
	alertStore, ok := n.candleStore.(store.AlertStore)
	if !ok {
		return "⚠️ Alertes\nLa source d alertes n est pas disponible.", nil
	}

	alerts, total, err := alertStore.ListAlerts(5, 0, true)
	if err != nil {
		return "", fmt.Errorf("alertes indisponibles: %w", err)
	}

	if total == 0 {
		return "✅ Alertes\nAucune alerte active.", nil
	}

	lines := []string{fmt.Sprintf("🚨 Alertes actives (%d)", total)}
	for i, alert := range alerts {
		lines = append(lines, fmt.Sprintf("%d. %s %s", i+1, telegramAlertPrefix(alert), alert.Message))
		lines = append(lines, fmt.Sprintf("   [%s/%s] %s", alert.Category, alert.Severity, alert.Key))
	}

	return strings.Join(lines, "\n"), nil
}

func (n *TelegramAlertNotifier) sendText(ctx context.Context, chatID, text string) error {
	if n == nil || n.bot == nil {
		return fmt.Errorf("telegram notifier is nil")
	}

	telegramChatID, err := telegramChatIDFromString(chatID)
	if err != nil {
		return err
	}

	msg := tu.Message(telegramChatID, text).WithReplyMarkup(telegramQuickReplyKeyboard())

	_, err = n.bot.SendMessage(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to send telegram alert: %s", n.redactSensitiveError(err))
	}

	return nil
}

func telegramQuickReplyKeyboard() *telego.ReplyKeyboardMarkup {
	keyboard := tu.Keyboard(
		tu.KeyboardRow(
			tu.KeyboardButton("wallet"),
			tu.KeyboardButton("opportunites"),
			tu.KeyboardButton("decision"),
		),
		tu.KeyboardRow(
			tu.KeyboardButton("health"),
			tu.KeyboardButton("alertes"),
		),
	).WithResizeKeyboard().WithInputFieldPlaceholder("Choisis une action rapide")

	return keyboard
}

func (n *TelegramAlertNotifier) getUpdates(ctx context.Context) ([]telego.Update, error) {
	if n == nil || n.bot == nil {
		return nil, fmt.Errorf("telegram notifier is nil")
	}

	params := &telego.GetUpdatesParams{
		Timeout: int(defaultTelegramLongPollTimeout / time.Second),
	}
	if n.nextUpdateID > 0 {
		params.Offset = int(n.nextUpdateID)
	}

	updates, err := n.bot.GetUpdates(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to poll telegram updates: %s", n.redactSensitiveError(err))
	}

	return updates, nil
}

func telegramChatIDFromString(value string) (telego.ChatID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return telego.ChatID{}, fmt.Errorf("telegram chat id is required")
	}

	if strings.HasPrefix(trimmed, "@") {
		return tu.Username(trimmed), nil
	}

	id, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return telego.ChatID{}, fmt.Errorf("invalid telegram chat id %q: %w", trimmed, err)
	}

	return tu.ID(id), nil
}

func (n *TelegramAlertNotifier) redactSensitiveError(err error) string {
	if err == nil {
		return ""
	}

	message := err.Error()
	if token := strings.TrimSpace(n.botToken); token != "" {
		message = strings.ReplaceAll(message, token, "[REDACTED]")
	}

	return message
}

func formatTelegramAlert(alert store.AlertEvent) string {
	statusLabel := telegramAlertState(alert)
	severityLabel := translateTelegramSeverity(alert.Severity)
	categoryLabel := translateTelegramCategory(alert.Category)
	sourceLabel := translateTelegramSource(alert.Source)
	seenLabel := formatTelegramAlertSeenRange(alert.FirstSeen, alert.LastSeen)
	actionLabel := telegramAlertActionHint(alert)

	lines := []string{
		fmt.Sprintf("%s Karasu %s", telegramAlertPrefix(alert), statusLabel),
		fmt.Sprintf("Niveau: %s", severityLabel),
		fmt.Sprintf("Type: %s", categoryLabel),
		fmt.Sprintf("Origine: %s", sourceLabel),
	}

	if symbol := strings.TrimSpace(alert.Symbol); symbol != "" {
		lines = append(lines, fmt.Sprintf("Symbole: %s", symbol))
	}

	lines = append(
		lines,
		fmt.Sprintf("Détail: %s", strings.TrimSpace(alert.Message)),
		fmt.Sprintf("Occurrences: %d", alert.Count),
		fmt.Sprintf("Période: %s", seenLabel),
		fmt.Sprintf("Action recommandée: %s", actionLabel),
	)

	return strings.Join(lines, "\n")
}

func formatTelegramAlertSeenRange(firstSeen, lastSeen time.Time) string {
	if firstSeen.IsZero() && lastSeen.IsZero() {
		return "inconnue"
	}

	if firstSeen.IsZero() {
		return fmt.Sprintf("dernier signal %s", formatTelegramAlertTime(lastSeen))
	}

	if lastSeen.IsZero() || lastSeen.Equal(firstSeen) {
		return fmt.Sprintf("signal unique à %s", formatTelegramAlertTime(firstSeen))
	}

	return fmt.Sprintf("%s -> %s", formatTelegramAlertTime(firstSeen), formatTelegramAlertTime(lastSeen))
}

func formatTelegramAlertTime(ts time.Time) string {
	if ts.IsZero() {
		return "inconnue"
	}
	return ts.UTC().Format("2006-01-02 15:04:05 UTC")
}

func translateTelegramSeverity(severity store.AlertSeverity) string {
	switch severity {
	case store.AlertSeverityCritical:
		return "critique"
	case store.AlertSeverityWarning:
		return "alerte"
	default:
		return "information"
	}
}

func translateTelegramCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "exchange":
		return "exchange"
	case "health":
		return "sante systeme"
	case "backfill":
		return "rattrapage historique"
	case "opportunity":
		return "opportunites OR"
	case "decision":
		return "decision portefeuille"
	default:
		if category == "" {
			return "non precise"
		}
		return category
	}
}

func translateTelegramSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "exchange":
		return "flux exchange"
	case "system-health":
		return "controle de sante"
	case "backfill-worker", "backfill":
		return "worker backfill"
	case "opportunity-engine":
		return "moteur opportunites"
	case "decision-engine":
		return "moteur decision"
	default:
		if source == "" {
			return "non precisee"
		}
		return source
	}
}

func telegramAlertActionHint(alert store.AlertEvent) string {
	if !alert.Active {
		return "aucune action urgente, surveiller recurrence"
	}

	category := strings.ToLower(strings.TrimSpace(alert.Category))
	if category == "exchange" {
		return "reduire la frequence d ingestion et verifier la connectivite exchange"
	}
	if category == "health" {
		return "verifier la fraicheur des donnees et l etat du scheduler"
	}
	if category == "backfill" {
		return "inspecter la queue de backfill et relancer un job si necessaire"
	}
	if category == "opportunity" {
		return "verifier /opportunities et prioriser les entrees fraiches"
	}
	if category == "decision" {
		return "ouvrir /decision et evaluer un allegement immediat"
	}

	if alert.Severity == store.AlertSeverityCritical {
		return "intervention recommandee rapidement"
	}
	if alert.Severity == store.AlertSeverityWarning {
		return "surveillance rapprochee"
	}

	return "information de contexte"
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

func telegramHelpMessage() string {
	return "Commandes disponibles:\n/wallet\n/opportunities\n/decision\n/health\n/alerts\n\n" + telegramQuickRepliesMessage()
}

func telegramQuickRepliesMessage() string {
	return "Réponses rapides:\n- wallet\n- opportunites\n- decision\n- health\n- alertes\n\nTu peux aussi utiliser /help"
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
	case "Pulse":
		return "Pulse"
	case "Balance":
		return "Balance"
	case "Trend":
		return "Trend"
	default:
		return label
	}
}

func translateTelegramProfileDescription(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return "profil sans description"
	}
	return description
}

func compactTelegramProfileHint(label, description string) string {
	switch strings.TrimSpace(label) {
	case "Pulse":
		return "court terme reactif"
	case "Balance":
		return "equilibre swing"
	case "Trend":
		return "suivi de tendance long terme"
	default:
		return translateTelegramProfileDescription(description)
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
