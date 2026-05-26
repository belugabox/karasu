package market

import (
	"fmt"
	"karasu/internal/exchange"
	"karasu/internal/scoring"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Market holds ranking data for a single tradable symbol.
type Market struct {
	Symbol string

	QuoteVolume         float64 // volume in quote currency (e.g., EUR)
	QuoteVolumePosition int     // position in the ranking by quote volume
	QualityScore        float64 // technical momentum/quality score from 0 to 100
	QualityRSI          float64 // RSI contribution input score from 0 to 100
	QualityMACD         float64 // MACD contribution input score from 0 to 100
	QualityBollinger    float64 // Bollinger contribution input score from 0 to 100
	QualityVolume       float64 // Volume contribution input score from 0 to 100
	QualitySMA          float64 // SMA contribution input score from 0 to 100

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
	var (
		prioritySymbols map[string]struct{}
		candleBundles   []exchange.CandleBundle
		errWallet       error
		errCandles      error
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		prioritySymbols, errWallet = walletPrioritySymbols(exchangeClient)
	}()
	go func() {
		defer wg.Done()
		candleBundles, errCandles = exchangeClient.CandlesLast24h()
	}()
	wg.Wait()

	if errWallet != nil {
		return nil, fmt.Errorf("failed to load wallet priority symbols: %w", errWallet)
	}
	if errCandles != nil {
		return nil, fmt.Errorf("failed to get 24h candles: %w", errCandles)
	}

	for _, candleBundle := range candleBundles {
		candle := candleBundle.Candles[0]

		lastPrice := candle.Close
		openPrice := candle.Open
		volume := candle.Volume

		quoteVolume := roundTo(lastPrice*volume, 0)
		_, forceInclude := prioritySymbols[strings.ToUpper(candleBundle.Symbol)]
		if quoteVolume < 50000 && !forceInclude {
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
	markets = trimMarketsWithPriority(markets, prioritySymbols, maxMarkets)

	symbols := make([]string, len(markets))
	for i, market := range markets {
		symbols[i] = market.Symbol
	}

	return symbols, nil
}

// TopMarketPositions returns the full ranked market list with 1h and 5m changes.
func TopMarketPositions(exchangeClient exchange.ExchangeClient) ([]Market, error) {
	markets := make([]Market, 0)
	var (
		prioritySymbols map[string]struct{}
		candleBundles   []exchange.CandleBundle
		errWallet       error
		errCandles      error
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		prioritySymbols, errWallet = walletPrioritySymbols(exchangeClient)
	}()
	go func() {
		defer wg.Done()
		candleBundles, errCandles = exchangeClient.CandlesLast24h()
	}()
	wg.Wait()

	if errWallet != nil {
		return nil, fmt.Errorf("failed to load wallet priority symbols: %w", errWallet)
	}
	if errCandles != nil {
		return nil, fmt.Errorf("failed to get 24h candles: %w", errCandles)
	}

	for _, candleBundle := range candleBundles {
		candle := candleBundle.Candles[0]

		lastPrice := candle.Close
		openPrice := candle.Open
		volume := candle.Volume

		quoteVolume := roundTo(lastPrice*volume, 0)
		_, forceInclude := prioritySymbols[strings.ToUpper(candleBundle.Symbol)]
		if quoteVolume < 50000 && !forceInclude {
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
	markets = trimMarketsWithPriority(markets, prioritySymbols, maxMarkets)

	if err := computeShortTermChangesParallel(exchangeClient, markets); err != nil {
		return nil, err
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

func walletPrioritySymbols(exchangeClient exchange.ExchangeClient) (map[string]struct{}, error) {
	wallet, err := exchangeClient.Wallet()
	if err != nil {
		return nil, err
	}

	const minWalletValueEUR = 0.009
	priority := make(map[string]struct{})
	for _, asset := range wallet.Assets {
		symbol := strings.ToUpper(strings.TrimSpace(asset.Symbol))
		if symbol == "" || symbol == "EUR" {
			continue
		}
		if asset.Value > minWalletValueEUR {
			priority[symbol] = struct{}{}
		}
	}

	return priority, nil
}

func trimMarketsWithPriority(markets []Market, prioritySymbols map[string]struct{}, maxMarkets int) []Market {
	if len(markets) <= maxMarkets {
		return markets
	}

	selected := make([]Market, 0, maxMarkets)
	added := make(map[string]struct{}, maxMarkets)

	for _, market := range markets {
		if len(selected) >= maxMarkets {
			break
		}
		symbol := strings.ToUpper(strings.TrimSpace(market.Symbol))
		if _, wanted := prioritySymbols[symbol]; !wanted {
			continue
		}
		if _, exists := added[symbol]; exists {
			continue
		}
		selected = append(selected, market)
		added[symbol] = struct{}{}
	}

	for _, market := range markets {
		if len(selected) >= maxMarkets {
			break
		}
		symbol := strings.ToUpper(strings.TrimSpace(market.Symbol))
		if _, exists := added[symbol]; exists {
			continue
		}
		selected = append(selected, market)
		added[symbol] = struct{}{}
	}

	return selected
}

func computeShortTermChangesParallel(exchangeClient exchange.ExchangeClient, markets []Market) error {
	if len(markets) == 0 {
		return nil
	}

	const workerCount = 8
	jobs := make(chan int)
	var wg sync.WaitGroup

	var firstErr error
	var errMu sync.Mutex

	now := time.Now()

	worker := func() {
		defer wg.Done()
		for i := range jobs {
			candleBundle1m, err := exchangeClient.Candles1mByDate(markets[i].Symbol, now)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to get 1m candles for %s: %w", markets[i].Symbol, err)
				}
				errMu.Unlock()
				continue
			}

			candles := candleBundle1m.Candles
			quality := computeQualityBreakdownFromExchangeCandles(candles)
			markets[i].QualityScore = roundTo(quality.Score, 2)
			markets[i].QualityRSI = roundTo(quality.RSI, 2)
			markets[i].QualityMACD = roundTo(quality.MACD, 2)
			markets[i].QualityBollinger = roundTo(quality.Bollinger, 2)
			markets[i].QualityVolume = roundTo(quality.Volume, 2)
			markets[i].QualitySMA = roundTo(quality.SMA, 2)
			if change1h, ok := computeWindowChange(candles, time.Hour); ok {
				markets[i].Change1h = roundTo(change1h, 2)
			}
			if change5m, ok := computeWindowChange(candles, 5*time.Minute); ok {
				markets[i].Change5m = roundTo(change5m, 2)
			}
		}
	}

	w := workerCount
	if len(markets) < w {
		w = len(markets)
	}
	for i := 0; i < w; i++ {
		wg.Add(1)
		go worker()
	}

	for i := range markets {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	return nil
}

func computeQualityBreakdownFromExchangeCandles(candles []exchange.Candle) scoring.QualityBreakdown {
	if len(candles) == 0 {
		return scoring.QualityBreakdown{}
	}

	sorted := make([]exchange.Candle, 0, len(candles))
	for _, candle := range candles {
		if candle.Open <= 0 || candle.High <= 0 || candle.Low <= 0 || candle.Close <= 0 {
			continue
		}
		sorted = append(sorted, candle)
	}
	if len(sorted) == 0 {
		return scoring.QualityBreakdown{}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].OpenTime.Before(sorted[j].OpenTime)
	})

	scoringCandles := make([]scoring.Candle, 0, len(sorted))
	for _, candle := range sorted {
		scoringCandles = append(scoringCandles, scoring.Candle{
			Open:      candle.Open,
			High:      candle.High,
			Low:       candle.Low,
			Close:     candle.Close,
			Volume:    candle.Volume,
			OpenTime:  candle.OpenTime,
			CloseTime: candle.CloseTime,
		})
	}

	return scoring.ComputeQualityBreakdown(scoringCandles, "short")
}

func computeWindowChange(candles []exchange.Candle, window time.Duration) (float64, bool) {
	if len(candles) < 2 {
		return 0, false
	}

	sorted := make([]exchange.Candle, 0, len(candles))
	for _, c := range candles {
		if c.Open <= 0 || c.Close <= 0 || c.OpenTime.IsZero() {
			continue
		}
		sorted = append(sorted, c)
	}
	if len(sorted) < 2 {
		return 0, false
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].OpenTime.Before(sorted[j].OpenTime)
	})

	latest := sorted[len(sorted)-1]
	latestTime := latest.CloseTime
	if latestTime.IsZero() {
		latestTime = latest.OpenTime.Add(time.Minute)
	}
	targetTime := latestTime.Add(-window)

	refIdx := -1
	for i := len(sorted) - 1; i >= 0; i-- {
		if !sorted[i].OpenTime.After(targetTime) {
			refIdx = i
			break
		}
	}
	if refIdx < 0 {
		return 0, false
	}

	ref := sorted[refIdx]
	if ref.Open <= 0 || latest.Close <= 0 {
		return 0, false
	}

	actualSpan := latestTime.Sub(ref.OpenTime)
	minSpan := window - 2*time.Minute
	if window >= time.Hour {
		minSpan = window - 10*time.Minute
	}
	if actualSpan < minSpan {
		return 0, false
	}

	change := ((latest.Close - ref.Open) / ref.Open) * 100
	return change, true
}
