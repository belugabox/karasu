package scoring

import (
	"math"
	"testing"
	"time"
)

func TestComputeQualityBreakdownReturnsZeroForShortSeries(t *testing.T) {
	t.Parallel()

	breakdown := ComputeQualityBreakdown(buildCandles(29, 100, 0.8, 1000, 10), "medium")

	if breakdown != (QualityBreakdown{}) {
		t.Fatalf("expected empty breakdown for short series, got %#v", breakdown)
	}
}

func TestComputeQualityBreakdownAppliesTimeframeWeights(t *testing.T) {
	t.Parallel()

	candles := buildCandles(80, 100, 1.2, 1000, 25)

	short := ComputeQualityBreakdown(candles, "short")
	medium := ComputeQualityBreakdown(candles, "medium")
	long := ComputeQualityBreakdown(candles, "long")

	assertClose(t, short.Score, clamp(short.RSI*0.30+short.MACD*0.40+short.Bollinger*0.10+short.Volume*0.20, 0, 100))
	assertClose(t, medium.Score, clamp(medium.RSI*0.25+medium.MACD*0.35+medium.Bollinger*0.20+medium.Volume*0.10+medium.SMA*0.10, 0, 100))
	assertClose(t, long.Score, clamp(long.RSI*0.20+long.MACD*0.30+long.Bollinger*0.15+long.Volume*0.05+long.SMA*0.30, 0, 100))

	assertClose(t, short.RSI, medium.RSI)
	assertClose(t, medium.RSI, long.RSI)
	assertClose(t, short.MACD, medium.MACD)
	assertClose(t, medium.MACD, long.MACD)
	assertClose(t, short.Bollinger, medium.Bollinger)
	assertClose(t, medium.Bollinger, long.Bollinger)
	assertClose(t, short.Volume, medium.Volume)
	assertClose(t, medium.Volume, long.Volume)
	assertClose(t, short.SMA, medium.SMA)
	assertClose(t, medium.SMA, long.SMA)

	if got := ComputeQualityScore(candles, "medium"); math.Abs(got-medium.Score) > 1e-9 {
		t.Fatalf("expected ComputeQualityScore to match breakdown score, got %.12f want %.12f", got, medium.Score)
	}
}

func TestComputeQualityBreakdownSnapshot(t *testing.T) {
	t.Parallel()

	candles := buildCandles(80, 100, 1.2, 1000, 25)

	short := ComputeQualityBreakdown(candles, "short")
	medium := ComputeQualityBreakdown(candles, "medium")
	long := ComputeQualityBreakdown(candles, "long")

	assertClose(t, short.Score, 39.583817592324)
	assertClose(t, short.RSI, 10)
	assertClose(t, short.MACD, 50.989402475289)
	assertClose(t, short.Bollinger, 42.337187026646)
	assertClose(t, short.Volume, 59.771689497717)
	assertClose(t, short.SMA, 100)

	assertClose(t, medium.Score, 44.790897221452)
	assertClose(t, medium.RSI, 10)
	assertClose(t, medium.MACD, 50.989402475289)
	assertClose(t, medium.Bollinger, 42.337187026646)
	assertClose(t, medium.Volume, 59.771689497717)
	assertClose(t, medium.SMA, 100)

	assertClose(t, long.Score, 56.635983271469)
	assertClose(t, long.RSI, 10)
	assertClose(t, long.MACD, 50.989402475289)
	assertClose(t, long.Bollinger, 42.337187026646)
	assertClose(t, long.Volume, 59.771689497717)
	assertClose(t, long.SMA, 100)
}

func TestVolumeScoreClampsHighRatio(t *testing.T) {
	t.Parallel()

	volumes := make([]float64, 20)
	for i := 0; i < 19; i++ {
		volumes[i] = 100
	}
	volumes[19] = 1000

	if got := volumeScore(volumes); got != 100 {
		t.Fatalf("expected volume score to clamp to 100, got %.2f", got)
	}
}

