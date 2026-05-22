package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// run scheduler
	scheduler, err := newScheduler()
	if err != nil {
		slog.Error("failed to create scheduler", "err", err)
		return
	}
	addJob(scheduler, "ETL Task", 60*time.Second, launcheETL)
	defer scheduler.Shutdown()
	scheduler.Start()

	// run http server
	srv := newHTTPServer(os.Getenv("PORT"), newRouter())

	slog.Info("services started", "url", "http://localhost"+srv.Addr)
	if err := runHTTPServer(ctx, srv); err != nil {
		slog.Error("application runtime error", "err", err)
		return
	}
}
