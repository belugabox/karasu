# karasu

Karasu est un bot crypto (Go) avec persistance SQLite pour historiser les donnees financieres.

## Base de donnees

- Driver: SQLite (`modernc.org/sqlite`)
- Migration manager: Goose (`github.com/pressly/goose/v3`)
- Fichier DB par defaut: `./karasu.db`
- Variable d'environnement optionnelle: `KARASU_DB_PATH`

La table `symbols` centralise les marches suivis:

- `id`
- `exchange`
- `symbol`

Contrainte d'unicite: `(exchange, symbol)`.
Les tables `crypto_price_history` et `crypto_candles` pointent vers `symbols.id` via `symbol_id`.

La table `crypto_price_history` stocke:

- `exchange` (ex: bitvavo)
- `symbol` (ex: BTC-EUR)
- `price`
- `volume`
- `collected_at`

Contrainte d'unicite: `(exchange, symbol, collected_at)` pour eviter les doublons.

La table `crypto_candles` stocke les bougies OHLCV:

- `exchange`
- `symbol`
- `timeframe` (ex: 1m, 5m, 1h)
- `open`, `high`, `low`, `close`, `volume`
- `open_time`, `close_time`

Contrainte d'unicite: `(exchange, symbol, timeframe, open_time)`.

## Migrations Goose

- Les migrations SQL sont dans `storage/migrations/`
- Elles sont embarquees dans le binaire Go via `embed`
- Au demarrage, l'application lance automatiquement `goose up`

Migration initiale:

- `storage/migrations/00001_create_crypto_price_history.sql`
- `storage/migrations/00002_create_crypto_candles.sql`
- `storage/migrations/00003_create_symbols_and_link_history.sql`

Optionnel (CLI Goose):

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

## Lancer

```bash
go run .
```

Le programme initialise le schema, insere un point de prix de demonstration, puis lit les 5 derniers points.
Il insere aussi une bougie OHLCV de demonstration et charge les bougies sur un intervalle de temps.

## Ingestion live 1m + stockage 5m

Le backend execute des jobs planifies:

- `refresh universe` toutes les 15 min (univers EUR + top symbols)
- `ingest top symbols` toutes les 1 min (cache live 1m + persistance 5m)
- `ingest other symbols` toutes les 5 min (rotation par batch)

Principes:

- Les bougies 1m sont maintenues en memoire pour le direct frontend (`/api/live-1m`)
- Les bougies 5m closes sont agregees et upsertees en SQLite (`crypto_candles`)
- Contrainte d'unicite: `(exchange, symbol, timeframe, open_time)`

Variable d'environnement:

- `KARASU_DB_PATH` (optionnel, defaut: `./karasu.db`)
- `KARASU_TELEGRAM_BOT_TOKEN` + `KARASU_TELEGRAM_CHAT_ID` (optionnels, activent l'envoi des alertes dédoublonnées vers Telegram)
- `KARASU_REFRESH_UNIVERSE_INTERVAL` (optionnel, defaut: `15m`)
- `KARASU_INGEST_TOP_INTERVAL` (optionnel, defaut: `1m`)
- `KARASU_INGEST_OTHER_INTERVAL` (optionnel, defaut: `5m`)
- `KARASU_INGEST_REPAIR_LOOKBACK` (optionnel, defaut: `6h`)
- `KARASU_BACKFILL_CHUNK` (optionnel, defaut: `12h`)

Les durees utilisent le format Go (`30s`, `1m`, `5m`, `1h`).

## API

- `GET /api/markets`
	- top marchés classes (liquidite/momentum)

- `GET /api/opportunities?limit=15`
	- retourne les opportunites priorisees par score metier
	- inclut resume, action primaire, convergence, fraicheur, raisons et risques

- `GET /api/system-health?staleThresholdMin=20`
	- retourne un snapshot de sante systeme (fraicheur live, retards 5m, etat backfill)
	- inclut une liste d'issues actionnables si le systeme est degrade

- `GET /api/alerts/recent?limit=50&activeOnly=false`
	- retourne l'historique recent des alertes dedoublonnees (exchange, health, backfill)
	- `activeOnly=true` pour filtrer uniquement les alertes actives

- `GET /api/live-1m?symbols=BTC,ETH&limit=20`
	- retourne le dernier snapshot 1m en memoire

- `GET /api/candles-5m?symbol=BTC&limit=500`
	- retourne l'historique 5m persiste (ordre chronologique)

- `POST /api/backfill-5m?symbols=BTC,ETH&from=2026-05-01T00:00:00Z&to=2026-05-22T00:00:00Z`
	- lance un backfill longue plage en agregant du 1m vers 5m
	- `from` et `to` acceptent RFC3339 ou unix milliseconds
	- si `symbols` est omis, le backfill cible tout l'univers courant

## Telegram

Si `KARASU_TELEGRAM_BOT_TOKEN` et `KARASU_TELEGRAM_CHAT_ID` sont renseignés, Karasu pousse les transitions d'alertes dédoublonnées vers Telegram :

- apparition d'une nouvelle alerte
- changement de sévérité, message, source ou symbole
- resolution d'une alerte

## Roadmap

La roadmap quasi production et le decoupage par sprint sont decrits dans `docs/roadmap-quasi-production.md`.
