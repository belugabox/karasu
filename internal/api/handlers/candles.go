package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"karasu/internal/ingestion"
	"karasu/internal/store"

	"github.com/gin-gonic/gin"
)

func RegisterCandles(r *gin.Engine, ingestionService *ingestion.IngestionService, candleStore store.CandleStore) {
	r.GET("/api/live-1m", func(c *gin.Context) {
		rawSymbols := strings.TrimSpace(c.Query("symbols"))
		symbols := make([]string, 0)
		if rawSymbols != "" {
			for _, symbol := range strings.Split(rawSymbols, ",") {
				symbol = strings.TrimSpace(symbol)
				if symbol == "" {
					continue
				}
				symbols = append(symbols, strings.ToUpper(symbol))
			}
		}

		limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}

		candles := ingestionService.LiveCandles(symbols, limit)
		c.JSON(http.StatusOK, gin.H{
			"count":   len(candles),
			"candles": candles,
		})
	})

	r.GET("/api/candles-5m", func(c *gin.Context) {
		symbol := strings.ToUpper(strings.TrimSpace(c.Query("symbol")))
		if symbol == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
			return
		}

		limit, err := strconv.Atoi(c.DefaultQuery("limit", "500"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}

		candles, err := candleStore.QueryCandles("bitvavo", symbol, "5m", limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"symbol":    symbol,
			"timeframe": "5m",
			"count":     len(candles),
			"candles":   candles,
		})
	})
}
