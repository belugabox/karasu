package main

import (
	"fmt"
	"karasu/exchange"
	"math"
	"sort"
	"time"
)

type Market struct {
	Symbol string

	QuoteVolume         float64 // volume in quote currency (e.g., EUR)
	QuoteVolumePosition int     // position in the ranking by quote volume

	Change24h         float64 // percentage change over 24h
	Change24hPosition int     // position in the ranking by 24h change

	Change1h         float64 // percentage change over 1h
	Change1hPosition int     // position in the ranking by 1h change

	Change5m         float64 // percentage change over 5m
	Change5mPosition int     // position in the ranking by 5m change
}

func TopMarketPositions(exchangeClient exchange.ExchangeClient) ([]Market, error) {
	markets := make([]Market, 0)

	// On récupére les 24h candles pour tous les marchés
	candleBundles, err := exchangeClient.CandlesLast24h()
	if err != nil {
		return nil, fmt.Errorf("failed to get 24h candles: %w", err)
	}

	// Pour chaque marché
	for _, candleBundle := range candleBundles {
		candle := candleBundle.Candles[0] // on suppose qu'il n'y a qu'une seule bougie par marché dans les 24h candles

		// On récupère les prix d'ouverture et de clôture, ainsi que le volume
		lastPrice := candle.Close
		openPrice := candle.Open
		volume := candle.Volume

		// On calcule le volume en quote currency (e.g., EUR) pour filtrer les marchés peu liquides, et on arrondit à 2 décimales pour éviter les problèmes de précision
		quoteVolume := roundTo(lastPrice*volume, 0)

		// On filtre les marchés avec un volume en quote currency inférieur à 50k EUR
		if quoteVolume < 50000 {
			continue
		}

		// On calcule la variation en pourcentage sur 24h pour trier les marchés
		change24h := ((lastPrice - openPrice) / openPrice) * 100

		// On ajoute le marché à la liste des marchés à surveiller
		markets = append(markets, Market{
			Symbol:              candleBundle.Symbol,
			QuoteVolume:         quoteVolume,
			Change24h:           roundTo(change24h, 2),
			QuoteVolumePosition: -1,
			Change24hPosition:   -1,
		})
	}

	// On trie les marchés par volume en quote currency (du plus liquide au moins liquide)
	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].QuoteVolume > markets[j].QuoteVolume
	})
	// On assigne les positions dans le classement par volume en quote currency
	for i := range markets {
		markets[i].QuoteVolumePosition = i + 1
	}

	// On trie les marchés par variation sur 24h (du plus haussier au plus baissier)
	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change24h > markets[j].Change24h
	})
	// On assigne les positions dans le classement par variation sur 24h
	for i := range markets {
		markets[i].Change24hPosition = i + 1
	}

	// On trie les marchés par la somme de leurs positions dans les deux classements (volume en quote currency et variation sur 24h)
	sort.SliceStable(markets, func(i, j int) bool {
		return (markets[i].QuoteVolumePosition + markets[i].Change24hPosition) < (markets[j].QuoteVolumePosition + markets[j].Change24hPosition)
	})

	// On ne garde que les maxMarkets premiers marchés pour la suite de l'analyse
	const maxMarkets = 10
	if len(markets) > maxMarkets {
		markets = markets[:maxMarkets]
	}

	// Pour chaque marché, on calcule son evolution sur la dernière heure pour trier les marchés les plus volatils sur la dernière heure
	for i := range markets {
		candleBundle1m, err := exchangeClient.Candles1mByDate(markets[i].Symbol, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to get 1m candles for %s: %w", markets[i].Symbol, err)
		}

		candleBundle1h, err := exchange.AggregateTo(candleBundle1m, exchange.Interval1h)
		if err != nil {
			return nil, fmt.Errorf("failed to aggregate 1h candles for %s: %w", markets[i].Symbol, err)
		}
		// Calcul de l'évolution sur la dernière heure
		if len(candleBundle1h.Candles) > 0 {
			openPrice := candleBundle1h.Candles[0].Open
			lastPrice := candleBundle1h.Candles[len(candleBundle1h.Candles)-1].Close
			markets[i].Change1h = roundTo(((lastPrice-openPrice)/openPrice)*100, 2)
		}

		candleBundle5m, err := exchange.Aggregate1mTo(candleBundle1m, exchange.Interval5m)
		if err != nil {
			return nil, fmt.Errorf("failed to aggregate 5m candles for %s: %w", markets[i].Symbol, err)
		}
		// Calcul de l'évolution sur les 5 dernières minutes
		if len(candleBundle5m.Candles) > 0 {
			openPrice := candleBundle5m.Candles[0].Open
			lastPrice := candleBundle5m.Candles[len(candleBundle5m.Candles)-1].Close
			markets[i].Change5m = roundTo(((lastPrice-openPrice)/openPrice)*100, 2)
		}
	}
	// On trie les marchés par variation sur 1h (du plus haussier au plus baissier)
	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change1h > markets[j].Change1h
	})
	// On assigne les positions dans le classement par variation sur 1h
	for i := range markets {
		markets[i].Change1hPosition = i + 1
	}

	// On trie les marchés par variation sur 5m (du plus haussier au plus baissier)
	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change5m > markets[j].Change5m
	})
	// On assigne les positions dans le classement par variation sur 5m
	for i := range markets {
		markets[i].Change5mPosition = i + 1
	}

	// On trie les marchés par la somme de leurs positions dans les trois classements (volume en quote currency, variation sur 24h et variation sur 1h)
	sort.SliceStable(markets, func(i, j int) bool {
		return (markets[i].QuoteVolumePosition + markets[i].Change24hPosition + markets[i].Change1hPosition + markets[i].Change5mPosition) < (markets[j].QuoteVolumePosition + markets[j].Change24hPosition + markets[j].Change1hPosition + markets[j].Change5mPosition)
	})

	// On retourne la liste des marchés à surveiller
	return markets, nil
}

func roundTo(value float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(value*factor) / factor
}
