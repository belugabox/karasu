-- +goose Up
PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS symbols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    exchange TEXT NOT NULL,
    symbol TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(exchange, symbol)
);

INSERT OR IGNORE INTO symbols (exchange, symbol)
SELECT exchange, symbol
FROM (
    SELECT exchange, symbol FROM crypto_price_history
    UNION
    SELECT exchange, symbol FROM crypto_candles
);

CREATE TABLE crypto_price_history_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol_id INTEGER NOT NULL,
    price REAL NOT NULL,
    volume REAL,
    collected_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON DELETE CASCADE,
    UNIQUE(symbol_id, collected_at)
);

INSERT INTO crypto_price_history_new (id, symbol_id, price, volume, collected_at, created_at)
SELECT h.id, s.id, h.price, h.volume, h.collected_at, h.created_at
FROM crypto_price_history h
JOIN symbols s
  ON s.exchange = h.exchange
 AND s.symbol = h.symbol;

DROP TABLE crypto_price_history;
ALTER TABLE crypto_price_history_new RENAME TO crypto_price_history;

CREATE INDEX idx_crypto_history_symbol_time
ON crypto_price_history(symbol_id, collected_at DESC);

CREATE TABLE crypto_candles_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol_id INTEGER NOT NULL,
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
    FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON DELETE CASCADE,
    UNIQUE(symbol_id, timeframe, open_time)
);

INSERT INTO crypto_candles_new (id, symbol_id, timeframe, open, high, low, close, volume, open_time, close_time, created_at)
SELECT c.id, s.id, c.timeframe, c.open, c.high, c.low, c.close, c.volume, c.open_time, c.close_time, c.created_at
FROM crypto_candles c
JOIN symbols s
  ON s.exchange = c.exchange
 AND s.symbol = c.symbol;

DROP TABLE crypto_candles;
ALTER TABLE crypto_candles_new RENAME TO crypto_candles;

CREATE INDEX idx_crypto_candles_symbol_tf_time
ON crypto_candles(symbol_id, timeframe, open_time DESC);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

CREATE TABLE crypto_price_history_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    exchange TEXT NOT NULL,
    symbol TEXT NOT NULL,
    price REAL NOT NULL,
    volume REAL,
    collected_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(exchange, symbol, collected_at)
);

INSERT INTO crypto_price_history_old (id, exchange, symbol, price, volume, collected_at, created_at)
SELECT h.id, s.exchange, s.symbol, h.price, h.volume, h.collected_at, h.created_at
FROM crypto_price_history h
JOIN symbols s ON s.id = h.symbol_id;

DROP TABLE crypto_price_history;
ALTER TABLE crypto_price_history_old RENAME TO crypto_price_history;

CREATE INDEX idx_crypto_history_symbol_time
ON crypto_price_history(symbol, collected_at DESC);

CREATE INDEX idx_crypto_history_exchange_time
ON crypto_price_history(exchange, collected_at DESC);

CREATE TABLE crypto_candles_old (
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

INSERT INTO crypto_candles_old (id, exchange, symbol, timeframe, open, high, low, close, volume, open_time, close_time, created_at)
SELECT c.id, s.exchange, s.symbol, c.timeframe, c.open, c.high, c.low, c.close, c.volume, c.open_time, c.close_time, c.created_at
FROM crypto_candles c
JOIN symbols s ON s.id = c.symbol_id;

DROP TABLE crypto_candles;
ALTER TABLE crypto_candles_old RENAME TO crypto_candles;

CREATE INDEX idx_crypto_candles_symbol_tf_time
ON crypto_candles(symbol, timeframe, open_time DESC);

CREATE INDEX idx_crypto_candles_exchange_time
ON crypto_candles(exchange, open_time DESC);

DROP TABLE symbols;

PRAGMA foreign_keys = ON;
