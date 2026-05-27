package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"karasu/internal/exchange"

	"github.com/gin-gonic/gin"
)

func setupOrderRouter(client *fakeExchangeClient) *gin.Engine {
	r := gin.New()
	RegisterOrder(r, client)
	return r
}

func TestPlaceOrder_Buy_OK(t *testing.T) {
	client := &fakeExchangeClient{
		placeOrderResult: exchange.OrderResult{
			OrderID: "order-1",
			Market:  "BTC-EUR",
			Side:    "buy",
			Status:  "filled",
		},
	}
	r := setupOrderRouter(client)

	body, _ := json.Marshal(map[string]any{"symbol": "BTC", "side": "buy", "amountEur": 100.0})
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result exchange.OrderResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.OrderID != "order-1" {
		t.Errorf("expected orderId order-1, got %s", result.OrderID)
	}
	if result.Side != "buy" {
		t.Errorf("expected side buy, got %s", result.Side)
	}
}

func TestPlaceOrder_Sell_OK(t *testing.T) {
	client := &fakeExchangeClient{
		placeOrderResult: exchange.OrderResult{
			OrderID: "order-2",
			Market:  "ETH-EUR",
			Side:    "sell",
			Status:  "filled",
		},
	}
	r := setupOrderRouter(client)

	body, _ := json.Marshal(map[string]any{"symbol": "ETH", "side": "sell", "amountEur": 50.0})
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlaceOrder_InvalidSide(t *testing.T) {
	client := &fakeExchangeClient{}
	r := setupOrderRouter(client)

	body, _ := json.Marshal(map[string]any{"symbol": "BTC", "side": "hold", "amountEur": 100.0})
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPlaceOrder_MissingSymbol(t *testing.T) {
	client := &fakeExchangeClient{}
	r := setupOrderRouter(client)

	body, _ := json.Marshal(map[string]any{"side": "buy", "amountEur": 100.0})
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPlaceOrder_ZeroAmount(t *testing.T) {
	client := &fakeExchangeClient{}
	r := setupOrderRouter(client)

	body, _ := json.Marshal(map[string]any{"symbol": "BTC", "side": "buy", "amountEur": 0})
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPlaceOrder_ExchangeError(t *testing.T) {
	client := &fakeExchangeClient{
		placeOrderErr: fmt.Errorf("exchange error"),
	}
	r := setupOrderRouter(client)

	body, _ := json.Marshal(map[string]any{"symbol": "BTC", "side": "buy", "amountEur": 100.0})
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
