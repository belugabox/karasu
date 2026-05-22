package main

import (
	"context"
	"fmt"
	"karasu/internal/exchange"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"karasu/internal/scheduler"

	"github.com/dpotapov/slogpfx"
	"github.com/joho/godotenv"
	"github.com/phsym/console-slog"
)

func main() {
	// capture interrupt signals to allow for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// time
	time.Local = time.UTC

	// slog
	console := console.NewHandler(os.Stderr, &console.HandlerOptions{Level: slog.LevelDebug})
	consoleWithPrefix := slogpfx.NewHandler(console, &slogpfx.HandlerOptions{
		PrefixKeys: []string{"task"},
	})
	slog.SetDefault(slog.New(consoleWithPrefix))

	// load config
	if err := godotenv.Load(".env"); err != nil {
		slog.Error("no .env file found, relying on environment variables", "err", err)
		return
	}

	exchangeClient, err := exchange.NewBitvavoClient()
	if err != nil {
		slog.Error("failed to create exchange client", "err", err)
		return
	}

	store, err := NewCandleStore(os.Getenv("KARASU_DB_PATH"))
	if err != nil {
		slog.Error("failed to initialize candle store", "err", err)
		return
	}
	defer store.Close()

	ingestionService := NewIngestionService(exchangeClient, store)
	if err := ingestionService.RefreshUniverse(); err != nil {
		slog.Warn("initial universe refresh failed", "err", err)
	}

	refreshUniverseInterval, err := durationFromEnv("KARASU_REFRESH_UNIVERSE_INTERVAL", 15*time.Minute)
	if err != nil {
		slog.Error("invalid scheduler interval", "env", "KARASU_REFRESH_UNIVERSE_INTERVAL", "err", err)
		return
	}
	topSymbolsInterval, err := durationFromEnv("KARASU_INGEST_TOP_INTERVAL", 1*time.Minute)
	if err != nil {
		slog.Error("invalid scheduler interval", "env", "KARASU_INGEST_TOP_INTERVAL", "err", err)
		return
	}
	otherSymbolsInterval, err := durationFromEnv("KARASU_INGEST_OTHER_INTERVAL", 5*time.Minute)
	if err != nil {
		slog.Error("invalid scheduler interval", "env", "KARASU_INGEST_OTHER_INTERVAL", "err", err)
		return
	}
	repairLookback, err := durationFromEnv("KARASU_INGEST_REPAIR_LOOKBACK", 6*time.Hour)
	if err != nil {
		slog.Error("invalid ingestion setting", "env", "KARASU_INGEST_REPAIR_LOOKBACK", "err", err)
		return
	}
	backfillChunk, err := durationFromEnv("KARASU_BACKFILL_CHUNK", 12*time.Hour)
	if err != nil {
		slog.Error("invalid ingestion setting", "env", "KARASU_BACKFILL_CHUNK", "err", err)
		return
	}
	if err := ingestionService.SetRepairLookback(repairLookback); err != nil {
		slog.Error("failed to configure ingestion repair lookback", "err", err)
		return
	}
	if err := ingestionService.SetBackfillChunk(backfillChunk); err != nil {
		slog.Error("failed to configure backfill chunk", "err", err)
		return
	}

	// run scheduler
	s, err := scheduler.NewScheduler()
	if err != nil {
		slog.Error("failed to create scheduler", "err", err)
		return
	}
	if err := s.AddJob("refresh universe", refreshUniverseInterval, ingestionService.RefreshUniverse); err != nil {
		slog.Error("failed to add scheduler job", "job", "refresh universe", "err", err)
		return
	}
	if err := s.AddJob("ingest top symbols", topSymbolsInterval, ingestionService.IngestTopSymbols); err != nil {
		slog.Error("failed to add scheduler job", "job", "ingest top symbols", "err", err)
		return
	}
	if err := s.AddJob("ingest other symbols", otherSymbolsInterval, ingestionService.IngestOtherSymbols); err != nil {
		slog.Error("failed to add scheduler job", "job", "ingest other symbols", "err", err)
		return
	}
	slog.Info(
		"scheduler intervals configured",
		"refreshUniverse", refreshUniverseInterval.String(),
		"ingestTop", topSymbolsInterval.String(),
		"ingestOther", otherSymbolsInterval.String(),
		"repairLookback", repairLookback.String(),
		"backfillChunk", backfillChunk.String(),
	)
	defer s.Stop()
	s.Start()

	// run http server
	srv := newHTTPServer(os.Getenv("PORT"), newRouter(exchangeClient, ingestionService, store))

	slog.Info("services started", "url", "http://localhost"+srv.Addr)
	if err := runHTTPServer(ctx, srv); err != nil {
		slog.Error("application runtime error", "err", err)
		return
	}
}

func durationFromEnv(envName string, defaultValue time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return defaultValue, nil
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid Go duration (example: 30s, 1m, 5m): %w", envName, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be > 0", envName)
	}

	return parsed, nil
}
