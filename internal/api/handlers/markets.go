package handlers

import (
	"log/slog"
	"net/http"

	"karasu/internal/exchange"
	"karasu/internal/market"

	"github.com/gin-gonic/gin"
)

func RegisterMarkets(r *gin.Engine, exchangeClient exchange.ExchangeClient) {
	r.GET("/api/markets", func(c *gin.Context) {
		markets, err := market.TopMarketPositions(exchangeClient)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		slog.Info("top market positions", "count", len(markets))
		c.JSON(http.StatusOK, markets)
	})
}
