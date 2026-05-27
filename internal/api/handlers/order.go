package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"karasu/internal/exchange"

	"github.com/gin-gonic/gin"
)

type placeOrderRequest struct {
	Symbol    string  `json:"symbol" binding:"required"`
	Side      string  `json:"side" binding:"required"`
	AmountEUR float64 `json:"amountEur" binding:"required,gt=0"`
}

func RegisterOrder(r *gin.Engine, exchangeClient exchange.ExchangeClient) {
	r.POST("/api/orders", func(c *gin.Context) {
		var req placeOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))
		req.Side = strings.ToLower(strings.TrimSpace(req.Side))

		if req.Side != "buy" && req.Side != "sell" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "side must be buy or sell"})
			return
		}

		if req.Symbol == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
			return
		}

		result, err := exchangeClient.PlaceMarketOrder(req.Symbol, req.Side, req.AmountEUR)
		if err != nil {
			slog.Error("failed to place order", "symbol", req.Symbol, "side", req.Side, "amountEur", req.AmountEUR, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		slog.Info("order placed", "symbol", req.Symbol, "side", req.Side, "amountEur", req.AmountEUR, "orderId", result.OrderID)
		c.JSON(http.StatusOK, result)
	})
}
