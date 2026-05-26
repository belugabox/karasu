package store

import (
	"time"

	"karasu/internal/exchange"
)

type DailySymbolActivity struct {
	Day         string `json:"day"`
	SymbolCount int    `json:"symbolCount"`
	CandleCount int    `json:"candleCount"`
}

// CandleStore is the abstraction over the persistence layer for OHLCV candles.
// Use NewSQLiteStore to obtain a concrete implementation.
type CandleStore interface {
	UpsertCandles(exchangeName, symbol, timeframe string, candles []exchange.Candle) error
	QueryCandles(exchangeName, symbol, timeframe string, limit int) ([]exchange.Candle, error)
	LastCandleOpenTime(exchangeName, symbol, timeframe string) (time.Time, bool, error)
	QueryDailySymbolActivity(exchangeName, timeframe string, days int) ([]DailySymbolActivity, error)
	Close() error
}

// AlertSeverity classifies how urgent an alert is.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertEvent records a system or market alert that has been raised or resolved.
type AlertEvent struct {
	ID        string        `json:"id"`
	Key       string        `json:"key"`
	Category  string        `json:"category"`
	Severity  AlertSeverity `json:"severity"`
	Message   string        `json:"message"`
	Source    string        `json:"source"`
	Symbol    string        `json:"symbol,omitempty"`
	Active    bool          `json:"active"`
	Count     int           `json:"count"`
	FirstSeen time.Time     `json:"firstSeen"`
	LastSeen  time.Time     `json:"lastSeen"`
}

// AlertStore is the abstraction over the persistence layer for alert history.
// Use NewSQLiteStore to obtain a concrete implementation.
type AlertStore interface {
	// UpsertAlert inserts or updates an alert keyed by AlertEvent.Key.
	UpsertAlert(alert AlertEvent) error
	// ListAlerts returns a page of alerts ordered by (active DESC, last_seen DESC).
	// It also returns the total number of matching rows for pagination.
	ListAlerts(limit, offset int, activeOnly bool) (alerts []AlertEvent, total int, err error)
	// PruneAlerts removes alerts whose last_seen is older than the given threshold.
	PruneAlerts(before time.Time) error
}
