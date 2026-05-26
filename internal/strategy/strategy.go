package strategy

import (
	"strings"
	"time"

	"karasu/internal/scoring"
)

type State string

const (
	StateAvoid State = "avoid"
	StateWatch State = "watch"
	StateEntry State = "entry"
	StateHold  State = "hold"
	StateExit  State = "exit"
)

type Profile struct {
	Name           string
	Label          string
	Timeframe      string
	RecentLookback int
	MinRecentMove  float64
	WatchThreshold float64
	EntryThreshold float64
	HoldThreshold  float64
	ExitThreshold  float64
	MinMACD        float64
	MinVolume      float64
	MinSMA         float64
	MinBollinger   float64
}

type Evaluation struct {
	Profile   Profile
	State     State
	Score     float64
	Breakdown scoring.QualityBreakdown
	Reasons   []string
	Risks     []string
}

type HistoryPoint struct {
	OpenTime  time.Time
	CloseTime time.Time
	State     State
	Score     float64
}

type ProfileHistory struct {
	Profile Profile
	Points  []HistoryPoint
}

func IntradayMomentumProfile() Profile {
	return Profile{
		Name:           "intraday-momentum",
		Label:          "Intraday Momentum",
		Timeframe:      "short",
		RecentLookback: 5,
		MinRecentMove:  1.2,
		WatchThreshold: 50,
		EntryThreshold: 62,
		HoldThreshold:  55,
		ExitThreshold:  42,
		MinMACD:        55,
		MinVolume:      55,
		MinBollinger:   25,
	}
}

func SwingProfile() Profile {
	return Profile{
		Name:           "swing-balance",
		Label:          "Swing Balance",
		Timeframe:      "medium",
		RecentLookback: 8,
		MinRecentMove:  0.6,
		WatchThreshold: 48,
		EntryThreshold: 60,
		HoldThreshold:  54,
		ExitThreshold:  44,
		MinMACD:        50,
		MinVolume:      45,
		MinSMA:         52,
		MinBollinger:   35,
	}
}

func TrendProfile() Profile {
	return Profile{
		Name:           "trend-follow",
		Label:          "Trend Follow",
		Timeframe:      "long",
		RecentLookback: 12,
		MinRecentMove:  0.4,
		WatchThreshold: 50,
		EntryThreshold: 62,
		HoldThreshold:  57,
		ExitThreshold:  47,
		MinMACD:        48,
		MinVolume:      40,
		MinSMA:         60,
		MinBollinger:   30,
	}
}

func DefaultProfiles() []Profile {
	return []Profile{
		IntradayMomentumProfile(),
		SwingProfile(),
		TrendProfile(),
	}
}

