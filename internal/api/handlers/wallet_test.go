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

func newWalletRouter(ec *fakeExchangeClient) *gin.Engine {
	r := gin.New()
	RegisterWallet(r, ec)
	return r
}

// TestWalletEndpointReturnsWalletPayloadShape verifies GET /api/wallet
// returns a wallet object with the expected payload fields.
func TestWalletEndpointReturnsWalletPayloadShape(t *testing.T) {
	t.Parallel()

	ec := &fakeExchangeClient{
		wallet: exchange.Wallet{
			TotalValue:       5000,
			CashValue:        1000,
			AssetValue:       4000,
			NetDepositsValue: 3000,
			Assets: []exchange.WalletAsset{
				{Symbol: "BTC", Amount: 0.05, Value: 4000},
			},
		},
	}
	r := newWalletRouter(ec)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/wallet", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}

	for _, field := range []string{"TotalValue", "CashValue", "AssetValue", "NetDepositsValue", "Assets"} {
		if _, ok := body[field]; !ok {
			t.Errorf("expected field %q in wallet payload, not found", field)
		}
	}

	assets, ok := body["Assets"].([]interface{})
	if !ok {
		t.Fatal("expected assets to be an array")
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	asset, ok := assets[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected asset to be an object")
	}
	for _, field := range []string{"Symbol", "Amount", "Value"} {
		if _, ok := asset[field]; !ok {
			t.Errorf("expected field %q in asset payload, not found", field)
		}
	}
}

// TestWalletEndpointReturns500OnExchangeError verifies GET /api/wallet
// returns 500 when the exchange client fails.
func TestWalletEndpointReturns500OnExchangeError(t *testing.T) {
	t.Parallel()

	ec := &fakeExchangeClient{walletErr: errors.New("exchange unavailable")}
	r := newWalletRouter(ec)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/wallet", nil)
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

// TestWalletEndpointReturnsEmptyAssetsWhenNoPositions verifies GET /api/wallet
// returns an empty assets array when the wallet has no positions.
func TestWalletEndpointReturnsEmptyAssetsWhenNoPositions(t *testing.T) {
	t.Parallel()

	ec := &fakeExchangeClient{
		wallet: exchange.Wallet{
			TotalValue: 500,
			CashValue:  500,
			Assets:     []exchange.WalletAsset{},
		},
	}
	r := newWalletRouter(ec)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/wallet", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	assets, ok := body["Assets"].([]interface{})
	if !ok {
		t.Fatal("expected assets to be an array")
	}
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets, got %d", len(assets))
	}
}
