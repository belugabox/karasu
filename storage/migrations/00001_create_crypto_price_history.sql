-- +goose Up
CREATE TABLE IF NOT EXISTS crypto_price_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    exchange TEXT NOT NULL,
    symbol TEXT NOT NULL,
    price REAL NOT NULL,
    volume REAL,
    collected_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(exchange, symbol, collected_at)
);

CREATE INDEX IF NOT EXISTS idx_crypto_history_symbol_time
ON crypto_price_history(symbol, collected_at DESC);

CREATE INDEX IF NOT EXISTS idx_crypto_history_exchange_time
ON crypto_price_history(exchange, collected_at DESC);

-- +goose Down
DROP TABLE IF EXISTS crypto_price_history;
