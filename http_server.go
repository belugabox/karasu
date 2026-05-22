package main

import (
	"context"
	"errors"
	"fmt"
	"karasu/internal/exchange"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

func newRouter(exchangeClient exchange.ExchangeClient, ingestionService *IngestionService, store *CandleStore) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	router.Use(sloggin.NewWithConfig(slog.Default(), sloggin.Config{
		DefaultLevel: slog.LevelDebug,
	}))

	router.GET("/api/markets", func(c *gin.Context) {
		markets, err := TopMarketPositions(exchangeClient)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		slog.Info("top market positions", "markets", markets)
		c.JSON(http.StatusOK, markets)
	})

	router.GET("/api/live-1m", func(c *gin.Context) {
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

	router.GET("/api/candles-5m", func(c *gin.Context) {
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

		candles, err := store.QueryCandles("bitvavo", symbol, "5m", limit)
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

	router.POST("/api/backfill-5m", func(c *gin.Context) {
		rawSymbols := strings.TrimSpace(c.Query("symbols"))
		symbols := make([]string, 0)
		if rawSymbols != "" {
			for _, symbol := range strings.Split(rawSymbols, ",") {
				symbol = strings.ToUpper(strings.TrimSpace(symbol))
				if symbol == "" {
					continue
				}
				symbols = append(symbols, symbol)
			}
		}

		from, err := parseTimeQuery(c, "from")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		to, err := parseTimeQuery(c, "to")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		report, err := ingestionService.Backfill5m(symbols, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  err.Error(),
				"report": report,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"report": report,
		})
	})

	// Serve the embedded React/Vite frontend for all non-API routes
	router.NoRoute(gin.WrapH(spaFileServer()))

	return router
}

func newHTTPServer(port string, handler http.Handler) *http.Server {
	if port == "" {
		port = "8080"
	}
	return &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}
}

func runHTTPServer(ctx context.Context, srv *http.Server) error {
	go func() {
		<-ctx.Done()
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Warn("failed to shutdown http server cleanly", "err", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server failed: %w", err)
	}
	return nil
}

func parseTimeQuery(c *gin.Context, key string) (time.Time, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}, fmt.Errorf("%s is required (RFC3339 or unix milliseconds)", key)
	}

	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.UnixMilli(ms).UTC(), nil
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s: use RFC3339 (example 2026-05-01T00:00:00Z) or unix milliseconds", key)
	}

	return parsed.UTC(), nil
}
