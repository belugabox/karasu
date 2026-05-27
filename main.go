package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	karasuapi "karasu/internal/api"
	"karasu/internal/exchange"
	"karasu/internal/ingestion"
	"karasu/internal/notification"
	"karasu/internal/scheduler"
	"karasu/internal/store"

	"github.com/dpotapov/slogpfx"
	"github.com/joho/godotenv"
	console "github.com/phsym/console-slog"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	time.Local = time.UTC

	consoleHandler := console.NewHandler(os.Stderr, &console.HandlerOptions{Level: slog.LevelDebug})
	consoleWithPrefix := slogpfx.NewHandler(consoleHandler, &slogpfx.HandlerOptions{
		PrefixKeys: []string{"task"},
	})
	slog.SetDefault(slog.New(consoleWithPrefix))

	if err := godotenv.Load(".env"); err != nil {
		slog.Warn("no .env file found, relying on environment variables", "err", err)
	}

	exchangeClient, err := exchange.NewBitvavoClient()
	if err != nil {
		slog.Error("failed to create exchange client", "err", err)
		return
	}

	candleStore, err := store.NewSQLiteStore(os.Getenv("KARASU_DB_PATH"))
	if err != nil {
		slog.Error("failed to initialize candle store", "err", err)
		return
	}
	defer candleStore.Close()

	ingestionService := ingestion.NewIngestionService(exchangeClient, candleStore)
	ingestionService.SetAlertStore(candleStore)
	telegramAlertsEnabled, err := boolFromEnv("KARASU_TELEGRAM_ALERTS_ENABLED", false)
	if err != nil {
		slog.Error("invalid telegram setting", "env", "KARASU_TELEGRAM_ALERTS_ENABLED", "err", err)
		return
	}
	if telegramNotifier, err := telegramNotifierFromEnv(); err != nil {
		slog.Error("failed to configure telegram notifier", "err", err)
		return
	} else if telegramNotifier != nil {
		telegramNotifier.SetCommandSources(exchangeClient, candleStore)
		if telegramAlertsEnabled {
			ingestionService.SetAlertNotifier(telegramNotifier)
			slog.Info("telegram alert forwarding enabled")
		} else {
			slog.Info("telegram alert forwarding disabled", "env", "KARASU_TELEGRAM_ALERTS_ENABLED")
		}
		go telegramNotifier.Run(ctx)
		slog.Info("telegram bot commands enabled")
	}
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
	gapRepairInterval, err := durationFromEnv("KARASU_GAP_REPAIR_INTERVAL", 3*time.Minute)
	if err != nil {
		slog.Error("invalid scheduler interval", "env", "KARASU_GAP_REPAIR_INTERVAL", "err", err)
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
	alertORMinScore, err := floatFromEnv("KARASU_ALERT_OPPORTUNITY_OR_MIN_SCORE", 70)
	if err != nil {
		slog.Error("invalid alert setting", "env", "KARASU_ALERT_OPPORTUNITY_OR_MIN_SCORE", "err", err)
		return
	}
	alertUrgentMinReduce, err := intFromEnv("KARASU_ALERT_DECISION_URGENT_MIN_REDUCE", 1)
	if err != nil {
		slog.Error("invalid alert setting", "env", "KARASU_ALERT_DECISION_URGENT_MIN_REDUCE", "err", err)
		return
	}
	alertNotifyCooldown, err := durationFromEnv("KARASU_ALERT_NOTIFY_COOLDOWN", 15*time.Minute)
	if err != nil {
		slog.Error("invalid alert setting", "env", "KARASU_ALERT_NOTIFY_COOLDOWN", "err", err)
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
	if err := ingestionService.SetOpportunityAlertMinScore(alertORMinScore); err != nil {
		slog.Error("failed to configure opportunity OR alert threshold", "err", err)
		return
	}
	if err := ingestionService.SetUrgentDecisionMinReduce(alertUrgentMinReduce); err != nil {
		slog.Error("failed to configure urgent decision alert threshold", "err", err)
		return
	}
	if err := ingestionService.SetAlertNotifyCooldown(alertNotifyCooldown); err != nil {
		slog.Error("failed to configure alert notify cooldown", "err", err)
		return
	}

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
	if err := s.AddJob("repair gaps", gapRepairInterval, ingestionService.RepairDetectedGaps); err != nil {
		slog.Error("failed to add scheduler job", "job", "repair gaps", "err", err)
		return
	}
	slog.Info(
		"scheduler intervals configured",
		"refreshUniverse", refreshUniverseInterval.String(),
		"ingestTop", topSymbolsInterval.String(),
		"ingestOther", otherSymbolsInterval.String(),
		"repairGaps", gapRepairInterval.String(),
		"repairLookback", repairLookback.String(),
		"backfillChunk", backfillChunk.String(),
		"alertORMinScore", alertORMinScore,
		"alertUrgentMinReduce", alertUrgentMinReduce,
		"alertNotifyCooldown", alertNotifyCooldown.String(),
	)
	defer s.Stop()
	s.Start()

	router := karasuapi.NewRouter(exchangeClient, ingestionService, candleStore, spaFileServer())
	srv := karasuapi.NewHTTPServer(os.Getenv("PORT"), router)

	slog.Info("services started", "url", "http://localhost"+srv.Addr)
	if err := karasuapi.RunHTTPServer(ctx, srv); err != nil {
		slog.Error("application runtime error", "err", err)
		return
	}
}

func floatFromEnv(envName string, defaultValue float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid number: %w", envName, err)
	}

	return parsed, nil
}

func intFromEnv(envName string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", envName, err)
	}

	return parsed, nil
}

func boolFromEnv(envName string, defaultValue bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean (true/false): %w", envName, err)
	}

	return parsed, nil
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

func telegramNotifierFromEnv() (*notification.TelegramAlertNotifier, error) {
	botToken := strings.TrimSpace(os.Getenv("KARASU_TELEGRAM_BOT_TOKEN"))
	chatID := strings.TrimSpace(os.Getenv("KARASU_TELEGRAM_CHAT_ID"))

	if botToken == "" && chatID == "" {
		return nil, nil
	}

	return notification.NewTelegramAlertNotifier(botToken, chatID)
}
