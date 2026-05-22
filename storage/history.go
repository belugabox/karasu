package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// PricePoint represents one collected market datapoint.
type PricePoint struct {
	Exchange    string
	Symbol      string
	Price       float64
	Volume      float64
	CollectedAt time.Time
}

// Candle represents one OHLCV candle for backtesting and analytics.
type Candle struct {
	Exchange  string
	Symbol    string
	Timeframe string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	OpenTime  time.Time
	CloseTime time.Time
}

type symbolRecord struct {
	ID int64
}

// OpenHistoryDB opens or creates a SQLite database and initializes schema.
func OpenHistoryDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if _, err = db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err = goose.SetDialect("sqlite3"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set goose dialect: %w", err)
	}

	goose.SetBaseFS(migrationsFS)

	if err = goose.Up(db, "migrations"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run goose migrations: %w", err)
	}

	return db, nil
}

func ensureSymbolID(ctx context.Context, db *sql.DB, exchange string, symbol string) (int64, error) {
	if exchange == "" || symbol == "" {
		return 0, errors.New("exchange and symbol are required")
	}

	_, err := db.ExecContext(
		ctx,
		`INSERT INTO symbols (exchange, symbol)
		 VALUES (?, ?)
		 ON CONFLICT(exchange, symbol) DO NOTHING`,
		exchange,
		symbol,
	)
	if err != nil {
		return 0, fmt.Errorf("ensure symbol insert: %w", err)
	}

	var record symbolRecord
	if err := db.QueryRowContext(
		ctx,
		`SELECT id FROM symbols WHERE exchange = ? AND symbol = ?`,
		exchange,
		symbol,
	).Scan(&record.ID); err != nil {
		return 0, fmt.Errorf("load symbol id: %w", err)
	}

	return record.ID, nil
}

// SavePricePoint inserts or updates one price point for a market symbol.
func SavePricePoint(ctx context.Context, db *sql.DB, point PricePoint) error {
	if db == nil {
		return errors.New("db is nil")
	}
	if point.Exchange == "" || point.Symbol == "" {
		return errors.New("exchange and symbol are required")
	}
	if point.Price <= 0 {
		return errors.New("price must be greater than 0")
	}
	if point.CollectedAt.IsZero() {
		return errors.New("collected_at is required")
	}

	symbolID, err := ensureSymbolID(ctx, db, point.Exchange, point.Symbol)
	if err != nil {
		return err
	}

	query := `
	INSERT INTO crypto_price_history (symbol_id, price, volume, collected_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(symbol_id, collected_at)
	DO UPDATE SET
		price = excluded.price,
		volume = excluded.volume
	`

	_, err = db.ExecContext(
		ctx,
		query,
		symbolID,
		point.Price,
		point.Volume,
		point.CollectedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("save price point: %w", err)
	}

	return nil
}

// ListRecentPricePoints returns the most recent points for one symbol.
func ListRecentPricePoints(ctx context.Context, db *sql.DB, symbol string, limit int) ([]PricePoint, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.QueryContext(
		ctx,
		`SELECT s.exchange, s.symbol, h.price, COALESCE(h.volume, 0), h.collected_at
		 FROM crypto_price_history h
		 JOIN symbols s ON s.id = h.symbol_id
		 WHERE s.symbol = ?
		 ORDER BY h.collected_at DESC
		 LIMIT ?`,
		symbol,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query recent points: %w", err)
	}
	defer rows.Close()

	points := make([]PricePoint, 0, limit)
	for rows.Next() {
		var p PricePoint
		if err := rows.Scan(&p.Exchange, &p.Symbol, &p.Price, &p.Volume, &p.CollectedAt); err != nil {
			return nil, fmt.Errorf("scan price point: %w", err)
		}
		points = append(points, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return points, nil
}

// SaveCandle inserts or updates one OHLCV candle.
func SaveCandle(ctx context.Context, db *sql.DB, candle Candle) error {
	if db == nil {
		return errors.New("db is nil")
	}
	if candle.Exchange == "" || candle.Symbol == "" || candle.Timeframe == "" {
		return errors.New("exchange, symbol and timeframe are required")
	}
	if candle.Open <= 0 || candle.High <= 0 || candle.Low <= 0 || candle.Close <= 0 {
		return errors.New("ohlc values must be greater than 0")
	}
	if candle.High < candle.Low {
		return errors.New("high cannot be lower than low")
	}
	if candle.OpenTime.IsZero() || candle.CloseTime.IsZero() {
		return errors.New("open_time and close_time are required")
	}
	if !candle.CloseTime.After(candle.OpenTime) {
		return errors.New("close_time must be after open_time")
	}

	symbolID, err := ensureSymbolID(ctx, db, candle.Exchange, candle.Symbol)
	if err != nil {
		return err
	}

	query := `
	INSERT INTO crypto_candles (
		symbol_id, timeframe, open, high, low, close, volume, open_time, close_time
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(symbol_id, timeframe, open_time)
	DO UPDATE SET
		high = excluded.high,
		low = excluded.low,
		close = excluded.close,
		volume = excluded.volume,
		close_time = excluded.close_time
	`

	_, err = db.ExecContext(
		ctx,
		query,
		symbolID,
		candle.Timeframe,
		candle.Open,
		candle.High,
		candle.Low,
		candle.Close,
		candle.Volume,
		candle.OpenTime.UTC(),
		candle.CloseTime.UTC(),
	)
	if err != nil {
		return fmt.Errorf("save candle: %w", err)
	}

	return nil
}

// ListCandlesInRange returns candles for one symbol/timeframe between two timestamps.
func ListCandlesInRange(
	ctx context.Context,
	db *sql.DB,
	symbol string,
	timeframe string,
	from time.Time,
	to time.Time,
	limit int,
) ([]Candle, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}
	if symbol == "" || timeframe == "" {
		return nil, errors.New("symbol and timeframe are required")
	}
	if from.IsZero() || to.IsZero() {
		return nil, errors.New("from and to are required")
	}
	if !to.After(from) {
		return nil, errors.New("to must be after from")
	}
	if limit <= 0 {
		limit = 1000
	}

	rows, err := db.QueryContext(
		ctx,
		`SELECT s.exchange, s.symbol, c.timeframe, c.open, c.high, c.low, c.close, COALESCE(c.volume, 0), c.open_time, c.close_time
		 FROM crypto_candles c
		 JOIN symbols s ON s.id = c.symbol_id
		 WHERE s.symbol = ?
		   AND c.timeframe = ?
		   AND c.open_time >= ?
		   AND c.open_time < ?
		 ORDER BY c.open_time ASC
		 LIMIT ?`,
		symbol,
		timeframe,
		from.UTC(),
		to.UTC(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query candles in range: %w", err)
	}
	defer rows.Close()

	candles := make([]Candle, 0, limit)
	for rows.Next() {
		var c Candle
		if err := rows.Scan(
			&c.Exchange,
			&c.Symbol,
			&c.Timeframe,
			&c.Open,
			&c.High,
			&c.Low,
			&c.Close,
			&c.Volume,
			&c.OpenTime,
			&c.CloseTime,
		); err != nil {
			return nil, fmt.Errorf("scan candle: %w", err)
		}
		candles = append(candles, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candle rows: %w", err)
	}

	return candles, nil
}
