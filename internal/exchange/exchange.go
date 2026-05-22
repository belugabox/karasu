package exchange

import (
	"fmt"
	"sort"
	"time"
)

type CandleBundle struct {
	Symbol    string    // symbol, e.g., "BTC"
	Interval  Interval  // interval, e.g., "1m", "5m", "1h", "1d"
	Candles   []Candle  // list of candles for the symbol and interval
	StartTime time.Time // start time of the bundle (e.g., when the first candle opens)
	EndTime   time.Time // end time of the bundle (e.g., when the last candle closes)
}

type Candle struct {
	Open      float64   // open price
	High      float64   // highest price
	Low       float64   // lowest price
	Close     float64   // close price
	Volume    float64   // traded volume during the candle period
	OpenTime  time.Time // time when the candle opened
	CloseTime time.Time // time when the candle closed
}

type Interval string

const (
	// 1m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 1W, 1M
	Interval1m  Interval = "1m"
	Interval5m  Interval = "5m"
	Interval15m Interval = "15m"
	Interval30m Interval = "30m"
	Interval1h  Interval = "1h"
	Interval2h  Interval = "2h"
	Interval4h  Interval = "4h"
	Interval6h  Interval = "6h"
	Interval8h  Interval = "8h"
	Interval12h Interval = "12h"
	Interval1d  Interval = "1d"
	Interval1W  Interval = "1W"
	Interval1M  Interval = "1M"
)

type Wallet struct {
	TotalValue float64       // total value of the wallet in EUR
	CashValue  float64       // total value of the cash in EUR
	AssetValue float64       // total value of the assets in EUR
	Assets     []WalletAsset // list of assets in the wallet
}

type WalletAsset struct {
	Symbol        string  // symbol of the asset, e.g., "BTC"
	Amount        float64 // amount of the asset
	StakingAmount float64 // amount of the asset that is staked (if applicable)
	Value         float64 // value of the asset in EUR
}

type ExchangeClient interface {
	Symbols() (map[string]string, error) // symbol to label, e.g., "BTC" to "Bitcoin"
	Prices() (map[string]float64, error) // symbol to price in EUR
	Wallet() (Wallet, error)

	Candles1mByDate(symbol string, date time.Time) (CandleBundle, error)
	Candles5mByDate(symbol string, date time.Time) (CandleBundle, error)
	CandlesLast24h() ([]CandleBundle, error)
	Candles(symbol string, start time.Time, end time.Time, interval Interval) (CandleBundle, error)
}

func intervalDuration(interval Interval) time.Duration {
	switch interval {
	case Interval1m:
		return time.Minute
	case Interval5m:
		return 5 * time.Minute
	case Interval15m:
		return 15 * time.Minute
	case Interval30m:
		return 30 * time.Minute
	case Interval1h:
		return time.Hour
	case Interval2h:
		return 2 * time.Hour
	case Interval4h:
		return 4 * time.Hour
	case Interval6h:
		return 6 * time.Hour
	case Interval8h:
		return 8 * time.Hour
	case Interval12h:
		return 12 * time.Hour
	case Interval1d:
		return 24 * time.Hour
	case Interval1W:
		return 7 * 24 * time.Hour
	case Interval1M:
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}

// Aggregate1mTo aggregates a 1m candle bundle into a higher interval bundle.
func Aggregate1mTo(bundle CandleBundle, interval Interval) (CandleBundle, error) {
	if bundle.Interval != Interval1m {
		return CandleBundle{}, fmt.Errorf("Aggregate1mTo expects 1m source bundle, got %s", bundle.Interval)
	}
	return AggregateTo(bundle, interval)
}

// AggregateTo aggregates a candle bundle from any supported source interval
// to a larger target interval (for example 1m->5m, 5m->1h, 1h->1d).
func AggregateTo(bundle CandleBundle, interval Interval) (CandleBundle, error) {
	sourceDuration := intervalDuration(bundle.Interval)
	if sourceDuration == 0 {
		return CandleBundle{}, fmt.Errorf("unsupported source interval: %s", bundle.Interval)
	}

	targetDuration := intervalDuration(interval)
	if targetDuration == 0 {
		return CandleBundle{}, fmt.Errorf("unsupported interval: %s", interval)
	}
	if targetDuration <= sourceDuration {
		return CandleBundle{}, fmt.Errorf("target interval must be greater than source interval (%s -> %s)", bundle.Interval, interval)
	}
	if targetDuration%sourceDuration != 0 {
		return CandleBundle{}, fmt.Errorf("target interval must be a multiple of source interval (%s -> %s)", bundle.Interval, interval)
	}

	if len(bundle.Candles) == 0 {
		return CandleBundle{
			Symbol:    bundle.Symbol,
			Interval:  interval,
			Candles:   []Candle{},
			StartTime: bundle.StartTime.UTC().Truncate(targetDuration),
			EndTime:   bundle.EndTime.UTC().Truncate(targetDuration),
		}, nil
	}

	source := make([]Candle, len(bundle.Candles))
	copy(source, bundle.Candles)
	sort.Slice(source, func(i, j int) bool {
		return source[i].OpenTime.Before(source[j].OpenTime)
	})

	result := make([]Candle, 0, len(source)/60+1)

	currentBucket := source[0].OpenTime.UTC().Truncate(targetDuration)
	current := Candle{
		Open:      source[0].Open,
		High:      source[0].High,
		Low:       source[0].Low,
		Close:     source[0].Close,
		Volume:    source[0].Volume,
		OpenTime:  currentBucket,
		CloseTime: currentBucket.Add(targetDuration),
	}

	flush := func() {
		result = append(result, current)
	}

	for i := 1; i < len(source); i++ {
		c := source[i]
		bucket := c.OpenTime.UTC().Truncate(targetDuration)

		if !bucket.Equal(currentBucket) {
			flush()
			currentBucket = bucket
			current = Candle{
				Open:      c.Open,
				High:      c.High,
				Low:       c.Low,
				Close:     c.Close,
				Volume:    c.Volume,
				OpenTime:  currentBucket,
				CloseTime: currentBucket.Add(targetDuration),
			}
			continue
		}

		if c.High > current.High {
			current.High = c.High
		}
		if c.Low < current.Low {
			current.Low = c.Low
		}
		current.Close = c.Close
		current.Volume += c.Volume
		current.CloseTime = currentBucket.Add(targetDuration)
	}

	flush()

	start := result[0].OpenTime
	end := result[len(result)-1].CloseTime

	if len(bundle.Candles) >= 2 && bundle.Candles[0].OpenTime.After(bundle.Candles[1].OpenTime) {
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
		start = result[len(result)-1].OpenTime
		end = result[0].CloseTime
	}

	return CandleBundle{
		Symbol:    bundle.Symbol,
		Interval:  interval,
		Candles:   result,
		StartTime: start,
		EndTime:   end,
	}, nil
}
