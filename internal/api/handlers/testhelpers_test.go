package handlers

import (
	"time"

	"karasu/internal/exchange"
	"karasu/internal/ingestion"
	"karasu/internal/store"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- fake exchange client ---

type fakeExchangeClient struct {
	wallet            exchange.Wallet
	walletErr         error
	candlesLast24h    []exchange.CandleBundle
	candlesLast24hErr error
	candles1m         map[string]exchange.CandleBundle
	candles1mErr      map[string]error
	placeOrderResult  exchange.OrderResult
	placeOrderErr     error
}

func (f *fakeExchangeClient) Symbols() (map[string]string, error) {
	return map[string]string{}, nil
}

func (f *fakeExchangeClient) Prices() (map[string]float64, error) {
	return map[string]float64{}, nil
}

func (f *fakeExchangeClient) Wallet() (exchange.Wallet, error) {
	if f.walletErr != nil {
		return exchange.Wallet{}, f.walletErr
	}
	return f.wallet, nil
}

func (f *fakeExchangeClient) Candles1mByDate(symbol string, date time.Time) (exchange.CandleBundle, error) {
	if err, ok := f.candles1mErr[symbol]; ok {
		return exchange.CandleBundle{}, err
	}
	if b, ok := f.candles1m[symbol]; ok {
		return b, nil
	}
	return exchange.CandleBundle{}, nil
}

func (f *fakeExchangeClient) Candles5mByDate(symbol string, date time.Time) (exchange.CandleBundle, error) {
	return exchange.CandleBundle{}, nil
}

func (f *fakeExchangeClient) CandlesLast24h() ([]exchange.CandleBundle, error) {
	if f.candlesLast24hErr != nil {
		return nil, f.candlesLast24hErr
	}
	if f.candlesLast24h != nil {
		return f.candlesLast24h, nil
	}
	return []exchange.CandleBundle{}, nil
}

func (f *fakeExchangeClient) Candles(symbol string, start time.Time, end time.Time, interval exchange.Interval) (exchange.CandleBundle, error) {
	return exchange.CandleBundle{}, nil
}

func (f *fakeExchangeClient) PlaceMarketOrder(symbol string, side string, amountEUR float64) (exchange.OrderResult, error) {
	if f.placeOrderErr != nil {
		return exchange.OrderResult{}, f.placeOrderErr
	}
	return f.placeOrderResult, nil
}

// --- fake candle store ---

type fakeCandleStore struct {
	candles map[string][]exchange.Candle
	err     error
}

func (f *fakeCandleStore) UpsertCandles(exchangeName, symbol, timeframe string, candles []exchange.Candle) error {
	return nil
}

func (f *fakeCandleStore) QueryCandles(exchangeName, symbol, timeframe string, limit int) ([]exchange.Candle, error) {
	if f.err != nil {
		return nil, f.err
	}
	result := append([]exchange.Candle(nil), f.candles[symbol]...)
	if limit > 0 && len(result) > limit {
		return result[len(result)-limit:], nil
	}
	return result, nil
}

func (f *fakeCandleStore) LastCandleOpenTime(exchangeName, symbol, timeframe string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (f *fakeCandleStore) QueryDailySymbolActivity(exchangeName, timeframe string, days int) ([]store.DailySymbolActivity, error) {
	return nil, nil
}

func (f *fakeCandleStore) Close() error {
	return nil
}

// --- helpers ---

func newIngestionService(ec exchange.ExchangeClient, cs store.CandleStore) *ingestion.IngestionService {
	return ingestion.NewIngestionService(ec, cs)
}

func build24hBundle(symbol string, open, close, volume float64) exchange.CandleBundle {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	return exchange.CandleBundle{
		Symbol:   symbol,
		Interval: exchange.Interval1d,
		Candles: []exchange.Candle{{
			Open:      open,
			High:      close * 1.01,
			Low:       open * 0.99,
			Close:     close,
			Volume:    volume,
			OpenTime:  now,
			CloseTime: now.Add(24 * time.Hour),
		}},
	}
}

func build1mBundle(symbol string, count int, startPrice float64) exchange.CandleBundle {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]exchange.Candle, count)
	for i := range candles {
		open := startPrice + float64(i)
		candles[i] = exchange.Candle{
			Open:      open,
			High:      open + 1,
			Low:       open - 0.5,
			Close:     open + 0.6,
			Volume:    1000 + float64(i)*10,
			OpenTime:  now.Add(time.Duration(i) * time.Minute),
			CloseTime: now.Add(time.Duration(i+1) * time.Minute),
		}
	}
	return exchange.CandleBundle{Symbol: symbol, Interval: exchange.Interval1m, Candles: candles}
}
