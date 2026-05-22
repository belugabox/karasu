package ingestion

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"karasu/internal/exchange"
	"karasu/internal/market"
	"karasu/internal/store"
)

const (
	defaultTopSymbolsCount = 30
	defaultOtherBatchSize  = 80
	defaultRepairLookback  = 6 * time.Hour
	defaultBackfillChunk   = 12 * time.Hour
	defaultBackfillQueue   = 128
)

// LiveCandle is a real-time candle snapshot served by /api/live-1m.
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

// LiveCache holds the latest 1m candle per symbol in memory.
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

// IngestionService orchestrates live candle ingestion and backfill.
type IngestionService struct {
	exchangeClient exchange.ExchangeClient
	store          store.CandleStore
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

	backfillQueue chan backfillRequest
	backfillSeq   atomic.Uint64
	jobsMu        sync.RWMutex
	jobs          map[string]BackfillJob
}

func NewIngestionService(exchangeClient exchange.ExchangeClient, candleStore store.CandleStore) *IngestionService {
	s := &IngestionService{
		exchangeClient: exchangeClient,
		store:          candleStore,
		liveCache:      NewLiveCache(),
		otherBatchSize: defaultOtherBatchSize,
		topSymbolsSize: defaultTopSymbolsCount,
		repairLookback: defaultRepairLookback,
		backfillChunk:  defaultBackfillChunk,
		allSymbols:     make([]string, 0),
		topSymbols:     make([]string, 0),
		backfillQueue:  make(chan backfillRequest, defaultBackfillQueue),
		jobs:           make(map[string]BackfillJob),
	}
	s.startBackfillWorker()
	return s
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

// BackfillReport summarises the result of a Backfill5m run.
type BackfillReport struct {
	Symbols          int   `json:"symbols"`
	Chunks           int   `json:"chunks"`
	Fetched1mCandles int   `json:"fetched1mCandles"`
	Aggregated5m     int   `json:"aggregated5m"`
	Filtered5m       int   `json:"filtered5m"`
	Persisted5m      int   `json:"persisted5m"`
	DurationMs       int64 `json:"durationMs"`
}

type BackfillJobState string

const (
	BackfillQueued  BackfillJobState = "queued"
	BackfillRunning BackfillJobState = "running"
	BackfillDone    BackfillJobState = "done"
	BackfillFailed  BackfillJobState = "failed"
)

type BackfillJob struct {
	ID        string           `json:"id"`
	Symbols   []string         `json:"symbols"`
	From      time.Time        `json:"from"`
	To        time.Time        `json:"to"`
	Reason    string           `json:"reason"`
	State     BackfillJobState `json:"state"`
	CreatedAt time.Time        `json:"createdAt"`
	StartedAt *time.Time       `json:"startedAt,omitempty"`
	EndedAt   *time.Time       `json:"endedAt,omitempty"`
	Report    *BackfillReport  `json:"report,omitempty"`
	Error     string           `json:"error,omitempty"`
}

type backfillRequest struct {
	JobID   string
	Symbols []string
	From    time.Time
	To      time.Time
	Reason  string
}

func (s *IngestionService) EnqueueBackfill(symbols []string, from, to time.Time, reason string) (BackfillJob, error) {
	from = from.UTC().Truncate(time.Minute)
	to = to.UTC().Truncate(time.Minute)
	if !from.Before(to) {
		return BackfillJob{}, fmt.Errorf("from must be before to")
	}

	normalizedSymbols := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		normalizedSymbols = append(normalizedSymbols, sym)
	}
	normalizedSymbols = uniqueSymbols(normalizedSymbols)

	id := fmt.Sprintf("bf_%d_%d", time.Now().UTC().UnixMilli(), s.backfillSeq.Add(1))
	job := BackfillJob{
		ID:        id,
		Symbols:   normalizedSymbols,
		From:      from,
		To:        to,
		Reason:    reason,
		State:     BackfillQueued,
		CreatedAt: time.Now().UTC(),
	}

	s.jobsMu.Lock()
	s.jobs[id] = job
	s.jobsMu.Unlock()

	request := backfillRequest{JobID: id, Symbols: normalizedSymbols, From: from, To: to, Reason: reason}
	select {
	case s.backfillQueue <- request:
		return job, nil
	default:
		s.jobsMu.Lock()
		delete(s.jobs, id)
		s.jobsMu.Unlock()
		return BackfillJob{}, fmt.Errorf("backfill queue is full")
	}
}

func (s *IngestionService) GetBackfillJob(jobID string) (BackfillJob, bool) {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return BackfillJob{}, false
	}
	return job, true
}