func Evaluate(candles []scoring.Candle, profile Profile) Evaluation {
	breakdown := scoring.ComputeQualityBreakdown(candles, profile.Timeframe)
	evaluation := Evaluation{
		Profile:   profile,
		Score:     breakdown.Score,
		Breakdown: breakdown,
		Reasons:   []string{},
		Risks:     []string{},
	}

	if len(candles) < 30 {
		evaluation.State = StateAvoid
		evaluation.Risks = append(evaluation.Risks, "insufficient history")
		return evaluation
	}

	recentMove, recentMoveOK := recentChangePercent(candles, profile.RecentLookback)
	if profile.RecentLookback > 0 {
		if recentMoveOK && recentMove >= profile.MinRecentMove {
			evaluation.Reasons = append(evaluation.Reasons, "recent price thrust matches profile")
		} else {
			evaluation.Risks = append(evaluation.Risks, "recent price thrust is insufficient")
		}
	}

	if breakdown.MACD >= profile.MinMACD {
		evaluation.Reasons = append(evaluation.Reasons, "macd momentum confirmed")
	} else {
		evaluation.Risks = append(evaluation.Risks, "macd momentum too weak")
	}

	if breakdown.Volume >= profile.MinVolume {
		evaluation.Reasons = append(evaluation.Reasons, "volume participation supportive")
	} else {
		evaluation.Risks = append(evaluation.Risks, "volume confirmation missing")
	}

	if profile.MinSMA > 0 {
		if breakdown.SMA >= profile.MinSMA {
			evaluation.Reasons = append(evaluation.Reasons, "trend structure aligned")
		} else {
			evaluation.Risks = append(evaluation.Risks, "trend structure below profile floor")
		}
	}

	if breakdown.Bollinger >= profile.MinBollinger {
		evaluation.Reasons = append(evaluation.Reasons, "price location remains tradable")
	} else {
		evaluation.Risks = append(evaluation.Risks, "price location too stretched or weak")
	}

	if breakdown.RSI < 35 {
		evaluation.Risks = append(evaluation.Risks, "rsi balance is weak")
	} else if breakdown.RSI >= 55 {
		evaluation.Reasons = append(evaluation.Reasons, "rsi balance is constructive")
	}

	standardEntry := breakdown.Score >= profile.EntryThreshold &&
		breakdown.MACD >= profile.MinMACD &&
		breakdown.Volume >= profile.MinVolume &&
		breakdown.Bollinger >= profile.MinBollinger &&
		(profile.MinSMA == 0 || breakdown.SMA >= profile.MinSMA) &&
		(profile.RecentLookback == 0 || (recentMoveOK && recentMove >= profile.MinRecentMove))

	momentumOverride := profile.Name == "intraday-momentum" &&
		recentMoveOK && recentMove >= profile.MinRecentMove*2 &&
		breakdown.MACD >= profile.MinMACD-5 &&
		breakdown.Volume >= profile.MinVolume &&
		breakdown.Bollinger >= profile.MinBollinger

	entryReady := standardEntry || momentumOverride

	holdReady := breakdown.Score >= profile.HoldThreshold && breakdown.MACD >= profile.MinMACD-5
	exitTriggered := breakdown.Score < profile.ExitThreshold || breakdown.MACD < profile.MinMACD-10 || (recentMoveOK && recentMove < -profile.MinRecentMove)

	switch {
	case entryReady:
		evaluation.State = StateEntry
	case exitTriggered:
		evaluation.State = StateExit
	case holdReady:
		evaluation.State = StateHold
	case breakdown.Score >= profile.WatchThreshold:
		evaluation.State = StateWatch
	default:
		evaluation.State = StateAvoid
	}

	evaluation.Reasons = uniqueNonEmpty(evaluation.Reasons)
	evaluation.Risks = uniqueNonEmpty(evaluation.Risks)
	return evaluation
}

func recentChangePercent(candles []scoring.Candle, lookback int) (float64, bool) {
	if lookback <= 0 || len(candles) <= lookback {
		return 0, false
	}
	ref := candles[len(candles)-1-lookback].Close
	last := candles[len(candles)-1].Close
	if ref <= 0 || last <= 0 {
		return 0, false
	}
	return ((last - ref) / ref) * 100, true
}

func EvaluateDefaults(candles []scoring.Candle) []Evaluation {
	profiles := DefaultProfiles()
	results := make([]Evaluation, 0, len(profiles))
	for _, profile := range profiles {
		results = append(results, Evaluate(candles, profile))
	}
	return results
}

func EvaluateHistory(candles []scoring.Candle, profile Profile, limit int) []HistoryPoint {
	if len(candles) < 30 {
		return nil
	}

	points := make([]HistoryPoint, 0, len(candles)-29)
	for end := 29; end < len(candles); end++ {
		evaluation := Evaluate(candles[:end+1], profile)
		points = append(points, HistoryPoint{
			OpenTime:  candles[end].OpenTime,
			CloseTime: candles[end].CloseTime,
			State:     evaluation.State,
			Score:     evaluation.Score,
		})
	}

	if limit > 0 && len(points) > limit {
		return append([]HistoryPoint(nil), points[len(points)-limit:]...)
	}

	return points
}

func EvaluateHistoryDefaults(candles []scoring.Candle, limit int) []ProfileHistory {
	profiles := DefaultProfiles()
	results := make([]ProfileHistory, 0, len(profiles))
	for _, profile := range profiles {
		results = append(results, ProfileHistory{
			Profile: profile,
			Points:  EvaluateHistory(candles, profile, limit),
		})
	}
	return results
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
