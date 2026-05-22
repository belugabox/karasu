package main

import (
	"fmt"
	"karasu/exchange"
	"log/slog"
)

func launcheETL() error {
	// Étape 1 : Extraction
	exchangeClient, err := exchange.NewBitvavoClient()
	if err != nil {
		return fmt.Errorf("failed to create Bitvavo client: %w", err)
	}

	// Étape 2 : Transformation
	markets, err := TopMarketPositions(exchangeClient)
	if err != nil {
		return fmt.Errorf("failed to get top market positions: %w", err)
	}

	// Étape 3 : Chargement
	for _, market := range markets {
		slog.Info("top market position", "market", market)
	}

	return nil
}
