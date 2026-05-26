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
	"karasu/internal/notification"
	"karasu/internal/store"
)

const (
	defaultTopSymbolsCount   = 30
	defaultOtherBatchSize    = 80
	defaultRepairLookback    = 6 * time.Hour
	defaultBackfillChunk     = 12 * time.Hour
	defaultBackfillQueue     = 128
	defaultRateLimitCooldown = 30 * time.Minute
	defaultAlertDedupWindow  = 5 * time.Minute
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

func (c *LiveCache) Stats() (count int, latestUpdatedAt time.Time, hasData bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count = len(c.latest)
	for _, entry := range c.latest {
		if !hasData || entry.updatedAt.After(latestUpdatedAt) {
			latestUpdatedAt = entry.updatedAt
			hasData = true
		}
	}

	return count, latestUpdatedAt, hasData
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
	alertStore     store.AlertStore
	alertNotifier  notification.AlertNotifier
	liveCache      *LiveCache
	otherBatchSize int
	topSymbolsSize int
	repairLookback time.Duration
	backfillChunk  time.Duration
	backfillMu     sync.Mutex

	mu                    sync.RWMutex
	allSymbols            []string
	topSymbols            []string
	otherCursor           int
	rateLimitBlockedUntil time.Time
	rateLimitLastError    string

	backfillQueue chan backfillRequest
	backfillSeq   atomic.Uint64
	alertSeq      atomic.Uint64
	jobsMu        sync.RWMutex
	jobs          map[string]BackfillJob
	alertsMu      sync.RWMutex
	alerts        map[string]store.AlertEvent
	lastPrunedAt  time.Time
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
		alerts:         make(map[string]store.AlertEvent),
	}
	s.startBackfillWorker()
	return s
}

// SetAlertStore wires a persistent AlertStore. When set, alerts are written to
// SQLite and read back from it; the in-memory cache is still kept for fast access
// by syncHealthIssueAlerts. If never called, alerts live in memory only (suitable
// for tests).
func (s *IngestionService) SetAlertStore(as store.AlertStore) {
	s.alertStore = as
}

func (s *IngestionService) SetAlertNotifier(an notification.AlertNotifier) {
	s.alertNotifier = an
}

func (s *IngestionService) upsertAlert(key, category string, severity store.AlertSeverity, message, source, symbol string, active bool) {
	if key == "" {
		return
	}

	now := time.Now().UTC()
	s.alertsMu.Lock()
	defer s.alertsMu.Unlock()

	existing, found := s.alerts[key]
	if !found {
		event := store.AlertEvent{
			ID:        fmt.Sprintf("al_%d_%d", now.UnixMilli(), s.alertSeq.Add(1)),
			Key:       key,
			Category:  category,
			Severity:  severity,
			Message:   message,
			Source:    source,
			Symbol:    symbol,
			Active:    active,
			Count:     1,
			FirstSeen: now,
			LastSeen:  now,
		}
		s.alerts[key] = event
		s.persistAlert(event)
		s.notifyAlert(event)
		return
	}

	changed := existing.Category != category ||
		existing.Message != message ||
		existing.Severity != severity ||
		existing.Source != source ||
		existing.Symbol != symbol ||
		existing.Active != active
	bumpCount := now.Sub(existing.LastSeen) >= defaultAlertDedupWindow || changed
	shouldNotify := changed

	existing.Category = category
	existing.Severity = severity
	existing.Message = message
	existing.Source = source
	existing.Symbol = symbol
	existing.Active = active
	existing.LastSeen = now
	if bumpCount {
		existing.Count++
	}

	s.alerts[key] = existing
	s.persistAlert(existing)
	if shouldNotify {
		s.notifyAlert(existing)
	}
}

