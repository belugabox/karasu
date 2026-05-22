package main

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"karasu/internal/exchange"

	_ "modernc.org/sqlite"
)

type CandleStore struct {
	db      *sql.DB
	writeMu sync.Mutex
}

func NewCandleStore(dbPath string) (*CandleStore, error) {
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

	store := &CandleStore{db: db}
	if err := store.ensureSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *CandleStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *CandleStore) ensureSchema() error {
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
	`)
	if err != nil {
		return fmt.Errorf("failed to ensure schema: %w", err)
	}
	return nil
}

func (s *CandleStore) UpsertCandles(exchangeName, symbol, timeframe string, candles []exchange.Candle) error {
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

func (s *CandleStore) upsertCandlesTx(exchangeName, symbol, timeframe string, candles []exchange.Candle) error {
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

func (s *CandleStore) QueryCandles(exchangeName, symbol, timeframe string, limit int) ([]exchange.Candle, error) {
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

func (s *CandleStore) LastCandleOpenTime(exchangeName, symbol, timeframe string) (time.Time, bool, error) {
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
