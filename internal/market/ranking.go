package market

import (
	"fmt"
	"karasu/internal/exchange"
	"math"
	"sort"
	"time"
)

// Market holds ranking data for a single tradable symbol.
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

// FindTopSymbols returns the top symbols by combined liquidity+momentum score.
func FindTopSymbols(exchangeClient exchange.ExchangeClient) ([]string, error) {
	markets := make([]Market, 0)

	candleBundles, err := exchangeClient.CandlesLast24h()
	if err != nil {
		return nil, fmt.Errorf("failed to get 24h candles: %w", err)
	}

	for _, candleBundle := range candleBundles {
		candle := candleBundle.Candles[0]

		lastPrice := candle.Close
		openPrice := candle.Open
		volume := candle.Volume

		quoteVolume := roundTo(lastPrice*volume, 0)
		if quoteVolume < 50000 {
			continue
		}

		change24h := ((lastPrice - openPrice) / openPrice) * 100

		markets = append(markets, Market{
			Symbol:              candleBundle.Symbol,
			QuoteVolume:         quoteVolume,
			Change24h:           roundTo(change24h, 2),
			QuoteVolumePosition: -1,
			Change24hPosition:   -1,
		})
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].QuoteVolume > markets[j].QuoteVolume
	})
	for i := range markets {
		markets[i].QuoteVolumePosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change24h > markets[j].Change24h
	})
	for i := range markets {
		markets[i].Change24hPosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return (markets[i].QuoteVolumePosition + markets[i].Change24hPosition) < (markets[j].QuoteVolumePosition + markets[j].Change24hPosition)
	})

	const maxMarkets = 30
	if len(markets) > maxMarkets {
		markets = markets[:maxMarkets]
	}

	symbols := make([]string, len(markets))
	for i, market := range markets {
		symbols[i] = market.Symbol
	}

	return symbols, nil
}

// TopMarketPositions returns the full ranked market list with 1h and 5m changes.
func TopMarketPositions(exchangeClient exchange.ExchangeClient) ([]Market, error) {
	markets := make([]Market, 0)

	candleBundles, err := exchangeClient.CandlesLast24h()
	if err != nil {
		return nil, fmt.Errorf("failed to get 24h candles: %w", err)
	}

	for _, candleBundle := range candleBundles {
		candle := candleBundle.Candles[0]

		lastPrice := candle.Close
		openPrice := candle.Open
		volume := candle.Volume

		quoteVolume := roundTo(lastPrice*volume, 0)
		if quoteVolume < 50000 {
			continue
		}

		change24h := ((lastPrice - openPrice) / openPrice) * 100

		markets = append(markets, Market{
			Symbol:              candleBundle.Symbol,
			QuoteVolume:         quoteVolume,
			Change24h:           roundTo(change24h, 2),
			QuoteVolumePosition: -1,
			Change24hPosition:   -1,
		})
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].QuoteVolume > markets[j].QuoteVolume
	})
	for i := range markets {
		markets[i].QuoteVolumePosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change24h > markets[j].Change24h
	})
	for i := range markets {
		markets[i].Change24hPosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return (markets[i].QuoteVolumePosition + markets[i].Change24hPosition) < (markets[j].QuoteVolumePosition + markets[j].Change24hPosition)
	})

	const maxMarkets = 30
	if len(markets) > maxMarkets {
		markets = markets[:maxMarkets]
	}

	for i := range markets {
		candleBundle1m, err := exchangeClient.Candles1mByDate(markets[i].Symbol, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to get 1m candles for %s: %w", markets[i].Symbol, err)
		}
		candles := candleBundle1m.Candles

		if len(candles) >= 60 {
			openPrice := candles[59].Open
			lastPrice := candles[0].Close
			markets[i].Change1h = roundTo(((lastPrice-openPrice)/openPrice)*100, 2)
		}
		if len(candles) >= 5 {
			openPrice := candles[4].Open
			lastPrice := candles[0].Close
			markets[i].Change5m = roundTo(((lastPrice-openPrice)/openPrice)*100, 2)
		}
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change1h > markets[j].Change1h
	})
	for i := range markets {
		markets[i].Change1hPosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change5m > markets[j].Change5m
	})
	for i := range markets {
		markets[i].Change5mPosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change1hPosition < markets[j].Change1hPosition
	})

	return markets, nil
}

func roundTo(value float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(value*factor) / factor
}
