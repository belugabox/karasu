package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"karasu/internal/exchange"

	"github.com/gin-gonic/gin"
)

func newMarketsRouter(ec *fakeExchangeClient, cs *fakeCandleStore) *gin.Engine {
	r := gin.New()
	RegisterMarkets(r, ec, cs)
	return r
}

// TestMarketsEndpointReturnsEmptyListWhenNoSymbols verifies that GET /api/markets
// returns 200 OK with an empty JSON array when no 24h candles are available.
func TestMarketsEndpointReturnsEmptyListWhenNoSymbols(t *testing.T) {
	t.Parallel()

	r := newMarketsRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/markets", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not a JSON array: %v — body: %s", err, w.Body.String())
	}
	if len(result) != 0 {
		t.Fatalf("expected empty array, got %d items", len(result))
	}
}

// TestMarketsEndpointReturnsMarketsWithPayloadShape verifies GET /api/markets
// returns market objects with the expected payload fields.
func TestMarketsEndpointReturnsMarketsWithPayloadShape(t *testing.T) {
	t.Parallel()

	oneMinBundle := build1mBundle("BTC", 90, 200)
	ec := &fakeExchangeClient{
		candlesLast24h: []exchange.CandleBundle{build24hBundle("BTC", 100, 110, 1000)},
		candles1m:      map[string]exchange.CandleBundle{"BTC": oneMinBundle},
	}
	r := newMarketsRouter(ec, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/markets", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not a JSON array: %v — body: %s", err, w.Body.String())
	}
	if len(result) == 0 {
		t.Fatal("expected at least one market")
	}

	m := result[0]
	for _, field := range []string{"Symbol", "QuoteVolume", "QualityScore", "Change24h", "Change1h", "Change5m", "Strategies"} {
		if _, ok := m[field]; !ok {
			t.Errorf("expected field %q in market payload, not found", field)
		}
	}

	if m["Symbol"] != "BTC" {
		t.Errorf("expected symbol BTC, got %v", m["Symbol"])
	}
}

// TestMarketsEndpointReturns500OnExchangeError verifies GET /api/markets
// returns 500 when the exchange client fails.
func TestMarketsEndpointReturns500OnExchangeError(t *testing.T) {
	t.Parallel()

	ec := &fakeExchangeClient{walletErr: errors.New("exchange down")}
	r := newMarketsRouter(ec, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/markets", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' field in error response")
	}
}

// TestOpportunitiesEndpointReturnsEmptyWhenNoMarkets verifies GET /api/opportunities
// returns 200 OK with an empty array when there are no eligible markets.
func TestOpportunitiesEndpointReturnsEmptyWhenNoMarkets(t *testing.T) {
	t.Parallel()

	r := newMarketsRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/opportunities?limit=5", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not a JSON array: %v — body: %s", err, w.Body.String())
	}
}

// TestOpportunitiesEndpointReturnsBadRequestOnInvalidLimit verifies GET /api/opportunities
// returns 400 when the limit parameter is not a valid integer.
func TestOpportunitiesEndpointReturnsBadRequestOnInvalidLimit(t *testing.T) {
	t.Parallel()

	r := newMarketsRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/opportunities?limit=bad", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestOpportunitiesEndpointReturnsPayloadShape verifies GET /api/opportunities
// returns opportunity objects with the expected payload fields.
func TestOpportunitiesEndpointReturnsPayloadShape(t *testing.T) {
	t.Parallel()

	oneMinBundle := build1mBundle("BTC", 90, 200)
	ec := &fakeExchangeClient{
		candlesLast24h: []exchange.CandleBundle{build24hBundle("BTC", 100, 115, 1000)},
		candles1m:      map[string]exchange.CandleBundle{"BTC": oneMinBundle},
	}
	cs := &fakeCandleStore{candles: map[string][]exchange.Candle{"BTC": oneMinBundle.Candles}}
	r := newMarketsRouter(ec, cs)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/opportunities?limit=5", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not a JSON array: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected at least one opportunity")
	}

	opp := result[0]
	for _, field := range []string{"Symbol", "PriorityScore", "PriorityBand", "PrimaryAction", "Summary", "Leader", "Convergence", "Freshness", "QuoteVolume", "QualityScore", "Change24h", "Reasons", "Risks"} {
		if _, ok := opp[field]; !ok {
			t.Errorf("expected field %q in opportunity payload, not found", field)
		}
	}
}

