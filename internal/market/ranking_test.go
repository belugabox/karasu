package market

import (
	"errors"
	"karasu/internal/exchange"
	"karasu/internal/store"
	"strings"
	"testing"
	"time"
)

func TestTopMarketPositionsIncludesQualityBreakdown(t *testing.T) {
	t.Parallel()

	oneDayBundle := exchange.CandleBundle{
		Symbol:   "BTC",
		Interval: exchange.Interval1d,
		Candles: []exchange.Candle{{
			Open:      100,
			High:      112,
			Low:       99,
			Close:     110,
			Volume:    1000,
			OpenTime:  time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
			CloseTime: time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC),
		}},
	}

	oneMinuteBundle := exchange.CandleBundle{
		Symbol:   "BTC",
		Interval: exchange.Interval1m,
		Candles:  buildExchangeCandles(90, 200, 0.9, 1200, 8),
	}

	client := fakeExchangeClient{
		wallet:         exchange.Wallet{},
		candlesLast24h: []exchange.CandleBundle{oneDayBundle},
		candles1m: map[string]exchange.CandleBundle{
			"BTC": oneMinuteBundle,
		},
	}

	markets, err := TopMarketPositions(client)
	if err != nil {
		t.Fatalf("TopMarketPositions returned error: %v", err)
	}

	if len(markets) != 1 {
		t.Fatalf("expected exactly one market, got %d", len(markets))
	}

	got := markets[0]
	if got.Symbol != "BTC" {
		t.Fatalf("expected BTC market, got %q", got.Symbol)
	}

	expectedQuality := computeQualityBreakdownFromExchangeCandles(oneMinuteBundle.Candles)
	assertClose(t, got.QualityScore, roundTo(expectedQuality.Score, 2))
	assertClose(t, got.QualityRSI, roundTo(expectedQuality.RSI, 2))
	assertClose(t, got.QualityMACD, roundTo(expectedQuality.MACD, 2))
	assertClose(t, got.QualityBollinger, roundTo(expectedQuality.Bollinger, 2))
	assertClose(t, got.QualityVolume, roundTo(expectedQuality.Volume, 2))
	assertClose(t, got.QualitySMA, roundTo(expectedQuality.SMA, 2))

	expectedChange1h, ok := computeWindowChange(oneMinuteBundle.Candles, time.Hour)
	if !ok {
		t.Fatal("expected 1h change to be computable")
	}
	expectedChange5m, ok := computeWindowChange(oneMinuteBundle.Candles, 5*time.Minute)
	if !ok {
		t.Fatal("expected 5m change to be computable")
	}

	assertClose(t, got.Change24h, 10)
	assertClose(t, got.Change1h, roundTo(expectedChange1h, 2))
	assertClose(t, got.Change5m, roundTo(expectedChange5m, 2))
	expectedStrategies := computeStrategyEvaluationsFromExchangeCandles(oneMinuteBundle.Candles)
	if len(got.Strategies) != len(expectedStrategies) {
		t.Fatalf("expected %d strategy evaluations, got %d", len(expectedStrategies), len(got.Strategies))
	}
	for i := range expectedStrategies {
		if got.Strategies[i].Name != expectedStrategies[i].Name {
			t.Fatalf("expected strategy %d name %q, got %q", i, expectedStrategies[i].Name, got.Strategies[i].Name)
		}
		if got.Strategies[i].Label != expectedStrategies[i].Label {
			t.Fatalf("expected strategy %d label %q, got %q", i, expectedStrategies[i].Label, got.Strategies[i].Label)
		}
		if got.Strategies[i].State != expectedStrategies[i].State {
			t.Fatalf("expected strategy %d state %q, got %q", i, expectedStrategies[i].State, got.Strategies[i].State)
		}
		assertClose(t, got.Strategies[i].Score, expectedStrategies[i].Score)
	}
	if got.Change1hPosition != 1 || got.Change5mPosition != 1 || got.QuoteVolumePosition != 1 || got.Change24hPosition != 1 {
		t.Fatalf("expected single market to rank first on all axes, got %#v", got)
	}
}

