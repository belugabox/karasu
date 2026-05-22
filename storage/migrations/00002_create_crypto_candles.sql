-- +goose Up
CREATE TABLE IF NOT EXISTS crypto_candles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    exchange TEXT NOT NULL,
    symbol TEXT NOT NULL,
    timeframe TEXT NOT NULL,
    open REAL NOT NULL,
    high REAL NOT NULL,
    low REAL NOT NULL,
    close REAL NOT NULL,
    volume REAL,
    open_time DATETIME NOT NULL,
    close_time DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK(high >= low),
    CHECK(open > 0),
    CHECK(high > 0),
    CHECK(low > 0),
    CHECK(close > 0),
    UNIQUE(exchange, symbol, timeframe, open_time)
);

CREATE INDEX IF NOT EXISTS idx_crypto_candles_symbol_tf_time
ON crypto_candles(symbol, timeframe, open_time DESC);

CREATE INDEX IF NOT EXISTS idx_crypto_candles_exchange_time
ON crypto_candles(exchange, open_time DESC);

-- +goose Down
DROP TABLE IF EXISTS crypto_candles;