func (s *IngestionService) ListBackfillJobs(limit int) []BackfillJob {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	s.jobsMu.RLock()
	jobs := make([]BackfillJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	s.jobsMu.RUnlock()

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	if len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs
}

func (s *IngestionService) startBackfillWorker() {
	go func() {
		for req := range s.backfillQueue {
			now := time.Now().UTC()
			s.jobsMu.Lock()
			job := s.jobs[req.JobID]
			job.State = BackfillRunning
			job.StartedAt = &now
			s.jobs[req.JobID] = job
			s.jobsMu.Unlock()

			report, err := s.Backfill5m(req.Symbols, req.From, req.To)

			ended := time.Now().UTC()
			s.jobsMu.Lock()
			job = s.jobs[req.JobID]
			job.EndedAt = &ended
			job.Report = &report
			if err != nil {
				job.State = BackfillFailed
				job.Error = err.Error()
			} else {
				job.State = BackfillDone
				job.Error = ""
			}
			s.jobs[req.JobID] = job
			s.jobsMu.Unlock()
		}
	}()
}

func (s *IngestionService) hasActiveBackfillForSymbol(symbol string) bool {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	for _, job := range s.jobs {
		if job.State != BackfillQueued && job.State != BackfillRunning {
			continue
		}
		for _, sym := range job.Symbols {
			if sym == symbol {
				return true
			}
		}
	}
	return false
}

// RepairDetectedGaps detects stale symbols and enqueues targeted backfills.
func (s *IngestionService) RepairDetectedGaps() error {
	symbols := s.getAllSymbols()
	if len(symbols) == 0 {
		if err := s.RefreshUniverse(); err != nil {
			return err
		}
		symbols = s.getAllSymbols()
	}
	if len(symbols) == 0 {
		return nil
	}

	nowFloor := time.Now().UTC().Truncate(5 * time.Minute)
	repairFromFloor := nowFloor.Add(-s.repairLookback)
	queued := 0
	const maxQueuedPerRun = 15

	for _, symbol := range symbols {
		if queued >= maxQueuedPerRun {
			break
		}
		if s.hasActiveBackfillForSymbol(symbol) {
			continue
		}

		lastOpen, found, err := s.store.LastCandleOpenTime("bitvavo", symbol, "5m")
		if err != nil {
			slog.Warn("gap repair read last candle failed", "symbol", symbol, "err", err)
			continue
		}

		from := repairFromFloor
		if found {
			expectedNext := lastOpen.UTC().Add(5 * time.Minute)
			if !expectedNext.Before(nowFloor.Add(-5 * time.Minute)) {
				continue
			}
			if expectedNext.After(from) {
				from = expectedNext
			}
		}

		if _, err := s.EnqueueBackfill([]string{symbol}, from, nowFloor, "gap-repair"); err != nil {
			slog.Warn("gap repair enqueue failed", "symbol", symbol, "err", err)
			continue
		}
		queued++
	}

	if queued > 0 {
		slog.Info("gap repair queued backfills", "count", queued)
	}
	return nil
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

				report.Aggregated5m += len(agg.Candles)

				closedFloor := chunkEnd.Truncate(5 * time.Minute)
				candles5m := filterClosedCandles(agg.Candles, closedFloor)
				report.Filtered5m += len(candles5m)
				if len(candles5m) > 0 {
					if err := s.store.UpsertCandles("bitvavo", symbol, "5m", candles5m); err != nil {
						slog.Warn("backfill persist failed", "symbol", symbol, "count", len(candles5m), "err", err)
						if firstErr == nil {
							firstErr = err
						}
					} else {
						report.Persisted5m += len(candles5m)
						slog.Debug("backfill upserted", "symbol", symbol, "count", len(candles5m), "from", candles5m[0].OpenTime, "to", candles5m[len(candles5m)-1].OpenTime)
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
		"aggregated5m", report.Aggregated5m,
		"filtered5m", report.Filtered5m,
		"persisted5m", report.Persisted5m,
		"duration_ms", report.DurationMs,
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

	topSymbols, err := market.FindTopSymbols(s.exchangeClient)
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

	batch := s.nextOtherBatchPrioritized(others)
	if len(batch) == 0 {
		return nil
	}

	return s.ingestSymbols(batch)
}

func (s *IngestionService) LiveCandles(symbols []string, limit int) []LiveCandle {
	normalized := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		s := strings.ToUpper(strings.TrimSpace(sym))
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

		repairStart := minuteEnd.Add(-s.repairLookback)
		if repairStart.Before(start) {
			start = repairStart
		}

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

func (s *IngestionService) nextOtherBatchPrioritized(others []string) []string {
	s.mu.Lock()
	start := s.otherCursor
	if start >= len(others) {
		start = 0
	}
	batchSize := s.otherBatchSize
	if batchSize <= 0 {
		batchSize = defaultOtherBatchSize
	}
	if batchSize > len(others) {
		batchSize = len(others)
	}
	candidateSize := batchSize * 3
	if candidateSize > len(others) {
		candidateSize = len(others)
	}

	candidates := make([]string, 0, candidateSize)
	for i := 0; i < candidateSize; i++ {
		idx := (start + i) % len(others)
		candidates = append(candidates, others[idx])
	}
	s.otherCursor = (start + candidateSize) % len(others)
	s.mu.Unlock()

	if len(others) == 0 {
		return nil
	}

	now := time.Now().UTC().Truncate(5 * time.Minute)
	type scoredSymbol struct {
		symbol string
		score  float64
	}
	scored := make([]scoredSymbol, 0, len(candidates))
	for _, symbol := range candidates {
		scored = append(scored, scoredSymbol{symbol: symbol, score: s.otherSymbolPriority(symbol, now)})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].symbol < scored[j].symbol
		}
		return scored[i].score > scored[j].score
	})

	batch := make([]string, 0, batchSize)
	for i := 0; i < batchSize && i < len(scored); i++ {
		batch = append(batch, scored[i].symbol)
	}
	return batch
}

func (s *IngestionService) otherSymbolPriority(symbol string, now time.Time) float64 {
	lastOpen, found, err := s.store.LastCandleOpenTime("bitvavo", symbol, "5m")
	if err != nil {
		slog.Debug("priority read failed", "symbol", symbol, "err", err)
		return 0
	}
	if !found {
		return 1_000_000
	}

	expectedNext := lastOpen.UTC().Add(5 * time.Minute)
	if expectedNext.After(now) {
		return 0
	}
	return now.Sub(expectedNext).Minutes()
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