func TestAnalyzeSymbolReturnsDetailedEvaluation(t *testing.T) {
	t.Parallel()

	oneDayBundle := exchange.CandleBundle{
		Symbol:   "BTC",
		Interval: exchange.Interval1d,
		Candles: []exchange.Candle{{
			Open:      100,
			High:      112,
			Low:       99,
			Close:     110,
			Volume:    1000,
			OpenTime:  time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
			CloseTime: time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC),
		}},
	}

	oneMinuteBundle := exchange.CandleBundle{
		Symbol:   "BTC",
		Interval: exchange.Interval1m,
		Candles:  buildExchangeCandles(90, 200, 0.9, 1200, 8),
	}

	client := fakeExchangeClient{
		candlesLast24h: []exchange.CandleBundle{oneDayBundle},
		candles1m: map[string]exchange.CandleBundle{
			"BTC": oneMinuteBundle,
		},
	}

	analysis, err := AnalyzeSymbol(client, "btc")
	if err != nil {
		t.Fatalf("AnalyzeSymbol returned error: %v", err)
	}

	if analysis.Symbol != "BTC" {
		t.Fatalf("expected BTC analysis, got %q", analysis.Symbol)
	}
	assertClose(t, analysis.QuoteVolume, 110000)
	assertClose(t, analysis.Change24h, 10)
	if analysis.CandleCount1m != len(oneMinuteBundle.Candles) {
		t.Fatalf("expected candle count %d, got %d", len(oneMinuteBundle.Candles), analysis.CandleCount1m)
	}

	expectedQuality := computeQualityBreakdownFromExchangeCandles(oneMinuteBundle.Candles)
	assertClose(t, analysis.Quality.Score, expectedQuality.Score)
	if len(analysis.Strategies) != 3 {
		t.Fatalf("expected 3 strategy evaluations, got %d", len(analysis.Strategies))
	}
}

