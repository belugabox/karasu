package store

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"karasu/internal/exchange"

	_ "modernc.org/sqlite"
)

// SQLiteStore is the SQLite-backed implementation of CandleStore.
type SQLiteStore struct {
	db      *sql.DB
	writeMu sync.Mutex
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	if dbPath == "" {
		dbPath = "./karasu.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// SQLite behaves best with a single writer connection in-process.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA foreign_keys=ON;
		PRAGMA busy_timeout=5000;
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply sqlite pragmas: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.ensureSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) ensureSchema() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS crypto_candles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		exchange TEXT NOT NULL,
		symbol TEXT NOT NULL,
		timeframe TEXT NOT NULL,
		open REAL NOT NULL,
		high REAL NOT NULL,
		low REAL NOT NULL,
		close REAL NOT NULL,
		volume REAL NOT NULL,
		open_time TEXT NOT NULL,
		close_time TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(exchange, symbol, timeframe, open_time)
	);

	CREATE INDEX IF NOT EXISTS idx_crypto_candles_symbol_tf_open
		ON crypto_candles(symbol, timeframe, open_time DESC);

	CREATE INDEX IF NOT EXISTS idx_crypto_candles_exchange_tf_open
		ON crypto_candles(exchange, timeframe, open_time DESC);

	CREATE TABLE IF NOT EXISTS alerts (
		id TEXT NOT NULL,
		key TEXT NOT NULL UNIQUE,
		category TEXT NOT NULL,
		severity TEXT NOT NULL,
		message TEXT NOT NULL,
		source TEXT NOT NULL,
		symbol TEXT NOT NULL DEFAULT '',
		active INTEGER NOT NULL DEFAULT 1,
		count INTEGER NOT NULL DEFAULT 1,
		first_seen TEXT NOT NULL,
		last_seen TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_alerts_active_last_seen
		ON alerts(active DESC, last_seen DESC);
	`)
	if err != nil {
		return fmt.Errorf("failed to ensure schema: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpsertCandles(exchangeName, symbol, timeframe string, candles []exchange.Candle) error {
	if len(candles) == 0 {
		return nil
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		err := s.upsertCandlesTx(exchangeName, symbol, timeframe, candles)
		if err == nil {
			return nil
		}

		lastErr = err
		if !isSQLiteBusy(err) || attempt == 3 {
			break
		}

		time.Sleep(time.Duration(attempt*150) * time.Millisecond)
	}

	return lastErr
}

func (s *SQLiteStore) upsertCandlesTx(exchangeName, symbol, timeframe string, candles []exchange.Candle) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO crypto_candles (
			exchange, symbol, timeframe,
			open, high, low, close, volume,
			open_time, close_time, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(exchange, symbol, timeframe, open_time) DO UPDATE SET
			high = excluded.high,
			low = excluded.low,
			close = excluded.close,
			volume = excluded.volume,
			close_time = excluded.close_time,
			updated_at = CURRENT_TIMESTAMP;
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare upsert statement: %w", err)
	}
	defer stmt.Close()

	for _, candle := range candles {
		if _, err := stmt.Exec(
			exchangeName,
			symbol,
			timeframe,
			candle.Open,
			candle.High,
			candle.Low,
			candle.Close,
			candle.Volume,
			candle.OpenTime.UTC().Format(time.RFC3339Nano),
			candle.CloseTime.UTC().Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("failed to upsert candle: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "DATABASE IS LOCKED")
}

func (s *SQLiteStore) QueryCandles(exchangeName, symbol, timeframe string, limit int) ([]exchange.Candle, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}

	rows, err := s.db.Query(`
		SELECT open, high, low, close, volume, open_time, close_time
		FROM crypto_candles
		WHERE exchange = ? AND symbol = ? AND timeframe = ?
		ORDER BY open_time DESC
		LIMIT ?;
	`, exchangeName, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query candles: %w", err)
	}
	defer rows.Close()

	candles := make([]exchange.Candle, 0, limit)
	for rows.Next() {
		var c exchange.Candle
		var openTime string
		var closeTime string
		if err := rows.Scan(&c.Open, &c.High, &c.Low, &c.Close, &c.Volume, &openTime, &closeTime); err != nil {
			return nil, fmt.Errorf("failed to scan candle row: %w", err)
		}

		c.OpenTime, err = time.Parse(time.RFC3339Nano, openTime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse open_time: %w", err)
		}
		c.CloseTime, err = time.Parse(time.RFC3339Nano, closeTime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse close_time: %w", err)
		}

		candles = append(candles, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while iterating candle rows: %w", err)
	}

	// Return oldest -> newest for chart-friendly consumption.
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}

	return candles, nil
}

func (s *SQLiteStore) LastCandleOpenTime(exchangeName, symbol, timeframe string) (time.Time, bool, error) {
	var raw string
	err := s.db.QueryRow(`
		SELECT open_time
		FROM crypto_candles
		WHERE exchange = ? AND symbol = ? AND timeframe = ?
		ORDER BY open_time DESC
		LIMIT 1;
	`, exchangeName, symbol, timeframe).Scan(&raw)

	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("failed to query last candle open time: %w", err)
	}

	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("failed to parse last candle open time: %w", err)
	}

	return parsed.UTC(), true, nil
}

func (s *SQLiteStore) QueryDailySymbolActivity(exchangeName, timeframe string, days int) ([]DailySymbolActivity, error) {
	if days <= 0 {
		days = 90
	}
	if days > 365 {
		days = 365
	}

	endDay := time.Now().UTC().Truncate(24 * time.Hour)
	startDay := endDay.AddDate(0, 0, -(days - 1))

	rows, err := s.db.Query(`
		SELECT
			substr(open_time, 1, 10) AS day,
			COUNT(DISTINCT symbol) AS symbol_count,
			COUNT(*) AS candle_count
		FROM crypto_candles
		WHERE exchange = ?
			AND timeframe = ?
			AND open_time >= ?
			AND open_time < ?
		GROUP BY day
		ORDER BY day ASC;
	`,
		exchangeName,
		timeframe,
		startDay.Format(time.RFC3339Nano),
		endDay.Add(24*time.Hour).Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily symbol activity: %w", err)
	}
	defer rows.Close()

	byDay := make(map[string]DailySymbolActivity, days)
	for rows.Next() {
		var item DailySymbolActivity
		if err := rows.Scan(&item.Day, &item.SymbolCount, &item.CandleCount); err != nil {
			return nil, fmt.Errorf("failed to scan daily symbol activity: %w", err)
		}
		byDay[item.Day] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while iterating daily symbol activity rows: %w", err)
	}

	result := make([]DailySymbolActivity, 0, days)
	for d := 0; d < days; d++ {
		day := startDay.AddDate(0, 0, d).Format("2006-01-02")
		if item, ok := byDay[day]; ok {
			result = append(result, item)
			continue
		}
		result = append(result, DailySymbolActivity{Day: day, SymbolCount: 0, CandleCount: 0})
	}

	return result, nil
}

// UpsertAlert inserts or updates an alert keyed by AlertEvent.Key.
func (s *SQLiteStore) UpsertAlert(alert AlertEvent) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO alerts (id, key, category, severity, message, source, symbol, active, count, first_seen, last_seen, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			id        = excluded.id,
			category  = excluded.category,
			severity  = excluded.severity,
			message   = excluded.message,
			source    = excluded.source,
			symbol    = excluded.symbol,
			active    = excluded.active,
			count     = excluded.count,
			last_seen = excluded.last_seen,
			updated_at = CURRENT_TIMESTAMP;
	`,
		alert.ID,
		alert.Key,
		alert.Category,
		string(alert.Severity),
		alert.Message,
		alert.Source,
		alert.Symbol,
		boolToInt(alert.Active),
		alert.Count,
		alert.FirstSeen.UTC().Format(time.RFC3339Nano),
		alert.LastSeen.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert alert: %w", err)
	}
	return nil
}