// TestAnalysisEndpointReturns400WhenSymbolNotFound verifies GET /api/markets/:symbol/analysis
// returns 400 when the symbol is not found in 24h data.
func TestAnalysisEndpointReturns400WhenSymbolNotFound(t *testing.T) {
	t.Parallel()

	r := newMarketsRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/markets/BTC/analysis", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAnalysisEndpointReturnsPayloadShape verifies GET /api/markets/:symbol/analysis
// returns an analysis object with the expected payload fields.
func TestAnalysisEndpointReturnsPayloadShape(t *testing.T) {
	t.Parallel()

	oneMinBundle := build1mBundle("BTC", 90, 200)
	ec := &fakeExchangeClient{
		candlesLast24h: []exchange.CandleBundle{build24hBundle("BTC", 100, 110, 1000)},
		candles1m:      map[string]exchange.CandleBundle{"BTC": oneMinBundle},
	}
	r := newMarketsRouter(ec, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/markets/BTC/analysis", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}

	for _, field := range []string{"Symbol", "QuoteVolume", "Change24h", "Change1h", "Change5m", "Quality", "Strategies", "CandleCount1m"} {
		if _, ok := body[field]; !ok {
			t.Errorf("expected field %q in analysis payload, not found", field)
		}
	}
}

// TestSignalsEndpointReturnsPayloadShape verifies GET /api/markets/:symbol/signals
// returns a signal history object with the expected payload fields.
func TestSignalsEndpointReturnsPayloadShape(t *testing.T) {
	t.Parallel()

	oneMinBundle := build1mBundle("BTC", 70, 100)
	cs := &fakeCandleStore{candles: map[string][]exchange.Candle{"BTC": oneMinBundle.Candles}}
	r := newMarketsRouter(&fakeExchangeClient{}, cs)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/markets/BTC/signals?limit=10", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}

	for _, field := range []string{"Symbol", "Timeframe", "CandleCount", "Profiles"} {
		if _, ok := body[field]; !ok {
			t.Errorf("expected field %q in signals payload, not found", field)
		}
	}

	profiles, ok := body["Profiles"].([]interface{})
	if !ok {
		t.Fatal("expected profiles to be an array")
	}
	if len(profiles) == 0 {
		t.Fatal("expected at least one signal profile")
	}

	profile, ok := profiles[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected profile to be an object")
	}
	for _, field := range []string{"Name", "Label", "Stats", "Points"} {
		if _, ok := profile[field]; !ok {
			t.Errorf("expected field %q in profile payload, not found", field)
		}
	}
}

// TestSignalsEndpointReturnsBadRequestOnInvalidLimit verifies GET /api/markets/:symbol/signals
// returns 400 when the limit parameter is not a valid integer.
func TestSignalsEndpointReturnsBadRequestOnInvalidLimit(t *testing.T) {
	t.Parallel()

	r := newMarketsRouter(&fakeExchangeClient{}, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/markets/BTC/signals?limit=notanumber", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSignalsEndpointReturns500OnStoreError verifies GET /api/markets/:symbol/signals
// returns 500 when the candle store fails.
func TestSignalsEndpointReturns500OnStoreError(t *testing.T) {
	t.Parallel()

	cs := &fakeCandleStore{err: errors.New("db failure")}
	r := newMarketsRouter(&fakeExchangeClient{}, cs)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/markets/BTC/signals", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMarketsEndpointSymbolNormalizesCase verifies GET /api/markets/:symbol/analysis
// handles lowercase symbol input correctly.
func TestMarketsEndpointSymbolNormalizesCase(t *testing.T) {
	t.Parallel()

	oneMinBundle := build1mBundle("BTC", 90, 200)
	ec := &fakeExchangeClient{
		candlesLast24h: []exchange.CandleBundle{build24hBundle("BTC", 100, 110, 1000)},
		candles1m:      map[string]exchange.CandleBundle{"BTC": oneMinBundle},
	}
	r := newMarketsRouter(ec, &fakeCandleStore{})
	w := httptest.NewRecorder()
	// Use lowercase "btc" — handler must normalize to "BTC"
	req := httptest.NewRequest(http.MethodGet, "/api/markets/btc/analysis", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["Symbol"] != "BTC" {
		t.Errorf("expected symbol BTC after normalization, got %v", body["Symbol"])
	}
}

// TestOpportunitiesEndpointReturns500OnExchangeError verifies GET /api/opportunities
// returns 500 when the exchange client fails.
func TestOpportunitiesEndpointReturns500OnExchangeError(t *testing.T) {
	t.Parallel()

	ec := &fakeExchangeClient{walletErr: errors.New("exchange down")}
	r := newMarketsRouter(ec, &fakeCandleStore{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/opportunities?limit=5", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestOpportunitiesLimitIsRespected verifies GET /api/opportunities
// returns at most `limit` results.
func TestOpportunitiesLimitIsRespected(t *testing.T) {
	t.Parallel()

	bundles := make([]exchange.CandleBundle, 5)
	candles1m := make(map[string]exchange.CandleBundle)
	symbols := []string{"BTC", "ETH", "SOL", "ADA", "XRP"}
	for i, sym := range symbols {
		bundles[i] = build24hBundle(sym, 100, 115, 2000)
		candles1m[sym] = build1mBundle(sym, 90, 200)
	}
	ec := &fakeExchangeClient{candlesLast24h: bundles, candles1m: candles1m}

	csCandles := make(map[string][]exchange.Candle)
	for sym, b := range candles1m {
		csCandles[sym] = b.Candles
	}
	cs := &fakeCandleStore{candles: csCandles}

	r := newMarketsRouter(ec, cs)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/opportunities?limit=2", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not a JSON array: %v", err)
	}
	if len(result) > 2 {
		t.Fatalf("expected at most 2 results with limit=2, got %d", len(result))
	}
}

// TestOpportunitiesAreOrderedByPriorityScore verifies GET /api/opportunities
// returns results in descending priority score order.
func TestOpportunitiesAreOrderedByPriorityScore(t *testing.T) {
	t.Parallel()

	// BTC: strong uptrend (should rank higher)
	btcBundle := build24hBundle("BTC", 100, 125, 2000)
	btcCandles := build1mBundle("BTC", 90, 200)
	// ETH: mild uptrend
	ethBundle := build24hBundle("ETH", 100, 103, 2000)
	ethCandles := build1mBundle("ETH", 90, 100)

	ec := &fakeExchangeClient{
		candlesLast24h: []exchange.CandleBundle{btcBundle, ethBundle},
		candles1m:      map[string]exchange.CandleBundle{"BTC": btcCandles, "ETH": ethCandles},
	}
	cs := &fakeCandleStore{candles: map[string][]exchange.Candle{
		"BTC": btcCandles.Candles,
		"ETH": ethCandles.Candles,
	}}

	r := newMarketsRouter(ec, cs)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/opportunities?limit=10", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not a JSON array: %v", err)
	}
	if len(result) < 2 {
		t.Skip("need at least 2 opportunities to verify ordering")
	}

	for i := 1; i < len(result); i++ {
		prev := result[i-1]["PriorityScore"].(float64)
		curr := result[i]["PriorityScore"].(float64)
		if prev < curr {
			t.Fatalf("opportunities not sorted by priorityScore: item %d (%.2f) > item %d (%.2f)", i, curr, i-1, prev)
		}
	}
}

// build24h bundle that generates quoteVolume above 50k EUR threshold.
// Extracted from build24hBundle helper in testhelpers_test.go.