func TestAnalyzeSymbolPropagatesNotFound(t *testing.T) {
	t.Parallel()

	client := fakeExchangeClient{
		candlesLast24h: []exchange.CandleBundle{},
		candles1m: map[string]exchange.CandleBundle{
			"BTC": {Symbol: "BTC", Interval: exchange.Interval1m, Candles: buildExchangeCandles(90, 200, 0.9, 1200, 8)},
		},
	}

	_, err := AnalyzeSymbol(client, "BTC")
	if err == nil {
		t.Fatal("expected symbol not found error")
	}
	if !strings.Contains(err.Error(), "symbol BTC not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildSignalHistoryReturnsProfiles(t *testing.T) {
	t.Parallel()

	candles := buildExchangeCandles(70, 100, 1.2, 1200, 10)
	history := BuildSignalHistory("btc", "5m", candles, 4)

	if history.Symbol != "BTC" {
		t.Fatalf("expected BTC symbol, got %q", history.Symbol)
	}
	if history.Timeframe != "5m" {
		t.Fatalf("expected 5m timeframe, got %q", history.Timeframe)
	}
	if history.CandleCount != len(candles) {
		t.Fatalf("expected candle count %d, got %d", len(candles), history.CandleCount)
	}
	if len(history.Profiles) != 3 {
		t.Fatalf("expected 3 signal profile histories, got %d", len(history.Profiles))
	}
	for _, profile := range history.Profiles {
		if len(profile.Points) != 4 {
			t.Fatalf("expected 4 points for %s, got %d", profile.Name, len(profile.Points))
		}
	}
}

func TestComputeSignalProfileStats(t *testing.T) {
	t.Parallel()

	points := []SignalPoint{
		{State: "watch", Score: 40},
		{State: "entry", Score: 62},
		{State: "hold", Score: 65},
		{State: "hold", Score: 67},
		{State: "exit", Score: 45},
		{State: "watch", Score: 52},
		{State: "entry", Score: 63},
		{State: "hold", Score: 61},
		{State: "avoid", Score: 35},
	}

	stats := computeSignalProfileStats(points)

	if stats.PointCount != 9 {
		t.Fatalf("expected 9 points, got %d", stats.PointCount)
	}
	if stats.LatestState != "avoid" {
		t.Fatalf("expected latest state avoid, got %q", stats.LatestState)
	}
	assertClose(t, stats.LatestScore, 35)
	if stats.LatestStateAgeBars != 1 {
		t.Fatalf("expected latest state age 1, got %d", stats.LatestStateAgeBars)
	}
	if stats.BarsSinceEntry != 2 {
		t.Fatalf("expected 2 bars since entry, got %d", stats.BarsSinceEntry)
	}
	if stats.BarsSinceExit != 4 {
		t.Fatalf("expected 4 bars since exit, got %d", stats.BarsSinceExit)
	}
	if !stats.JustChanged {
		t.Fatal("expected latest point to be marked as changed")
	}
	if stats.JustEntered {
		t.Fatal("did not expect just entered on avoid latest state")
	}
	if stats.JustExited {
		t.Fatal("did not expect just exited on avoid latest state")
	}
	assertClose(t, stats.AverageScore, 54.44)
	if stats.EntryCount != 2 || stats.HoldCount != 3 || stats.WatchCount != 2 || stats.ExitCount != 1 || stats.AvoidCount != 1 {
		t.Fatalf("unexpected state counts: %#v", stats)
	}
	if stats.TransitionCount != 8 {
		t.Fatalf("expected 8 transitions, got %d", stats.TransitionCount)
	}
	if stats.EntryTransitionCount != 2 {
		t.Fatalf("expected 2 entry transitions, got %d", stats.EntryTransitionCount)
	}
	assertClose(t, stats.EntryTransitionRate, 25)
	assertClose(t, stats.AverageHoldBars, 1.5)
	if stats.ResolvedTradeCount != 2 {
		t.Fatalf("expected 2 resolved trades, got %d", stats.ResolvedTradeCount)
	}
	if stats.ExitAfterEntryCount != 1 {
		t.Fatalf("expected 1 exit after entry, got %d", stats.ExitAfterEntryCount)
	}
	assertClose(t, stats.ExitAfterEntryRate, 50)
	assertClose(t, stats.StabilityRate, 12.5)
}

func TestComputeSignalProfileStatsFlagsFreshEntryAndAge(t *testing.T) {
	t.Parallel()

	points := []SignalPoint{
		{State: "watch", Score: 40},
		{State: "watch", Score: 42},
		{State: "entry", Score: 64},
	}

	stats := computeSignalProfileStats(points)

	if !stats.JustChanged {
		t.Fatal("expected just changed to be true")
	}
	if !stats.JustEntered {
		t.Fatal("expected just entered to be true")
	}
	if stats.JustExited {
		t.Fatal("did not expect just exited to be true")
	}
	if stats.LatestStateAgeBars != 1 {
		t.Fatalf("expected latest state age 1, got %d", stats.LatestStateAgeBars)
	}
	if stats.BarsSinceEntry != 0 {
		t.Fatalf("expected 0 bars since entry, got %d", stats.BarsSinceEntry)
	}
	if stats.BarsSinceExit != -1 {
		t.Fatalf("expected -1 bars since exit, got %d", stats.BarsSinceExit)
	}
}

func TestBuildSignalHistoryIncludesProfileStats(t *testing.T) {
	t.Parallel()

	candles := buildExchangeCandles(70, 100, 1.2, 1200, 10)
	history := BuildSignalHistory("btc", "5m", candles, 6)

	if len(history.Profiles) != 3 {
		t.Fatalf("expected 3 signal profile histories, got %d", len(history.Profiles))
	}

	for _, profile := range history.Profiles {
		if profile.Stats.PointCount != len(profile.Points) {
			t.Fatalf("expected point count to match profile points for %s", profile.Name)
		}
		if profile.Stats.LatestState == "" || profile.Stats.LatestState == "n/a" {
			t.Fatalf("expected latest state for %s, got %q", profile.Name, profile.Stats.LatestState)
		}
		if profile.Stats.TransitionCount < 0 {
			t.Fatalf("expected non-negative transition count for %s", profile.Name)
		}
	}
}

func TestBuildOpportunityReturnsActionableFreshConsensus(t *testing.T) {
	t.Parallel()

	marketData := Market{
		Symbol:       "BTC",
		QuoteVolume:  150000,
		QualityScore: 78,
		Change24h:    9,
		Change1h:     3.2,
		Change5m:     1.1,
		Strategies: []StrategyEvaluation{
			{Name: "intraday-momentum", Label: "Intraday Momentum", State: "entry", Score: 83, Reasons: []string{"momentum confirmed"}},
			{Name: "swing-balance", Label: "Swing Balance", State: "hold", Score: 74},
			{Name: "trend-follow", Label: "Trend Follow", State: "watch", Score: 68},
		},
	}

	history := SignalHistory{
		Symbol:    "BTC",
		Timeframe: "5m",
		Profiles: []SignalProfileHistory{
			{Name: "intraday-momentum", Label: "Intraday Momentum", Stats: SignalProfileStats{JustEntered: true, JustChanged: true, LatestStateAgeBars: 1, BarsSinceEntry: 0}},
			{Name: "swing-balance", Label: "Swing Balance", Stats: SignalProfileStats{LatestStateAgeBars: 2, BarsSinceEntry: 1}},
			{Name: "trend-follow", Label: "Trend Follow", Stats: SignalProfileStats{LatestStateAgeBars: 3, BarsSinceEntry: 4}},
		},
	}

	opp := buildOpportunity(marketData, history)

	if opp.Symbol != "BTC" {
		t.Fatalf("expected BTC opportunity, got %q", opp.Symbol)
	}
	if opp.PriorityBand != "actionable" {
		t.Fatalf("expected actionable band, got %q", opp.PriorityBand)
	}
	if opp.PrimaryAction != "act-now" {
		t.Fatalf("expected act-now action, got %q", opp.PrimaryAction)
	}
	if !opp.Convergence.Consensus {
		t.Fatal("expected convergence consensus")
	}
	if !opp.Freshness.HasFreshEntry {
		t.Fatal("expected fresh entry flag")
	}
	if opp.Freshness.YoungestEntryBars != 0 {
		t.Fatalf("expected youngest entry age 0, got %d", opp.Freshness.YoungestEntryBars)
	}
	if opp.PriorityScore < 80 {
		t.Fatalf("expected high priority score, got %.2f", opp.PriorityScore)
	}
	if !strings.Contains(strings.ToLower(opp.Summary), "fresh") {
		t.Fatalf("expected summary to mention freshness, got %q", opp.Summary)
	}
}

func TestTopOpportunitiesReturnsSortedMarkets(t *testing.T) {
	t.Parallel()

	client := fakeExchangeClient{
		wallet: exchange.Wallet{},
		candlesLast24h: []exchange.CandleBundle{
			{
				Symbol:   "BTC",
				Interval: exchange.Interval1d,
				Candles:  []exchange.Candle{{Open: 100, High: 120, Low: 95, Close: 115, Volume: 1000, OpenTime: time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC), CloseTime: time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC)}},
			},
			{
				Symbol:   "ETH",
				Interval: exchange.Interval1d,
				Candles:  []exchange.Candle{{Open: 100, High: 101, Low: 80, Close: 84, Volume: 1000, OpenTime: time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC), CloseTime: time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC)}},
			},
		},
		candles1m: map[string]exchange.CandleBundle{
			"BTC": {Symbol: "BTC", Interval: exchange.Interval1m, Candles: buildExchangeCandles(90, 200, 1.8, 1200, 10)},
			"ETH": {Symbol: "ETH", Interval: exchange.Interval1m, Candles: buildExchangeCandles(90, 200, -1.4, 1200, -3)},
		},
	}

	freshBTC := append(buildExchangeCandles(55, 100, 0.7, 1000, 10), buildExchangeCandles(15, 150, 2.4, 1100, 20)...)
	storeStub := fakeCandleStore{
		candles: map[string][]exchange.Candle{
			"BTC": freshBTC,
			"ETH": buildExchangeCandles(70, 200, -1.2, 1000, -5),
		},
	}

	opps, err := TopOpportunities(client, storeStub, 2)
	if err != nil {
		t.Fatalf("TopOpportunities returned error: %v", err)
	}
	if len(opps) != 2 {
		t.Fatalf("expected 2 opportunities, got %d", len(opps))
	}
	if opps[0].Symbol != "BTC" {
		t.Fatalf("expected BTC to rank first, got %q", opps[0].Symbol)
	}
	if opps[0].PriorityScore <= opps[1].PriorityScore {
		t.Fatalf("expected descending priority scores, got %.2f <= %.2f", opps[0].PriorityScore, opps[1].PriorityScore)
	}
	if opps[0].PrimaryAction == "avoid" {
		t.Fatalf("expected BTC opportunity not to be avoid, got %#v", opps[0])
	}
}