func TestBollingerScoreReturnsPenaltyOutsideBands(t *testing.T) {
	t.Parallel()

	closes := []float64{
		100, 101, 99, 100, 100,
		101, 99, 100, 100, 100,
		100, 101, 99, 100, 100,
		101, 99, 100, 100, 100,
	}

	_, upper, lower := bollingerCalc(closes, 20, 2)
	if upper <= lower {
		t.Fatalf("expected valid bands, got upper %.4f lower %.4f", upper, lower)
	}

	if got := bollingerScore(closes, upper+1); got != 35 {
		t.Fatalf("expected price above upper band to score 35, got %.2f", got)
	}

	if got := bollingerScore(closes, lower-1); got != 35 {
		t.Fatalf("expected price below lower band to score 35, got %.2f", got)
	}
}

func TestSMATrendScoreHandlesInsufficientAndBullishSeries(t *testing.T) {
	t.Parallel()

	if got := smaTrendScore([]float64{1, 2, 3}); got != 50 {
		t.Fatalf("expected insufficient close history to return 50, got %.2f", got)
	}

	closes := make([]float64, 60)
	for i := range closes {
		closes[i] = 100 + float64(i)*2
	}

	got := smaTrendScore(closes)
	if got <= 50 {
		t.Fatalf("expected bullish trend score above neutral, got %.2f", got)
	}
}

func TestRSIScoreEdgeCases(t *testing.T) {
	t.Parallel()

	if got := rsiScore([]float64{1, 2, 3}); got != 50 {
		t.Fatalf("expected insufficient RSI history to return neutral 50, got %.2f", got)
	}

	steadyRise := make([]float64, 0, 20)
	for i := 0; i < 20; i++ {
		steadyRise = append(steadyRise, float64(100+i))
	}

	if got := rsiScore(steadyRise); got != 10 {
		t.Fatalf("expected RSI score to penalize fully overbought series, got %.2f", got)
	}

	balanced := []float64{100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100}
	if got := rsiScore(balanced); got < 85 || got > 95 {
		t.Fatalf("expected balanced RSI score near optimal range, got %.2f", got)
	}
}

func TestMACDScoreEdgeCases(t *testing.T) {
	t.Parallel()

	if got := macdScore([]float64{1, 2, 3}); got != 50 {
		t.Fatalf("expected insufficient MACD history to return neutral 50, got %.2f", got)
	}

	nonPositiveTail := make([]float64, 35)
	for i := 0; i < 34; i++ {
		nonPositiveTail[i] = 100 + float64(i)
	}
	nonPositiveTail[34] = 0

	if got := macdScore(nonPositiveTail); got != 50 {
		t.Fatalf("expected non-positive latest price to return neutral 50, got %.2f", got)
	}

	bullish := make([]float64, 60)
	bearish := make([]float64, 60)
	for i := 0; i < 60; i++ {
		bullish[i] = 100 + float64(i)*1.8
		bearish[i] = 220 - float64(i)*1.8
	}

	bullishScore := macdScore(bullish)
	bearishScore := macdScore(bearish)
	if bullishScore <= bearishScore {
		t.Fatalf("expected bullish MACD score %.2f to exceed bearish score %.2f", bullishScore, bearishScore)
	}
}

func buildCandles(count int, startPrice, priceStep, baseVolume, volumeStep float64) []Candle {
	candles := make([]Candle, 0, count)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < count; i++ {
		open := startPrice + float64(i)*priceStep
		close := open + priceStep*0.6
		candles = append(candles, Candle{
			Open:      open,
			High:      close + 0.4,
			Low:       open - 0.4,
			Close:     close,
			Volume:    baseVolume + float64(i)*volumeStep,
			OpenTime:  start.Add(time.Duration(i) * 5 * time.Minute),
			CloseTime: start.Add(time.Duration(i+1) * 5 * time.Minute),
		})
	}

	return candles
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("unexpected value: got %.12f want %.12f", got, want)
	}
}
