# Karasu - AI Coding Agent Instructions

## Project Overview

**Karasu** is a Go-based cryptocurrency market monitoring bot with a React frontend. It ingests live market data from Bitvavo exchange, stores historical candle data in SQLite, and serves a web dashboard that displays top-performing crypto markets ranked by liquidity and momentum.

### Architecture

**Backend (Go)**:

- `main.go`: Application entry point, initializes exchange client, database, scheduler, and HTTP server
- `main.go`: Orchestration uniquement — DI, démarrage scheduler + serveur HTTP
- `internal/store/`: Interface `CandleStore` + implémentation SQLite (`SQLiteStore` via `NewSQLiteStore`)
- `internal/ingestion/`: `IngestionService` — cache live 1m + jobs d'ingestion/backfill
- `internal/market/`: Ranking des marchés (`TopMarketPositions`, `FindTopSymbols`)
- `internal/scoring/`: Indicateurs techniques (RSI, MACD, Bollinger, SMA) via `ComputeQualityScore`
- `internal/api/server.go`: `NewRouter`, `NewHTTPServer`, `RunHTTPServer` (Gin)
- `internal/api/handlers/`: Un fichier par groupe de routes (`markets.go`, `candles.go`, `backfill.go`)
- `internal/exchange/`: Interface `ExchangeClient` + implémentation Bitvavo
- `internal/scheduler/`: Scheduler gocron pour les jobs périodiques

**Note** : Les fichiers root `candle_store.go`, `market.go`, `ingestion.go`, `http_server.go`, `score.go` sont conservés vides (`package main`) — tout le code est dans `internal/`.

**Frontend (React)**: TypeScript + Vite, fetches `/api/markets` and `/api/live-1m` endpoints

## Data Architecture

**SQLite Schema** (`modernc.org/sqlite` with WAL mode):

- `symbols`: Exchange + symbol pair (unique constraint)
- `crypto_price_history`: One-time price snapshots (unique by exchange, symbol, timestamp)
- `crypto_candles`: OHLCV candles with timeframe (1m stored in-memory, 5m+ persisted)

**Key Constraint**: Prevent duplicate candles via `(exchange, symbol, timeframe, open_time)` unique index.

## Critical Workflows

### 1. Market Analysis Pipeline

**TopMarketPositions()** flow ([internal/market/ranking.go](../internal/market/ranking.go#L83)):

1. Fetch 24h candles for all Bitvavo pairs (`exchangeClient.CandlesLast24h()`)
2. Calculate quote volume (EUR) from `Close × Volume`, filter < 50k EUR threshold
3. Calculate 24h momentum: `(Close - Open) / Open × 100`
4. Rank independently by liquidity & momentum, sum positions, sort by combined score
5. Return top 30 symbols with detailed position rankings

**Why this pattern**: Dual-ranking prevents liquidity-biased results; prevents single whale movement dominating rankings.

### 2. Ingestion Scheduler

Three background jobs managed by `internal/scheduler` ([main.go](../main.go)):

- **refresh universe** (default 15m): `ingestionService.RefreshUniverse()` - updates tracked symbol universe
- **ingest top symbols** (default 1m): `ingestionService.IngestTopSymbols()` - collects live 1m candles, keeps 50-candle in-memory buffer
- **ingest other symbols** (default 5m): `ingestionService.IngestOtherSymbols()` - rotates through non-top symbols

**Memory Strategy**: 1m candles live in-memory (`/api/live-1m`), 5m+ aggregates persist to SQLite asynchronously. This avoids storage bottleneck while maintaining live data freshness.

### 3. Environment Configuration

All intervals use Go duration format (`30s`, `1m`, `5m`, `1h`):

```
KARASU_DB_PATH                    = ./karasu.db
KARASU_REFRESH_UNIVERSE_INTERVAL  = 15m
KARASU_INGEST_TOP_INTERVAL        = 1m
KARASU_INGEST_OTHER_INTERVAL      = 5m
KARASU_INGEST_REPAIR_LOOKBACK     = 6h  # fill gaps in persistence
KARASU_BACKFILL_CHUNK             = 12h # batch size for historical ingestion
```

Loaded via `.env` using `github.com/joho/godotenv`.

## Building & Running

**Backend**:

```bash
go run .              # Starts scheduler + HTTP server (port from PORT env var)
```

**Frontend** (from `web/` directory):

```bash
npm install && npm run dev    # Vite dev server (localhost:5173)
npm run build && npm run lint # Production build + ESLint validation
```

## API Contracts

**GET /api/markets**

- Returns ranked markets sorted by combined position score
- Response: `Market[]` with fields: `symbol`, `quoteVolume`, `change24h`, `change5m`, `change1h`, positions for each

**GET /api/live-1m?symbols=BTC,ETH&limit=20**

- Returns in-memory 1m candles for requested symbols
- Filters by symbol list, caps at `limit` (default 50)
- Response: `LiveCandle[]` with OHLCV data + `updatedAt` timestamp

## Code Patterns

### Go Conventions

- **Error wrapping**: Always use `fmt.Errorf("context: %w", err)` for error chains
- **Logging**: Use `slog` with structured fields (e.g., `slog.Info("event", "key", value)`)
- **Database access**: Lock writes via `CandleStore.writeMu` (SQLite single-writer constraint)
- **Time**: Always operate in UTC (`time.Local = time.UTC` in main)

### Frontend Conventions

- Fetch data on mount + setup polling intervals
- Type definitions mirrored from backend structs (`Market`, `LiveCandle`)
- No build state management beyond React hooks (useState, useMemo)

## Extension Points

1. **New Exchange**: Implement `exchange.ExchangeClient` interface (see `internal/exchange/exchange.go`)
2. **New API Endpoint**: Add to `http_server.go` router before `s.Run()`
3. **New Ingestion Job**: Add to scheduler in `main.go` via `s.AddJob(name, interval, task)`
4. **Schema Migration**: Create numbered SQL files + embed in Go via `//go:embed`

## Debugging Tips

- **Check scheduler health**: Look for "job completed" vs "job failed" in stderr (slog output)
- **Database locks**: Monitor `karasu.db-wal` file growth; indicates pending writes
- **Missing candles**: Verify `KARASU_INGEST_REPAIR_LOOKBACK` covers gaps
- **Frontend stale data**: Ensure `/api/live-1m` polling interval matches ingestion frequency