// ListAlerts returns a page of alerts and the total count of matching rows.
func (s *SQLiteStore) ListAlerts(limit, offset int, activeOnly bool) ([]AlertEvent, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	where := ""
	args := []any{}
	if activeOnly {
		where = "WHERE active = 1 "
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM alerts `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count alerts: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT id, key, category, severity, message, source, symbol, active, count, first_seen, last_seen
		 FROM alerts `+where+`ORDER BY active DESC, last_seen DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query alerts: %w", err)
	}
	defer rows.Close()

	alerts := make([]AlertEvent, 0, limit)
	for rows.Next() {
		var a AlertEvent
		var severity string
		var activeInt int
		var firstSeen, lastSeen string
		if err := rows.Scan(&a.ID, &a.Key, &a.Category, &severity, &a.Message, &a.Source, &a.Symbol, &activeInt, &a.Count, &firstSeen, &lastSeen); err != nil {
			return nil, 0, fmt.Errorf("failed to scan alert row: %w", err)
		}
		a.Severity = AlertSeverity(severity)
		a.Active = activeInt != 0
		if a.FirstSeen, err = time.Parse(time.RFC3339Nano, firstSeen); err != nil {
			return nil, 0, fmt.Errorf("failed to parse alert first_seen: %w", err)
		}
		if a.LastSeen, err = time.Parse(time.RFC3339Nano, lastSeen); err != nil {
			return nil, 0, fmt.Errorf("failed to parse alert last_seen: %w", err)
		}
		alerts = append(alerts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating alert rows: %w", err)
	}

	return alerts, total, nil
}

// PruneAlerts removes alerts whose last_seen timestamp is older than before.
func (s *SQLiteStore) PruneAlerts(before time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.Exec(
		`DELETE FROM alerts WHERE last_seen < ?`,
		before.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("failed to prune alerts: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

