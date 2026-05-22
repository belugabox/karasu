package store

import (
	"time"

	"karasu/internal/exchange"
)

// CandleStore is the abstraction over the persistence layer for OHLCV candles.
// Use NewSQLiteStore to obtain a concrete implementation.
type CandleStore interface {
	UpsertCandles(exchangeName, symbol, timeframe string, candles []exchange.Candle) error
	QueryCandles(exchangeName, symbol, timeframe string, limit int) ([]exchange.Candle, error)
	LastCandleOpenTime(exchangeName, symbol, timeframe string) (time.Time, bool, error)
	Close() error
}