// persistAlert writes an alert to the SQLite store when one is configured.
// It also prunes old alerts at most once per hour to enforce the retention policy.
// Must be called with alertsMu held (or after the lock is no longer needed).
func (s *IngestionService) persistAlert(event store.AlertEvent) {
	if s.alertStore == nil {
		return
	}
	if err := s.alertStore.UpsertAlert(event); err != nil {
		slog.Warn("failed to persist alert", "key", event.Key, "err", err)
	}

	// Prune resolved alerts older than 30 days, at most once per hour.
	if time.Since(s.lastPrunedAt) >= time.Hour {
		s.lastPrunedAt = time.Now().UTC()
		cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
		if err := s.alertStore.PruneAlerts(cutoff); err != nil {
			slog.Warn("failed to prune old alerts", "err", err)
		}
	}
}

func (s *IngestionService) notifyAlert(event store.AlertEvent) {
	if s.alertNotifier == nil {
		return
	}
	if err := s.alertNotifier.NotifyAlert(event); err != nil {
		slog.Warn("failed to send alert notification", "key", event.Key, "err", err)
	}
}

func (s *IngestionService) recordAlert(key, category string, severity store.AlertSeverity, message, source, symbol string) {
	s.upsertAlert(key, category, severity, message, source, symbol, true)
}

func (s *IngestionService) resolveAlert(key, category, source string) {
	s.upsertAlert(key, category, store.AlertSeverityInfo, "resolved", source, "", false)
}

// ListAlerts returns a page of alerts. When a persistent AlertStore is wired it
// reads from SQLite; otherwise it falls back to the in-memory cache.
// It returns the alerts slice, the total matching row count, and any error.
func (s *IngestionService) ListAlerts(limit, offset int, activeOnly bool) ([]store.AlertEvent, int, error) {
	if s.alertStore != nil {
		return s.alertStore.ListAlerts(limit, offset, activeOnly)
	}

	// In-memory fallback (used in tests / when no persistent store is set).
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	s.alertsMu.RLock()
	alerts := make([]store.AlertEvent, 0, len(s.alerts))
	for _, alert := range s.alerts {
		if activeOnly && !alert.Active {
			continue
		}
		alerts = append(alerts, alert)
	}
	s.alertsMu.RUnlock()

	sort.SliceStable(alerts, func(i, j int) bool {
		if alerts[i].Active != alerts[j].Active {
			return alerts[i].Active
		}
		return alerts[i].LastSeen.After(alerts[j].LastSeen)
	})

	total := len(alerts)
	if offset < len(alerts) {
		alerts = alerts[offset:]
	} else {
		alerts = alerts[:0]
	}
	if len(alerts) > limit {
		alerts = alerts[:limit]
	}
	return alerts, total, nil
}

func (s *IngestionService) syncHealthIssueAlerts(issues []string) {
	activeKeys := make(map[string]struct{}, len(issues))
	for _, issue := range issues {
		key := "health:" + issue
		activeKeys[key] = struct{}{}
		s.recordAlert(key, "health", store.AlertSeverityWarning, issue, "system-health", "")
	}

	s.alertsMu.RLock()
	healthKeys := make([]string, 0)
	for key := range s.alerts {
		if strings.HasPrefix(key, "health:") {
			healthKeys = append(healthKeys, key)
		}
	}
	s.alertsMu.RUnlock()

	for _, key := range healthKeys {
		if _, stillActive := activeKeys[key]; stillActive {
			continue
		}
		s.resolveAlert(key, "health", "system-health")
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

func isRateLimitBanError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "errorcode:105") ||
		(strings.Contains(message, "rate limit") && strings.Contains(message, "banned"))
}

func (s *IngestionService) setRateLimitCooldown(err error) {
	until := time.Now().UTC().Add(defaultRateLimitCooldown)
	s.mu.Lock()
	if s.rateLimitBlockedUntil.Before(until) {
		s.rateLimitBlockedUntil = until
	}
	if err != nil {
		s.rateLimitLastError = err.Error()
	}
	s.mu.Unlock()
	slog.Warn("rate limit cooldown activated", "until", until, "cooldown", defaultRateLimitCooldown.String(), "err", err)
	s.recordAlert(
		"exchange:rate-limit-ban",
		"exchange",
		AlertSeverityCritical,
		fmt.Sprintf("rate limit active until %s", until.Format(time.RFC3339)),
		"exchange",
		"",
	)
}