func TestTopMarketPositionsPropagatesWalletError(t *testing.T) {
	t.Parallel()

	client := fakeExchangeClient{
		walletErr: errors.New("wallet unavailable"),
	}

	_, err := TopMarketPositions(client)
	if err == nil {
		t.Fatal("expected wallet error")
	}
	if !strings.Contains(err.Error(), "failed to load wallet priority symbols") || !strings.Contains(err.Error(), "wallet unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTopMarketPositionsPropagatesLast24hError(t *testing.T) {
	t.Parallel()

	client := fakeExchangeClient{
		candlesLast24hErr: errors.New("candles unavailable"),
	}

	_, err := TopMarketPositions(client)
	if err == nil {
		t.Fatal("expected last24h error")
	}
	if !strings.Contains(err.Error(), "failed to get 24h candles") || !strings.Contains(err.Error(), "candles unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTopMarketPositionsPropagates1mError(t *testing.T) {
	t.Parallel()

	client := fakeExchangeClient{
		wallet: exchange.Wallet{},
		candlesLast24h: []exchange.CandleBundle{{
			Symbol:   "BTC",
			Interval: exchange.Interval1d,
			Candles: []exchange.Candle{{
				Open:      100,
				High:      112,
				Low:       99,
				Close:     110,
				Volume:    1000,
				OpenTime:  time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
				CloseTime: time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC),
			}},
		}},
		candles1mErr: map[string]error{
			"BTC": errors.New("1m unavailable"),
		},
	}

	_, err := TopMarketPositions(client)
	if err == nil {
		t.Fatal("expected 1m error")
	}
	if !strings.Contains(err.Error(), "failed to get 1m candles for BTC") || !strings.Contains(err.Error(), "1m unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func buildExchangeCandles(count int, startPrice, priceStep, baseVolume, volumeStep float64) []exchange.Candle {
	candles := make([]exchange.Candle, 0, count)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < count; i++ {
		open := startPrice + float64(i)*priceStep
		close := open + priceStep*0.6
		openTime := start.Add(time.Duration(i) * time.Minute)
		candles = append(candles, exchange.Candle{
			Open:      open,
			High:      close + 0.4,
			Low:       open - 0.4,
			Close:     close,
			Volume:    baseVolume + float64(i)*volumeStep,
			OpenTime:  openTime,
			CloseTime: openTime.Add(time.Minute),
		})
	}

	return candles
}

type fakeExchangeClient struct {
	wallet            exchange.Wallet
	walletErr         error
	candlesLast24h    []exchange.CandleBundle
	candlesLast24hErr error
	candles1m         map[string]exchange.CandleBundle
	candles1mErr      map[string]error
}

type fakeCandleStore struct {
	candles map[string][]exchange.Candle
	err     error
}

func (f fakeCandleStore) UpsertCandles(exchangeName, symbol, timeframe string, candles []exchange.Candle) error {
	return nil
}

func (f fakeCandleStore) QueryCandles(exchangeName, symbol, timeframe string, limit int) ([]exchange.Candle, error) {
	if f.err != nil {
		return nil, f.err
	}
	result := append([]exchange.Candle(nil), f.candles[symbol]...)
	if limit > 0 && len(result) > limit {
		return result[len(result)-limit:], nil
	}
	return result, nil
}

func (f fakeCandleStore) LastCandleOpenTime(exchangeName, symbol, timeframe string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (f fakeCandleStore) QueryDailySymbolActivity(exchangeName, timeframe string, days int) ([]store.DailySymbolActivity, error) {
	return nil, nil
}

func (f fakeCandleStore) Close() error {
	return nil
}

func (f fakeExchangeClient) Symbols() (map[string]string, error) {
	return map[string]string{}, nil
}

func (f fakeExchangeClient) Prices() (map[string]float64, error) {
	return map[string]float64{}, nil
}

func (f fakeExchangeClient) Wallet() (exchange.Wallet, error) {
	if f.walletErr != nil {
		return exchange.Wallet{}, f.walletErr
	}
	return f.wallet, nil
}

func (f fakeExchangeClient) Candles1mByDate(symbol string, date time.Time) (exchange.CandleBundle, error) {
	if err := f.candles1mErr[symbol]; err != nil {
		return exchange.CandleBundle{}, err
	}
	return f.candles1m[symbol], nil
}

func (f fakeExchangeClient) Candles5mByDate(symbol string, date time.Time) (exchange.CandleBundle, error) {
	return exchange.CandleBundle{}, nil
}

func (f fakeExchangeClient) CandlesLast24h() ([]exchange.CandleBundle, error) {
	if f.candlesLast24hErr != nil {
		return nil, f.candlesLast24hErr
	}
	return f.candlesLast24h, nil
}

func (f fakeExchangeClient) Candles(symbol string, start time.Time, end time.Time, interval exchange.Interval) (exchange.CandleBundle, error) {
	return exchange.CandleBundle{}, nil
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()

	if got != want {
		t.Fatalf("unexpected value: got %.12f want %.12f", got, want)
	}
}
