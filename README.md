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
