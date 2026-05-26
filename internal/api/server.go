package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"karasu/internal/api/handlers"
	"karasu/internal/exchange"
	"karasu/internal/ingestion"
	"karasu/internal/store"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

// NewRouter wires all API handlers. spaHandler is the fallback for non-API routes (embedded frontend).
func NewRouter(
	exchangeClient exchange.ExchangeClient,
	ingestionService *ingestion.IngestionService,
	candleStore store.CandleStore,
	spaHandler http.Handler,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(sloggin.NewWithConfig(slog.Default(), sloggin.Config{
		DefaultLevel: slog.LevelDebug,
	}))

	handlers.RegisterMarkets(router, exchangeClient, candleStore)
	handlers.RegisterCandles(router, ingestionService, candleStore)
	handlers.RegisterBackfill(router, ingestionService)
	handlers.RegisterWallet(router, exchangeClient)

	router.NoRoute(gin.WrapH(spaHandler))

	return router
}

func NewHTTPServer(port string, handler http.Handler) *http.Server {
	if port == "" {
		port = "8080"
	}
	return &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}
}

func RunHTTPServer(ctx context.Context, srv *http.Server) error {
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