func (s *IngestionService) rateLimitStatus() (bool, time.Time, string) {
	s.mu.RLock()
	blockedUntil := s.rateLimitBlockedUntil
	lastErr := s.rateLimitLastError
	s.mu.RUnlock()
	if blockedUntil.IsZero() {
		return false, time.Time{}, lastErr
	}
	if time.Now().UTC().After(blockedUntil) {
		return false, blockedUntil, lastErr
	}
	return true, blockedUntil, lastErr
}

func (s *IngestionService) clearRateLimitCooldown() {
	s.mu.Lock()
	s.rateLimitBlockedUntil = time.Time{}
	s.rateLimitLastError = ""
	s.mu.Unlock()
	s.resolveAlert("exchange:rate-limit-ban", "exchange", "exchange")
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

type AlertSeverity = store.AlertSeverity

const (
	AlertSeverityInfo     = store.AlertSeverityInfo
	AlertSeverityWarning  = store.AlertSeverityWarning
	AlertSeverityCritical = store.AlertSeverityCritical
)

type SystemHealth struct {
	GeneratedAt          time.Time  `json:"generatedAt"`
	IsHealthy            bool       `json:"isHealthy"`
	Issues               []string   `json:"issues"`
	UniverseSymbols      int        `json:"universeSymbols"`
	TopSymbols           int        `json:"topSymbols"`
	LiveSymbols          int        `json:"liveSymbols"`
	LiveLastUpdatedAt    *time.Time `json:"liveLastUpdatedAt,omitempty"`
	LiveFresh            bool       `json:"liveFresh"`
	StaleThresholdMin    int        `json:"staleThresholdMin"`
	TopSymbolsStale5m    int        `json:"topSymbolsStale5m"`
	TopStaleExamples     []string   `json:"topStaleExamples"`
	StoreReadErrors      int        `json:"storeReadErrors"`
	BackfillQueueDepth   int        `json:"backfillQueueDepth"`
	BackfillQueueCap     int        `json:"backfillQueueCap"`
	BackfillQueuedJobs   int        `json:"backfillQueuedJobs"`
	BackfillRunningJobs  int        `json:"backfillRunningJobs"`
	BackfillFailedJobs24 int        `json:"backfillFailedJobs24h"`
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
				s.recordAlert(
					"backfill:failed",
					"backfill",
					AlertSeverityWarning,
					fmt.Sprintf("backfill failed for job %s: %v", req.JobID, err),
					"backfill-worker",
					"",
				)
			} else {
				job.State = BackfillDone
				job.Error = ""
				s.resolveAlert("backfill:failed", "backfill", "backfill-worker")
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
	if blocked, until, _ := s.rateLimitStatus(); blocked {
		return BackfillReport{}, fmt.Errorf("backfill paused until %s because exchange rate limit is active", until.Format(time.RFC3339))
	}

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
				if isRateLimitBanError(err) {
					s.setRateLimitCooldown(err)
					if firstErr == nil {
						firstErr = err
					}
					break
				}
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
	if blocked, until, lastErr := s.rateLimitStatus(); blocked {
		if len(s.getAllSymbols()) > 0 {
			slog.Warn("refresh universe skipped due to active rate limit cooldown", "until", until, "lastErr", lastErr)
			return nil
		}
		return fmt.Errorf("failed to refresh universe: exchange cooldown active until %s", until.Format(time.RFC3339))
	}

	bundles, err := s.exchangeClient.CandlesLast24h()
	if err != nil {
		if isRateLimitBanError(err) {
			s.setRateLimitCooldown(err)
			if len(s.getAllSymbols()) > 0 {
				slog.Warn("refresh universe failed on rate limit, reusing cached universe", "err", err)
				return nil
			}
		}
		return fmt.Errorf("failed to refresh universe: %w", err)
	}
	s.clearRateLimitCooldown()

	symbols := make([]string, 0, len(bundles))
	for _, bundle := range bundles {
		if strings.TrimSpace(bundle.Symbol) == "" {
			continue
		}
		symbols = append(symbols, strings.ToUpper(bundle.Symbol))
	}
	symbols = uniqueSymbols(symbols)
	tradableSet := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		tradableSet[symbol] = struct{}{}
	}

	topSymbols, err := market.FindTopSymbols(s.exchangeClient)
	if err != nil {
		return fmt.Errorf("failed to compute top symbols: %w", err)
	}
	wallet, err := s.exchangeClient.Wallet()
	if err != nil {
		return fmt.Errorf("failed to load wallet for priority symbols: %w", err)
	}
	const minWalletValueEUR = 0.009
	mandatoryWalletSymbols := make([]string, 0, len(wallet.Assets))
	for _, asset := range wallet.Assets {
		symbol := strings.ToUpper(strings.TrimSpace(asset.Symbol))
		if symbol == "" || symbol == "EUR" {
			continue
		}
		if _, tradable := tradableSet[symbol]; !tradable {
			continue
		}
		if asset.Value > minWalletValueEUR {
			mandatoryWalletSymbols = append(mandatoryWalletSymbols, symbol)
		}
	}
	mandatoryWalletSymbols = uniqueSymbols(mandatoryWalletSymbols)

	// Ensure mandatory wallet symbols are part of the tradable universe.
	for _, symbol := range mandatoryWalletSymbols {
		symbols = append(symbols, symbol)
	}
	symbols = uniqueSymbols(symbols)

	for i := range topSymbols {
		topSymbols[i] = strings.ToUpper(topSymbols[i])
	}
	topSymbols = append(mandatoryWalletSymbols, topSymbols...)
	topSymbols = uniqueSymbols(topSymbols)
	if len(topSymbols) > s.topSymbolsSize {
		// Preserve mandatory wallet symbols first, then fill the remaining budget.
		mandatorySet := make(map[string]struct{}, len(mandatoryWalletSymbols))
		for _, symbol := range mandatoryWalletSymbols {
			mandatorySet[symbol] = struct{}{}
		}
		trimmed := make([]string, 0, s.topSymbolsSize)
		for _, symbol := range topSymbols {
			if len(trimmed) >= s.topSymbolsSize {
				break
			}
			if _, isMandatory := mandatorySet[symbol]; !isMandatory {
				continue
			}
			trimmed = append(trimmed, symbol)
		}
		if len(trimmed) < s.topSymbolsSize {
			for _, symbol := range topSymbols {
				if len(trimmed) >= s.topSymbolsSize {
					break
				}
				already := false
				for _, existing := range trimmed {
					if existing == symbol {
						already = true
						break
					}
				}
				if already {
					continue
				}
				trimmed = append(trimmed, symbol)
			}
		}
		topSymbols = trimmed
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
	if blocked, until, _ := s.rateLimitStatus(); blocked {
		slog.Warn("ingest top symbols skipped due to active rate limit cooldown", "until", until)
		return nil
	}

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
	if blocked, until, _ := s.rateLimitStatus(); blocked {
		slog.Warn("ingest other symbols skipped due to active rate limit cooldown", "until", until)
		return nil
	}

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

func (s *IngestionService) SystemHealthSnapshot(staleThreshold time.Duration) SystemHealth {
	if staleThreshold <= 0 {
		staleThreshold = 20 * time.Minute
	}

	now := time.Now().UTC()
	allSymbols := s.getAllSymbols()
	topSymbols := s.getTopSymbols()

	liveCount, liveLatest, hasLive := s.liveCache.Stats()
	liveFresh := hasLive && now.Sub(liveLatest) <= staleThreshold

	topStaleCount := 0
	storeReadErrors := 0
	topStaleExamples := make([]string, 0, 5)

	for _, symbol := range topSymbols {
		lastOpen, found, err := s.store.LastCandleOpenTime("bitvavo", symbol, "5m")
		if err != nil {
			storeReadErrors++
			continue
		}
		if !found {
			topStaleCount++
			if len(topStaleExamples) < cap(topStaleExamples) {
				topStaleExamples = append(topStaleExamples, symbol+":missing")
			}
			continue
		}

		expectedNext := lastOpen.UTC().Add(5 * time.Minute)
		if now.Sub(expectedNext) > staleThreshold {
			topStaleCount++
			if len(topStaleExamples) < cap(topStaleExamples) {
				topStaleExamples = append(topStaleExamples, fmt.Sprintf("%s:%dm", symbol, int(now.Sub(expectedNext).Minutes())))
			}
		}
	}

	jobs := s.ListBackfillJobs(200)
	queuedJobs := 0
	runningJobs := 0
	failed24h := 0
	for _, job := range jobs {
		switch job.State {
		case BackfillQueued:
			queuedJobs++
		case BackfillRunning:
			runningJobs++
		case BackfillFailed:
			if now.Sub(job.CreatedAt) <= 24*time.Hour {
				failed24h++
			}
		}
	}

	issues := make([]string, 0, 6)
	if blocked, until, _ := s.rateLimitStatus(); blocked {
		issues = append(issues, fmt.Sprintf("exchange en cooldown rate limit jusqu a %s", until.Format(time.RFC3339)))
	}
	if len(allSymbols) == 0 {
		issues = append(issues, "univers vide")
	}
	if len(topSymbols) == 0 {
		issues = append(issues, "top symbols vide")
	}
	if !liveFresh {
		issues = append(issues, "flux live 1m stale")
	}
	if topStaleCount > 0 {
		issues = append(issues, fmt.Sprintf("%d symboles top en retard 5m", topStaleCount))
	}
	if storeReadErrors > 0 {
		issues = append(issues, fmt.Sprintf("%d erreurs de lecture store", storeReadErrors))
	}
	if failed24h > 0 {
		issues = append(issues, fmt.Sprintf("%d jobs backfill echoues sur 24h", failed24h))
	}
	s.syncHealthIssueAlerts(issues)

	var liveLastUpdatedAt *time.Time
	if hasLive {
		live := liveLatest
		liveLastUpdatedAt = &live
	}

	return SystemHealth{
		GeneratedAt:          now,
		IsHealthy:            len(issues) == 0,
		Issues:               issues,
		UniverseSymbols:      len(allSymbols),
		TopSymbols:           len(topSymbols),
		LiveSymbols:          liveCount,
		LiveLastUpdatedAt:    liveLastUpdatedAt,
		LiveFresh:            liveFresh,
		StaleThresholdMin:    int(staleThreshold / time.Minute),
		TopSymbolsStale5m:    topStaleCount,
		TopStaleExamples:     topStaleExamples,
		StoreReadErrors:      storeReadErrors,
		BackfillQueueDepth:   len(s.backfillQueue),
		BackfillQueueCap:     cap(s.backfillQueue),
		BackfillQueuedJobs:   queuedJobs,
		BackfillRunningJobs:  runningJobs,
		BackfillFailedJobs24: failed24h,
	}
}

func (s *IngestionService) ingestSymbols(symbols []string) error {
	if blocked, until, _ := s.rateLimitStatus(); blocked {
		slog.Warn("ingestion batch skipped due to active rate limit cooldown", "until", until)
		return nil
	}

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
			if isRateLimitBanError(err) {
				s.setRateLimitCooldown(err)
				if firstErr == nil {
					firstErr = err
				}
				break
			}
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
