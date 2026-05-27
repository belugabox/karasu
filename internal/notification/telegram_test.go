package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"karasu/internal/exchange"
	"karasu/internal/store"

	"github.com/mymmrac/telego"
)

const telegramValidTestToken = "123456:ABCDEFGHIJKLMNOPQRSTUVWXYZabcde1234"

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
		ChatID json.RawMessage `json:"chat_id"`
		Text   string          `json:"text"`
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

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":456,"type":"private"},"text":"ok"}}`)
	}))
	defer server.Close()

	notifier, err := NewTelegramAlertNotifier(telegramValidTestToken, "456")
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}
	bot, err := telego.NewBot(
		telegramValidTestToken,
		telego.WithHTTPClient(server.Client()),
		telego.WithAPIServer(server.URL),
	)
	if err != nil {
		t.Fatalf("failed to create telego bot: %v", err)
	}
	notifier.bot = bot

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

	if gotPath != "/bot"+telegramValidTestToken+"/sendMessage" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("unexpected content-type %q", gotContentType)
	}
	if strings.TrimSpace(string(payload.ChatID)) != "456" {
		t.Fatalf("unexpected chat_id %s", string(payload.ChatID))
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

func TestTelegramAlertNotifierPollOnceRepliesToSupportedCommands(t *testing.T) {
	t.Parallel()

	sentMessages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot" + telegramValidTestToken + "/getUpdates":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true,"result":[{"update_id":11,"message":{"chat":{"id":456},"text":"/wallet"}},{"update_id":12,"message":{"chat":{"id":456},"text":"/opportunities"}},{"update_id":13,"message":{"chat":{"id":456},"text":"/decision"}}]}`)
		case "/bot" + telegramValidTestToken + "/sendMessage":
			var payload struct {
				Text string `json:"text"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode sendMessage payload: %v", err)
			}
			sentMessages = append(sentMessages, payload.Text)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":2,"date":0,"chat":{"id":456,"type":"private"},"text":"ok"}}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	notifier, err := NewTelegramAlertNotifier(telegramValidTestToken, "456")
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}
	bot, err := telego.NewBot(
		telegramValidTestToken,
		telego.WithHTTPClient(server.Client()),
		telego.WithAPIServer(server.URL),
	)
	if err != nil {
		t.Fatalf("failed to create telego bot: %v", err)
	}
	notifier.bot = bot
	notifier.SetCommandSources(
		telegramTestExchangeClient{
			wallet: exchange.Wallet{
				TotalValue: 5000,
				CashValue:  1000,
				AssetValue: 4000,
				PnLValue:   500,
				PnLPercent: 11.11,
				Assets: []exchange.WalletAsset{
					{Symbol: "BTC", Value: 2500, PnLPercent: 12.5},
					{Symbol: "ETH", Value: 1500, PnLPercent: -3.2},
				},
			},
			candlesLast24h: []exchange.CandleBundle{
				telegramBuild24hBundle("BTC", 100, 115, 1200),
				telegramBuild24hBundle("ETH", 100, 108, 1100),
			},
			candles1m: map[string]exchange.CandleBundle{
				"BTC": {Symbol: "BTC", Interval: exchange.Interval1m, Candles: telegramBuildExchangeCandles(90, 200, 1.2, 1200, 10)},
				"ETH": {Symbol: "ETH", Interval: exchange.Interval1m, Candles: telegramBuildExchangeCandles(90, 180, 0.8, 1100, 8)},
			},
		},
		telegramTestCandleStore{
			candles: map[string][]exchange.Candle{
				"BTC": telegramBuildExchangeCandles(60, 150, 1.1, 1000, 8),
				"ETH": telegramBuildExchangeCandles(60, 120, 0.7, 900, 7),
			},
		},
	)

	if err := notifier.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce returned error: %v", err)
	}
	if len(sentMessages) != 3 {
		t.Fatalf("expected 3 replies, got %d", len(sentMessages))
	}
	if !strings.Contains(sentMessages[0], "💼 Portefeuille") {
		t.Fatalf("expected wallet response, got %q", sentMessages[0])
	}
	if !strings.Contains(sentMessages[1], "🎯 Opportunités prioritaires") {
		t.Fatalf("expected opportunities response, got %q", sentMessages[1])
	}
	if !strings.Contains(sentMessages[2], "🧭 Décision portefeuille") {
		t.Fatalf("expected decision response, got %q", sentMessages[2])
	}
}

func TestTelegramAlertNotifierRedactsTokenFromHTTPClientErrors(t *testing.T) {
	t.Parallel()

	notifier, err := NewTelegramAlertNotifier(telegramValidTestToken, "456")
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}
	bot, err := telego.NewBot(
		telegramValidTestToken,
		telego.WithHTTPClient(&http.Client{Transport: telegramErrorTransport{}}),
	)
	if err != nil {
		t.Fatalf("failed to create telego bot: %v", err)
	}
	notifier.bot = bot

	sendErr := notifier.sendText(context.Background(), "456", "hello")
	if sendErr == nil {
		t.Fatal("expected sendText to return an error")
	}
	if strings.Contains(sendErr.Error(), telegramValidTestToken) {
		t.Fatalf("expected sendText error to redact bot token, got %q", sendErr)
	}

	_, pollErr := notifier.getUpdates(context.Background())
	if pollErr == nil {
		t.Fatal("expected getUpdates to return an error")
	}
	if strings.Contains(pollErr.Error(), telegramValidTestToken) {
		t.Fatalf("expected getUpdates error to redact bot token, got %q", pollErr)
	}
}

type telegramTestExchangeClient struct {
	wallet         exchange.Wallet
	candlesLast24h []exchange.CandleBundle
	candles1m      map[string]exchange.CandleBundle
}

func (f telegramTestExchangeClient) Symbols() (map[string]string, error) {
	return map[string]string{}, nil
}

func (f telegramTestExchangeClient) Prices() (map[string]float64, error) {
	return map[string]float64{}, nil
}

func (f telegramTestExchangeClient) Wallet() (exchange.Wallet, error) {
	return f.wallet, nil
}

func (f telegramTestExchangeClient) Candles1mByDate(symbol string, date time.Time) (exchange.CandleBundle, error) {
	return f.candles1m[symbol], nil
}

func (f telegramTestExchangeClient) Candles5mByDate(symbol string, date time.Time) (exchange.CandleBundle, error) {
	return exchange.CandleBundle{}, nil
}

func (f telegramTestExchangeClient) CandlesLast24h() ([]exchange.CandleBundle, error) {
	return f.candlesLast24h, nil
}

func (f telegramTestExchangeClient) Candles(symbol string, start time.Time, end time.Time, interval exchange.Interval) (exchange.CandleBundle, error) {
	return exchange.CandleBundle{}, nil
}

type telegramErrorTransport struct{}

func (telegramErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, &url.Error{
		Op:  req.Method,
		URL: req.URL.String(),
		Err: context.DeadlineExceeded,
	}
}

type telegramTestCandleStore struct {
	candles map[string][]exchange.Candle
}

func (f telegramTestCandleStore) UpsertCandles(exchangeName, symbol, timeframe string, candles []exchange.Candle) error {
	return nil
}

func (f telegramTestCandleStore) QueryCandles(exchangeName, symbol, timeframe string, limit int) ([]exchange.Candle, error) {
	result := append([]exchange.Candle(nil), f.candles[symbol]...)
	if limit > 0 && len(result) > limit {
		return result[len(result)-limit:], nil
	}
	return result, nil
}

func (f telegramTestCandleStore) LastCandleOpenTime(exchangeName, symbol, timeframe string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (f telegramTestCandleStore) QueryDailySymbolActivity(exchangeName, timeframe string, days int) ([]store.DailySymbolActivity, error) {
	return nil, nil
}

func (f telegramTestCandleStore) Close() error {
	return nil
}

func telegramBuild24hBundle(symbol string, open, close, volume float64) exchange.CandleBundle {
	now := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	return exchange.CandleBundle{
		Symbol:   symbol,
		Interval: exchange.Interval1d,
		Candles: []exchange.Candle{{
			Open:      open,
			High:      close * 1.01,
			Low:       open * 0.99,
			Close:     close,
			Volume:    volume,
			OpenTime:  now,
			CloseTime: now.Add(24 * time.Hour),
		}},
	}
}

func telegramBuildExchangeCandles(count int, startPrice, priceStep, baseVolume, volumeStep float64) []exchange.Candle {
	candles := make([]exchange.Candle, 0, count)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < count; i++ {
		open := startPrice + float64(i)*priceStep
		close := open + priceStep*0.6
		openTime := start.Add(time.Duration(i) * time.Minute)
		candles = append(candles, exchange.Candle{
			Open:      open,
			High:      close + 0.4,
			Low:       open - 0.4,
			Close:     close,
			Volume:    baseVolume + float64(i)*volumeStep,
			OpenTime:  openTime,
			CloseTime: openTime.Add(time.Minute),
		})
	}

	return candles
}
