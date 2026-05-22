package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type appConfig struct {
	Port string
}

func loadAppConfig() appConfig {
	if err := godotenv.Load(".env"); err != nil {
		slog.Warn("no .env file found, relying on environment variables", "err", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return appConfig{Port: port}
}
