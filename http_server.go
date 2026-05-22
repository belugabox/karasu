package main

import (
	"context"
	"errors"
	"fmt"
	"karasu/exchange"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

func newRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	router.Use(sloggin.NewWithConfig(slog.Default(), sloggin.Config{
		WithSpanID:  true,
		WithTraceID: true,
	}))

	router.GET("/ping", func(c *gin.Context) {
		exchangeClient, err := exchange.NewBitvavoClient()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		markets, err := TopMarketPositions(exchangeClient)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		slog.Info("top market positions", "markets", markets)
		c.JSON(http.StatusOK, markets)
	})
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
