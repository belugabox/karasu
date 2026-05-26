package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"karasu/internal/exchange"
	"karasu/internal/market"
	"karasu/internal/store"

	"github.com/gin-gonic/gin"
)

func RegisterMarkets(r *gin.Engine, exchangeClient exchange.ExchangeClient, candleStore store.CandleStore) {
	r.GET("/api/markets", func(c *gin.Context) {
		markets, err := market.TopMarketPositions(exchangeClient)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		slog.Info("top market positions", "count", len(markets))
		c.JSON(http.StatusOK, markets)
	})

	r.GET("/api/opportunities", func(c *gin.Context) {
		limit, err := strconv.Atoi(c.DefaultQuery("limit", "15"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}

		opportunities, err := market.TopOpportunities(exchangeClient, candleStore, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		slog.Info("top opportunities", "count", len(opportunities), "limit", limit)
		c.JSON(http.StatusOK, opportunities)
	})

	r.GET("/api/markets/:symbol/analysis", func(c *gin.Context) {
		symbol := strings.ToUpper(strings.TrimSpace(c.Param("symbol")))
		analysis, err := market.AnalyzeSymbol(exchangeClient, symbol)
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "symbol is required") || strings.Contains(err.Error(), "not found") {
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, analysis)
	})

	r.GET("/api/markets/:symbol/signals", func(c *gin.Context) {
		symbol := strings.ToUpper(strings.TrimSpace(c.Param("symbol")))
		if symbol == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
			return
		}

		limit, err := strconv.Atoi(c.DefaultQuery("limit", "40"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}

		candles, err := candleStore.QueryCandles("bitvavo", symbol, "5m", 500)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		history := market.BuildSignalHistory(symbol, "5m", candles, limit)
		c.JSON(http.StatusOK, history)
	})
}
