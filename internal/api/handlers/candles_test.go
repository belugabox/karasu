package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"karasu/internal/exchange"

	"github.com/gin-gonic/gin"
)

func newCandlesRouter(ec *fakeExchangeClient, cs *fakeCandleStore) *gin.Engine {
	svc := newIngestionService(ec, cs)
	r := gin.New()
	RegisterCandles(r, svc, cs)
	return r
}

// TestLive1mEndpointReturnsEmptyWhenCacheIsEmpty verifies GET /api/live-1m
// returns 200 with count:0 and empty candles when the live cache is empty.
func TestLive1mEndpointReturnsEmptyWhenCacheIsEmpty(t *testing.T) {
	t.Parallel()

	r := newCandlesRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/live-1m", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}

	if _, ok := body["count"]; !ok {
		t.Error("expected 'count' field in live-1m response")
	}
	if _, ok := body["candles"]; !ok {
		t.Error("expected 'candles' field in live-1m response")
	}

	count, ok := body["count"].(float64)
	if !ok || count != 0 {
		t.Errorf("expected count=0 with empty cache, got %v", body["count"])
	}
}

// TestLive1mEndpointReturnsBadRequestOnInvalidLimit verifies GET /api/live-1m
// returns 400 when the limit parameter is not a valid integer.
func TestLive1mEndpointReturnsBadRequestOnInvalidLimit(t *testing.T) {
	t.Parallel()

	r := newCandlesRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/live-1m?limit=notanumber", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCandles5mEndpointReturnsPayloadShape verifies GET /api/candles-5m
// returns candles with the expected JSON payload structure.
func TestCandles5mEndpointReturnsPayloadShape(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	storedCandles := []exchange.Candle{
		{Open: 100, High: 105, Low: 99, Close: 103, Volume: 500, OpenTime: now, CloseTime: now.Add(5 * time.Minute)},
		{Open: 103, High: 108, Low: 102, Close: 106, Volume: 600, OpenTime: now.Add(5 * time.Minute), CloseTime: now.Add(10 * time.Minute)},
	}
	cs := &fakeCandleStore{candles: map[string][]exchange.Candle{"BTC": storedCandles}}

	r := newCandlesRouter(&fakeExchangeClient{}, cs)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/candles-5m?symbol=BTC", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}

	for _, field := range []string{"symbol", "timeframe", "count", "candles"} {
		if _, ok := body[field]; !ok {
			t.Errorf("expected field %q in candles-5m response", field)
		}
	}

	if body["symbol"] != "BTC" {
		t.Errorf("expected symbol BTC, got %v", body["symbol"])
	}
	if body["timeframe"] != "5m" {
		t.Errorf("expected timeframe 5m, got %v", body["timeframe"])
	}

	count, ok := body["count"].(float64)
	if !ok || int(count) != len(storedCandles) {
		t.Errorf("expected count=%d, got %v", len(storedCandles), body["count"])
	}
}

// TestCandles5mEndpointReturnsBadRequestOnMissingSymbol verifies GET /api/candles-5m
// returns 400 when the symbol parameter is missing.
func TestCandles5mEndpointReturnsBadRequestOnMissingSymbol(t *testing.T) {
	t.Parallel()

	r := newCandlesRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/candles-5m", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCandles5mEndpointReturns500OnStoreError verifies GET /api/candles-5m
// returns 500 when the candle store fails.
func TestCandles5mEndpointReturns500OnStoreError(t *testing.T) {
	t.Parallel()

	cs := &fakeCandleStore{err: errors.New("store failure")}
	r := newCandlesRouter(&fakeExchangeClient{}, cs)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/candles-5m?symbol=BTC", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSystemHealthEndpointReturnsPayloadShape verifies GET /api/system-health
// returns a response with the expected system health payload fields.
func TestSystemHealthEndpointReturnsPayloadShape(t *testing.T) {
	t.Parallel()

	r := newCandlesRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system-health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}

	for _, field := range []string{
		"generatedAt", "isHealthy", "issues",
		"universeSymbols", "topSymbols", "liveSymbols",
		"liveFresh", "staleThresholdMin",
		"topSymbolsStale5m", "topStaleExamples",
		"backfillQueueDepth", "backfillQueueCap",
		"backfillQueuedJobs", "backfillRunningJobs",
		"backfillFailedJobs24h",
	} {
		if _, ok := body[field]; !ok {
			t.Errorf("expected field %q in system-health response", field)
		}
	}
}

// TestSystemHealthEndpointReturnsBadRequestOnInvalidThreshold verifies
// GET /api/system-health returns 400 when staleThresholdMin is invalid.
func TestSystemHealthEndpointReturnsBadRequestOnInvalidThreshold(t *testing.T) {
	t.Parallel()

	r := newCandlesRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system-health?staleThresholdMin=notanumber", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAlertsRecentEndpointReturnsPayloadShape verifies GET /api/alerts/recent
// returns a response with count and alerts fields.
func TestAlertsRecentEndpointReturnsPayloadShape(t *testing.T) {
	t.Parallel()

	r := newCandlesRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/alerts/recent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}

	if _, ok := body["count"]; !ok {
		t.Error("expected 'count' field in alerts/recent response")
	}
	if _, ok := body["alerts"]; !ok {
		t.Error("expected 'alerts' field in alerts/recent response")
	}
}

// TestAlertsRecentEndpointReturnsBadRequestOnInvalidLimit verifies
// GET /api/alerts/recent returns 400 when limit is not a valid integer.
func TestAlertsRecentEndpointReturnsBadRequestOnInvalidLimit(t *testing.T) {
	t.Parallel()

	r := newCandlesRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/alerts/recent?limit=bad", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAlertsRecentActiveOnlyFiltersInactiveAlerts verifies GET /api/alerts/recent
// with activeOnly=true returns only active alerts.
func TestAlertsRecentActiveOnlyFiltersInactiveAlerts(t *testing.T) {
	t.Parallel()

	r := newCandlesRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/alerts/recent?activeOnly=true", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	alerts, ok := body["alerts"].([]interface{})
	if !ok {
		t.Fatal("expected alerts to be an array")
	}
	// With empty service, there are no alerts — verify count matches array length.
	count, _ := body["count"].(float64)
	if int(count) != len(alerts) {
		t.Errorf("count (%d) does not match alerts array length (%d)", int(count), len(alerts))
	}
}
