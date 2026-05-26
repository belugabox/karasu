package strategy

import (
	"testing"
	"time"

	"karasu/internal/scoring"
)

func TestEvaluateReturnsAvoidOnInsufficientHistory(t *testing.T) {
	t.Parallel()

	evaluation := Evaluate(buildCandles(20, 100, 0.8, 1000, 20), IntradayMomentumProfile())

	if evaluation.State != StateAvoid {
		t.Fatalf("expected avoid for insufficient history, got %s", evaluation.State)
	}
	if len(evaluation.Risks) == 0 || evaluation.Risks[0] != "insufficient history" {
		t.Fatalf("expected insufficient history risk, got %#v", evaluation.Risks)
	}
}

func TestEvaluateIntradayMomentumReturnsEntryOnStrongSeries(t *testing.T) {
	t.Parallel()

	evaluation := Evaluate(buildCandles(90, 100, 2.4, 1000, 80), IntradayMomentumProfile())

	if evaluation.State != StateEntry {
		t.Fatalf("expected entry state, got %s with score %.2f and breakdown %#v", evaluation.State, evaluation.Score, evaluation.Breakdown)
	}
	if len(evaluation.Reasons) == 0 {
		t.Fatal("expected reasons for intraday entry")
	}
	foundRecentThrust := false
	for _, reason := range evaluation.Reasons {
		if reason == "recent price thrust matches profile" {
			foundRecentThrust = true
			break
		}
	}
	if !foundRecentThrust {
		t.Fatalf("expected intraday entry to mention recent thrust, got %#v", evaluation.Reasons)
	}
}

func TestEvaluateSwingReturnsWatchForMixedSeries(t *testing.T) {
	t.Parallel()

	evaluation := Evaluate(buildMixedCandles(), SwingProfile())

	if evaluation.State != StateWatch {
		t.Fatalf("expected watch state, got %s with score %.2f and breakdown %#v", evaluation.State, evaluation.Score, evaluation.Breakdown)
	}
}

func TestEvaluateTrendReturnsExitOnWeakSeries(t *testing.T) {
	t.Parallel()

	evaluation := Evaluate(buildCandles(90, 240, -1.8, 1500, -5), TrendProfile())

	if evaluation.State != StateExit {
		t.Fatalf("expected exit state, got %s with score %.2f and breakdown %#v", evaluation.State, evaluation.Score, evaluation.Breakdown)
	}
	if len(evaluation.Risks) == 0 {
		t.Fatal("expected risks for weak trend evaluation")
	}
}

func TestEvaluateDefaultsReturnsThreeProfiles(t *testing.T) {
	t.Parallel()

	results := EvaluateDefaults(buildCandles(90, 100, 1.4, 1000, 20))
	if len(results) != 3 {
		t.Fatalf("expected 3 default strategy evaluations, got %d", len(results))
	}
	if results[0].Profile.Name == results[1].Profile.Name || results[1].Profile.Name == results[2].Profile.Name {
		t.Fatalf("expected distinct strategy profiles, got %#v", results)
	}
}

func TestEvaluateHistoryReturnsRollingPoints(t *testing.T) {
	t.Parallel()

	candles := buildCandles(40, 100, 1.2, 1000, 30)
	points := EvaluateHistory(candles, SwingProfile(), 0)

	if len(points) != 11 {
		t.Fatalf("expected 11 rolling points, got %d", len(points))
	}
	if !points[0].OpenTime.Equal(candles[29].OpenTime) {
		t.Fatalf("expected first point to start at candle 29, got %v want %v", points[0].OpenTime, candles[29].OpenTime)
	}
	if !points[len(points)-1].CloseTime.Equal(candles[len(candles)-1].CloseTime) {
		t.Fatalf("expected last point to match last candle close time")
	}
}

func TestEvaluateHistoryRespectsLimit(t *testing.T) {
	t.Parallel()

	candles := buildCandles(50, 100, 1.1, 1000, 15)
	points := EvaluateHistory(candles, TrendProfile(), 5)

	if len(points) != 5 {
		t.Fatalf("expected limited history of 5 points, got %d", len(points))
	}
	if !points[0].OpenTime.Equal(candles[len(candles)-5].OpenTime) {
		t.Fatalf("expected first limited point to align with last 5 candles window")
	}
}

func TestEvaluateHistoryDefaultsReturnsThreeProfiles(t *testing.T) {
	t.Parallel()

	history := EvaluateHistoryDefaults(buildCandles(60, 100, 1.3, 1000, 20), 3)
	if len(history) != 3 {
		t.Fatalf("expected 3 profile histories, got %d", len(history))
	}
	for _, profileHistory := range history {
		if len(profileHistory.Points) != 3 {
			t.Fatalf("expected 3 points for %s, got %d", profileHistory.Profile.Name, len(profileHistory.Points))
		}
	}
}

func buildCandles(count int, startPrice, priceStep, baseVolume, volumeStep float64) []scoring.Candle {
	candles := make([]scoring.Candle, 0, count)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	volume := baseVolume
	for i := 0; i < count; i++ {
		open := startPrice + float64(i)*priceStep
		close := open + priceStep*0.5
		if volume < 1 {
			volume = 1
		}
		candles = append(candles, scoring.Candle{
			Open:      open,
			High:      max(open, close) + 0.6,
			Low:       min(open, close) - 0.6,
			Close:     close,
			Volume:    volume,
			OpenTime:  start.Add(time.Duration(i) * time.Minute),
			CloseTime: start.Add(time.Duration(i+1) * time.Minute),
		})
		volume += volumeStep
	}

	return candles
}

func buildMixedCandles() []scoring.Candle {
	candles := make([]scoring.Candle, 0, 90)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	price := 100.0
	volume := 1000.0
	for i := 0; i < 90; i++ {
		step := 0.7
		if i%4 == 0 {
			step = -0.5
		}
		open := price
		close := price + step
		candles = append(candles, scoring.Candle{
			Open:      open,
			High:      max(open, close) + 0.4,
			Low:       min(open, close) - 0.4,
			Close:     close,
			Volume:    volume,
			OpenTime:  start.Add(time.Duration(i) * 15 * time.Minute),
			CloseTime: start.Add(time.Duration(i+1) * 15 * time.Minute),
		})
		price = close
		if i%3 == 0 {
			volume += 10
		}
	}
	return candles
}

func max(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func min(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
