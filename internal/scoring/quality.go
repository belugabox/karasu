package scoring

import (
	"math"
	"time"
)

// Candle is the scoring package's own OHLCV representation,
// independent of exchange.Candle to keep this package self-contained.
type Candle struct {
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	OpenTime  time.Time
	CloseTime time.Time
}

type QualityBreakdown struct {
	Score     float64
	RSI       float64
	MACD      float64
	Bollinger float64
	Volume    float64
	SMA       float64
}

// ComputeQualityScore returns a 0-100 momentum/quality score for a candle series.
// timeframe can be "short", "medium", or anything else (treated as "long").
func ComputeQualityScore(candles []Candle, timeframe string) float64 {
	return ComputeQualityBreakdown(candles, timeframe).Score
}

// ComputeQualityBreakdown returns the total score and the weighted indicator components.
func ComputeQualityBreakdown(candles []Candle, timeframe string) QualityBreakdown {
	if len(candles) < 30 {
		return QualityBreakdown{}
	}

	closes := candleCloses(candles)
	volumes := candleVolumes(candles)
	price := closes[len(closes)-1]

	rsiVal := rsiScore(closes)
	macdVal := macdScore(closes)
	bollingerVal := bollingerScore(closes, price)
	volumeVal := volumeScore(volumes)
	smaVal := smaTrendScore(closes)

	breakdown := QualityBreakdown{
		RSI:       rsiVal,
		MACD:      macdVal,
		Bollinger: bollingerVal,
		Volume:    volumeVal,
		SMA:       smaVal,
	}

	switch timeframe {
	case "short":
		breakdown.Score = clamp(rsiVal*0.30+macdVal*0.40+bollingerVal*0.10+volumeVal*0.20, 0, 100)
	case "medium":
		breakdown.Score = clamp(rsiVal*0.25+macdVal*0.35+bollingerVal*0.20+volumeVal*0.10+smaVal*0.10, 0, 100)
	default:
		breakdown.Score = clamp(rsiVal*0.20+macdVal*0.30+bollingerVal*0.15+volumeVal*0.05+smaVal*0.30, 0, 100)
	}

	return breakdown
}

func candleCloses(candles []Candle) []float64 {
	out := make([]float64, 0, len(candles))
	for _, c := range candles {
		out = append(out, c.Close)
	}
	return out
}

func candleVolumes(candles []Candle) []float64 {
	out := make([]float64, 0, len(candles))
	for _, c := range candles {
		out = append(out, c.Volume)
	}
	return out
}

func rsiScore(closes []float64) float64 {
	r := rsi(closes, 14)
	if r == 0 {
		return 50
	}
	return clamp(100-math.Abs(r-55)*2, 0, 100)
}

func macdScore(closes []float64) float64 {
	macdLine, signal := macdCalc(closes)
	if len(macdLine) == 0 || len(signal) == 0 {
		return 50
	}
	hist := macdLine[len(macdLine)-1] - signal[len(signal)-1]
	price := closes[len(closes)-1]
	if price <= 0 {
		return 50
	}
	pct := (hist / price) * 100
	return clamp(50+pct*120, 0, 100)
}

func bollingerScore(closes []float64, price float64) float64 {
	mid, up, low := bollingerCalc(closes, 20, 2)
	if up <= low || price <= 0 {
		return 50
	}
	if price < low {
		return 35
	}
	if price > up {
		return 35
	}
	distance := math.Abs(price-mid) / (up - low)
	return clamp(100-distance*140, 0, 100)
}

func volumeScore(volumes []float64) float64 {
	if len(volumes) < 20 {
		return 50
	}
	last := volumes[len(volumes)-1]
	sma20 := sma(volumes, 20)
	if sma20 <= 0 {
		return 50
	}
	ratio := last / sma20
	return clamp(ratio*55, 0, 100)
}

func smaTrendScore(closes []float64) float64 {
	if len(closes) < 60 {
		return 50
	}
	fast := sma(closes, 20)
	slow := sma(closes, 50)
	last := closes[len(closes)-1]
	if slow <= 0 || last <= 0 {
		return 50
	}
	base := 50 + ((last-slow)/slow)*300
	if fast > slow {
		base += 10
	}
	return clamp(base, 0, 100)
}

func sma(values []float64, period int) float64 {
	if len(values) < period || period <= 0 {
		return 0
	}
	sum := 0.0
	for i := len(values) - period; i < len(values); i++ {
		sum += values[i]
	}
	return sum / float64(period)
}

func ema(values []float64, period int) []float64 {
	if len(values) == 0 || period <= 0 {
		return nil
	}
	alpha := 2.0 / float64(period+1)
	out := make([]float64, len(values))
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

func macdCalc(closes []float64) ([]float64, []float64) {
	if len(closes) < 35 {
		return nil, nil
	}
	ema12 := ema(closes, 12)
	ema26 := ema(closes, 26)
	line := make([]float64, len(closes))
	for i := range closes {
		line[i] = ema12[i] - ema26[i]
	}
	signal := ema(line, 9)
	return line, signal
}

func rsi(closes []float64, period int) float64 {
	if len(closes) <= period {
		return 0
	}
	gain := 0.0
	loss := 0.0
	for i := len(closes) - period; i < len(closes); i++ {
		delta := closes[i] - closes[i-1]
		if delta > 0 {
			gain += delta
		} else {
			loss -= delta
		}
	}
	if loss == 0 {
		return 100
	}
	rs := (gain / float64(period)) / (loss / float64(period))
	return 100 - (100 / (1 + rs))
}

func bollingerCalc(closes []float64, period int, deviation float64) (float64, float64, float64) {
	if len(closes) < period {
		return 0, 0, 0
	}
	window := closes[len(closes)-period:]
	mean := sma(window, period)
	variance := 0.0
	for _, value := range window {
		variance += math.Pow(value-mean, 2)
	}
	std := math.Sqrt(variance / float64(period))
	upper := mean + deviation*std
	lower := mean - deviation*std
	return mean, upper, lower
}

func clamp(value, minV, maxV float64) float64 {
	if value < minV {
		return minV
	}
	if value > maxV {
		return maxV
	}
	return value
}
