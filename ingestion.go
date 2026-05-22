package main

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"karasu/internal/exchange"
)

const (
	defaultTopSymbolsCount = 30
	defaultOtherBatchSize  = 80
	defaultRepairLookback  = 6 * time.Hour
	defaultBackfillChunk   = 12 * time.Hour
)

type LiveCandle struct {
	Symbol    string    `json:"symbol"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	OpenTime  time.Time `json:"openTime"`
	CloseTime time.Time `json:"closeTime"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type liveEntry struct {
	candle    exchange.Candle
	updatedAt time.Time
}

type LiveCache struct {
	mu     sync.RWMutex
	latest map[string]liveEntry
}

func NewLiveCache() *LiveCache {
	return &LiveCache{latest: make(map[string]liveEntry)}
}

func (c *LiveCache) Set(symbol string, candle exchange.Candle) {
	c.mu.Lock()
	c.latest[symbol] = liveEntry{
		candle:    candle,
		updatedAt: time.Now().UTC(),
	}
	c.mu.Unlock()
}

func (c *LiveCache) Snapshot(symbols []string, limit int) []LiveCandle {
	if limit <= 0 {
		limit = 50
	}
	if limit > 2000 {
		limit = 2000
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	res := make([]LiveCandle, 0, min(limit, len(c.latest)))
	if len(symbols) > 0 {
		for _, symbol := range symbols {
			entry, ok := c.latest[symbol]
			if !ok {
				continue
			}
			res = append(res, toLiveCandle(symbol, entry))
			if len(res) >= limit {
				break
			}
		}
		return res
	}

	sortedSymbols := make([]string, 0, len(c.latest))
	for symbol := range c.latest {
		sortedSymbols = append(sortedSymbols, symbol)
	}
	sort.Strings(sortedSymbols)

	for _, symbol := range sortedSymbols {
		res = append(res, toLiveCandle(symbol, c.latest[symbol]))
		if len(res) >= limit {
			break
		}
	}
	return res
}

func toLiveCandle(symbol string, entry liveEntry) LiveCandle {
	return LiveCandle{
		Symbol:    symbol,
		Open:      entry.candle.Open,
		High:      entry.candle.High,
		Low:       entry.candle.Low,
		Close:     entry.candle.Close,
		Volume:    entry.candle.Volume,
		OpenTime:  entry.candle.OpenTime,
		CloseTime: entry.candle.CloseTime,
		UpdatedAt: entry.updatedAt,
	}
}

type IngestionService struct {
	exchangeClient exchange.ExchangeClient
	store          *CandleStore
	liveCache      *LiveCache
	otherBatchSize int
	topSymbolsSize int
	repairLookback time.Duration
	backfillChunk  time.Duration
	backfillMu     sync.Mutex

	mu          sync.RWMutex
	allSymbols  []string
	topSymbols  []string
	otherCursor int
}

func NewIngestionService(exchangeClient exchange.ExchangeClient, store *CandleStore) *IngestionService {
	return &IngestionService{
		exchangeClient: exchangeClient,
		store:          store,
		liveCache:      NewLiveCache(),
		otherBatchSize: defaultOtherBatchSize,
		topSymbolsSize: defaultTopSymbolsCount,
		repairLookback: defaultRepairLookback,
		backfillChunk:  defaultBackfillChunk,
		allSymbols:     make([]string, 0),
		topSymbols:     make([]string, 0),
	}
}

func (s *IngestionService) SetRepairLookback(duration time.Duration) error {
	if duration <= 0 {
		return fmt.Errorf("repair lookback must be > 0")
	}

	s.mu.Lock()
	s.repairLookback = duration
	s.mu.Unlock()
	return nil
}

func (s *IngestionService) SetBackfillChunk(duration time.Duration) error {
	if duration <= 0 {
		return fmt.Errorf("backfill chunk must be > 0")
	}

	s.mu.Lock()
	s.backfillChunk = duration
	s.mu.Unlock()
	return nil
}

type BackfillReport struct {
	Symbols          int   `json:"symbols"`
	Chunks           int   `json:"chunks"`
	Fetched1mCandles int   `json:"fetched1mCandles"`
	Persisted5m      int   `json:"persisted5m"`
	DurationMs       int64 `json:"durationMs"`
}

func (s *IngestionService) Backfill5m(symbols []string, from, to time.Time) (BackfillReport, error) {
	startedAt := time.Now()
	from = from.UTC().Truncate(time.Minute)
	to = to.UTC().Truncate(time.Minute)

	if !from.Before(to) {
		return BackfillReport{}, fmt.Errorf("from must be before to")
	}

	if len(symbols) == 0 {
		symbols = s.getAllSymbols()
		if len(symbols) == 0 {
			if err := s.RefreshUniverse(); err != nil {
				return BackfillReport{}, err
			}
			symbols = s.getAllSymbols()
		}
	}
	if len(symbols) == 0 {
		return BackfillReport{}, fmt.Errorf("no symbols available for backfill")
	}

	s.backfillMu.Lock()
	defer s.backfillMu.Unlock()

	report := BackfillReport{Symbols: len(symbols)}
	chunk := s.backfillChunk

	var firstErr error
	for _, symbol := range symbols {
		cursor := from
		for cursor.Before(to) {
			chunkEnd := cursor.Add(chunk)
			if chunkEnd.After(to) {
				chunkEnd = to
			}

			bundle, err := s.exchangeClient.Candles(symbol, cursor, chunkEnd, exchange.Interval1m)
			if err != nil {
				slog.Warn("backfill fetch failed", "symbol", symbol, "from", cursor, "to", chunkEnd, "err", err)
				if firstErr == nil {
					firstErr = err
				}
				cursor = chunkEnd
				continue
			}

			report.Chunks++
			report.Fetched1mCandles += len(bundle.Candles)

			if len(bundle.Candles) > 0 {
				agg, err := exchange.Aggregate1mTo(bundle, exchange.Interval5m)
				if err != nil {
					slog.Warn("backfill aggregate failed", "symbol", symbol, "from", cursor, "to", chunkEnd, "err", err)
					if firstErr == nil {
						firstErr = err
					}
					cursor = chunkEnd
					continue
				}

				closedFloor := chunkEnd.Truncate(5 * time.Minute)
				candles5m := filterClosedCandles(agg.Candles, closedFloor)
				if len(candles5m) > 0 {
					if err := s.store.UpsertCandles("bitvavo", symbol, "5m", candles5m); err != nil {
						slog.Warn("backfill persist failed", "symbol", symbol, "count", len(candles5m), "err", err)
						if firstErr == nil {
							firstErr = err
						}
					} else {
						report.Persisted5m += len(candles5m)
					}
				}
			}

			cursor = chunkEnd
		}
	}

	report.DurationMs = time.Since(startedAt).Milliseconds()
	slog.Info(
		"backfill completed",
		"symbols", report.Symbols,
		"chunks", report.Chunks,
		"fetched1m", report.Fetched1mCandles,
		"persisted5m", report.Persisted5m,
		"durationMs", report.DurationMs,
	)

	if firstErr != nil {
		return report, fmt.Errorf("backfill completed with errors: %w", firstErr)
	}

	return report, nil
}

func (s *IngestionService) RefreshUniverse() error {
	bundles, err := s.exchangeClient.CandlesLast24h()
	if err != nil {
		return fmt.Errorf("failed to refresh universe: %w", err)
	}

	symbols := make([]string, 0, len(bundles))
	for _, bundle := range bundles {
		if strings.TrimSpace(bundle.Symbol) == "" {
			continue
		}
		symbols = append(symbols, strings.ToUpper(bundle.Symbol))
	}
	symbols = uniqueSymbols(symbols)

	topSymbols, err := findTopSymbols(s.exchangeClient)
	if err != nil {
		return fmt.Errorf("failed to compute top symbols: %w", err)
	}
	for i := range topSymbols {
		topSymbols[i] = strings.ToUpper(topSymbols[i])
	}
	topSymbols = uniqueSymbols(topSymbols)
	if len(topSymbols) > s.topSymbolsSize {
		topSymbols = topSymbols[:s.topSymbolsSize]
	}

	s.mu.Lock()
	s.allSymbols = symbols
	s.topSymbols = topSymbols
	s.otherCursor = 0
	s.mu.Unlock()

	slog.Info("universe refreshed", "symbols", len(symbols), "topSymbols", len(topSymbols))
	return nil
}

func (s *IngestionService) IngestTopSymbols() error {
	symbols := s.getTopSymbols()
	if len(symbols) == 0 {
		if err := s.RefreshUniverse(); err != nil {
			return err
		}
		symbols = s.getTopSymbols()
	}
	if len(symbols) == 0 {
		return nil
	}

	return s.ingestSymbols(symbols)
}

func (s *IngestionService) IngestOtherSymbols() error {
	allSymbols := s.getAllSymbols()
	if len(allSymbols) == 0 {
		if err := s.RefreshUniverse(); err != nil {
			return err
		}
		allSymbols = s.getAllSymbols()
	}
	if len(allSymbols) == 0 {
		return nil
	}

	topSet := make(map[string]struct{}, len(s.getTopSymbols()))
	for _, symbol := range s.getTopSymbols() {
		topSet[symbol] = struct{}{}
	}

	others := make([]string, 0, len(allSymbols))
	for _, symbol := range allSymbols {
		if _, isTop := topSet[symbol]; isTop {
			continue
		}
		others = append(others, symbol)
	}
	if len(others) == 0 {
		return nil
	}

	batch := s.nextOtherBatch(others)
	if len(batch) == 0 {
		return nil
	}

	return s.ingestSymbols(batch)
}

func (s *IngestionService) LiveCandles(symbols []string, limit int) []LiveCandle {
	normalized := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		s := strings.ToUpper(strings.TrimSpace(symbol))
		if s == "" {
			continue
		}
		normalized = append(normalized, s)
	}
	return s.liveCache.Snapshot(normalized, limit)
}

func (s *IngestionService) ingestSymbols(symbols []string) error {
	minuteEnd := time.Now().UTC().Truncate(time.Minute)
	defaultStart := minuteEnd.Add(-15 * time.Minute)
	fiveMinuteFloor := time.Now().UTC().Truncate(5 * time.Minute)

	totalPersisted := 0
	var firstErr error

	for _, symbol := range symbols {
		start := defaultStart
		lastOpen, found, err := s.store.LastCandleOpenTime("bitvavo", symbol, "5m")
		if err != nil {
			slog.Warn("failed to read last stored 5m candle", "symbol", symbol, "err", err)
		} else if found {
			nextNeeded := lastOpen.Add(5 * time.Minute)
			if nextNeeded.Before(start) {
				start = nextNeeded
			}
		}

		// Rolling reconciliation window to heal gaps in the middle of the recent history.
		repairStart := minuteEnd.Add(-s.repairLookback)
		if repairStart.Before(start) {
			start = repairStart
		}

		// Keep requests bounded in case of very old gaps; catch-up continues on next runs.
		maxLookback := minuteEnd.Add(-24 * time.Hour)
		if start.Before(maxLookback) {
			start = maxLookback
		}

		bundle, err := s.exchangeClient.Candles(symbol, start, minuteEnd, exchange.Interval1m)
		if err != nil {
			slog.Warn("failed to fetch 1m candles", "symbol", symbol, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(bundle.Candles) == 0 {
			continue
		}

		latest := latestClosedCandle(bundle.Candles, minuteEnd)
		if latest != nil {
			s.liveCache.Set(symbol, *latest)
		}

		agg, err := exchange.Aggregate1mTo(bundle, exchange.Interval5m)
		if err != nil {
			slog.Warn("failed to aggregate candles to 5m", "symbol", symbol, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		closedCandles := filterClosedCandles(agg.Candles, fiveMinuteFloor)
		if len(closedCandles) == 0 {
			continue
		}

		if err := s.store.UpsertCandles("bitvavo", symbol, "5m", closedCandles); err != nil {
			slog.Warn("failed to persist 5m candles", "symbol", symbol, "count", len(closedCandles), "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		totalPersisted += len(closedCandles)
	}

	slog.Info("ingestion batch completed", "symbols", len(symbols), "persisted5m", totalPersisted)
	if firstErr != nil {
		return fmt.Errorf("ingestion completed with errors: %w", firstErr)
	}
	return nil
}

func (s *IngestionService) getTopSymbols() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]string, len(s.topSymbols))
	copy(res, s.topSymbols)
	return res
}

func (s *IngestionService) getAllSymbols() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]string, len(s.allSymbols))
	copy(res, s.allSymbols)
	return res
}

func (s *IngestionService) nextOtherBatch(others []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(others) == 0 {
		return nil
	}

	batchSize := s.otherBatchSize
	if batchSize <= 0 {
		batchSize = defaultOtherBatchSize
	}
	if batchSize > len(others) {
		batchSize = len(others)
	}

	start := s.otherCursor
	if start >= len(others) {
		start = 0
	}

	batch := make([]string, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		idx := (start + i) % len(others)
		batch = append(batch, others[idx])
	}

	s.otherCursor = (start + batchSize) % len(others)
	return batch
}

func filterClosedCandles(candles []exchange.Candle, closeBeforeOrAt time.Time) []exchange.Candle {
	if len(candles) == 0 {
		return nil
	}

	res := make([]exchange.Candle, 0, len(candles))
	for _, candle := range candles {
		if candle.CloseTime.After(closeBeforeOrAt) {
			continue
		}
		res = append(res, candle)
	}
	return res
}

func latestClosedCandle(candles []exchange.Candle, closeBeforeOrAt time.Time) *exchange.Candle {
	for _, candle := range candles {
		if candle.CloseTime.After(closeBeforeOrAt) {
			continue
		}
		c := candle
		return &c
	}
	return nil
}

func uniqueSymbols(symbols []string) []string {
	set := make(map[string]struct{}, len(symbols))
	res := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		if _, ok := set[symbol]; ok {
			continue
		}
		set[symbol] = struct{}{}
		res = append(res, symbol)
	}
	sort.Strings(res)
	return res
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
