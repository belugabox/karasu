package market

import (
	"fmt"
	"karasu/internal/exchange"
	"karasu/internal/scoring"
	"karasu/internal/store"
	"karasu/internal/strategy"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Market holds ranking data for a single tradable symbol.
type Market struct {
	Symbol     string
	Strategies []StrategyEvaluation

	QuoteVolume         float64 // volume in quote currency (e.g., EUR)
	QuoteVolumePosition int     // position in the ranking by quote volume
	QualityScore        float64 // technical momentum/quality score from 0 to 100
	QualityRSI          float64 // RSI contribution input score from 0 to 100
	QualityMACD         float64 // MACD contribution input score from 0 to 100
	QualityBollinger    float64 // Bollinger contribution input score from 0 to 100
	QualityVolume       float64 // Volume contribution input score from 0 to 100
	QualitySMA          float64 // SMA contribution input score from 0 to 100

	Change24h         float64 // percentage change over 24h
	Change24hPosition int     // position in the ranking by 24h change

	Change1h         float64 // percentage change over 1h
	Change1hPosition int     // position in the ranking by 1h change

	Change5m         float64 // percentage change over 5m
	Change5mPosition int     // position in the ranking by 5m change
}

type StrategyEvaluation struct {
	Name        string
	Label       string
	Description string
	Icon        string
	Color       string
	State       string
	Score       float64
	Reasons     []string
	Risks       []string
}

type MarketAnalysis struct {
	Symbol        string
	QuoteVolume   float64
	Change24h     float64
	Change1h      float64
	Change5m      float64
	Quality       scoring.QualityBreakdown
	Strategies    []StrategyEvaluation
	CandleCount1m int
}

type SignalHistory struct {
	Symbol      string
	Timeframe   string
	CandleCount int
	Profiles    []SignalProfileHistory
}

type SignalProfileHistory struct {
	Name        string
	Label       string
	Description string
	Icon        string
	Color       string
	Stats       SignalProfileStats
	Points      []SignalPoint
}

type SignalProfileStats struct {
	PointCount           int
	LatestState          string
	LatestScore          float64
	LatestStateAgeBars   int
	AverageScore         float64
	EntryCount           int
	HoldCount            int
	WatchCount           int
	ExitCount            int
	AvoidCount           int
	BarsSinceEntry       int
	BarsSinceExit        int
	JustChanged          bool
	JustEntered          bool
	JustExited           bool
	TransitionCount      int
	EntryTransitionCount int
	EntryTransitionRate  float64
	AverageHoldBars      float64
	ResolvedTradeCount   int
	ExitAfterEntryCount  int
	ExitAfterEntryRate   float64
	StabilityRate        float64
}

type SignalPoint struct {
	OpenTime  time.Time
	CloseTime time.Time
	State     string
	Score     float64
}

type Opportunity struct {
	Symbol        string
	PriorityScore float64
	PriorityBand  string
	PrimaryAction string
	Summary       string
	Leader        StrategyEvaluation
	Convergence   OpportunityConvergence
	Freshness     OpportunityFreshness
	QuoteVolume   float64
	QualityScore  float64
	Change24h     float64
	Change1h      float64
	Change5m      float64
	Reasons       []string
	Risks         []string
}

type OpportunityConvergence struct {
	ActiveProfiles        int
	ConstructiveProfiles  int
	Consensus             bool
	ConstructiveAlignment bool
}

type OpportunityFreshness struct {
	ChangedProfiles    int
	HasFreshEntry      bool
	HasFreshExit       bool
	FreshEntryProfiles []string
	FreshExitProfiles  []string
	YoungestEntryBars  int
	YoungestExitBars   int
	YoungestStateBars  int
}

// FindTopSymbols returns the top symbols by combined liquidity+momentum score.
func FindTopSymbols(exchangeClient exchange.ExchangeClient) ([]string, error) {
	markets := make([]Market, 0)
	var (
		prioritySymbols map[string]struct{}
		candleBundles   []exchange.CandleBundle
		errWallet       error
		errCandles      error
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		prioritySymbols, errWallet = walletPrioritySymbols(exchangeClient)
	}()
	go func() {
		defer wg.Done()
		candleBundles, errCandles = exchangeClient.CandlesLast24h()
	}()
	wg.Wait()

	if errWallet != nil {
		return nil, fmt.Errorf("failed to load wallet priority symbols: %w", errWallet)
	}
	if errCandles != nil {
		return nil, fmt.Errorf("failed to get 24h candles: %w", errCandles)
	}

	for _, candleBundle := range candleBundles {
		candle := candleBundle.Candles[0]

		lastPrice := candle.Close
		openPrice := candle.Open
		volume := candle.Volume

		quoteVolume := roundTo(lastPrice*volume, 0)
		_, forceInclude := prioritySymbols[strings.ToUpper(candleBundle.Symbol)]
		if quoteVolume < 50000 && !forceInclude {
			continue
		}

		change24h := ((lastPrice - openPrice) / openPrice) * 100

		markets = append(markets, Market{
			Symbol:              candleBundle.Symbol,
			QuoteVolume:         quoteVolume,
			Change24h:           roundTo(change24h, 2),
			QuoteVolumePosition: -1,
			Change24hPosition:   -1,
		})
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].QuoteVolume > markets[j].QuoteVolume
	})
	for i := range markets {
		markets[i].QuoteVolumePosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change24h > markets[j].Change24h
	})
	for i := range markets {
		markets[i].Change24hPosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return (markets[i].QuoteVolumePosition + markets[i].Change24hPosition) < (markets[j].QuoteVolumePosition + markets[j].Change24hPosition)
	})

	const maxMarkets = 30
	markets = trimMarketsWithPriority(markets, prioritySymbols, maxMarkets)

	symbols := make([]string, len(markets))
	for i, market := range markets {
		symbols[i] = market.Symbol
	}

	return symbols, nil
}

// TopMarketPositions returns the full ranked market list with 1h and 5m changes.
func TopMarketPositions(exchangeClient exchange.ExchangeClient) ([]Market, error) {
	markets := make([]Market, 0)
	var (
		prioritySymbols map[string]struct{}
		candleBundles   []exchange.CandleBundle
		errWallet       error
		errCandles      error
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		prioritySymbols, errWallet = walletPrioritySymbols(exchangeClient)
	}()
	go func() {
		defer wg.Done()
		candleBundles, errCandles = exchangeClient.CandlesLast24h()
	}()
	wg.Wait()

	if errWallet != nil {
		return nil, fmt.Errorf("failed to load wallet priority symbols: %w", errWallet)
	}
	if errCandles != nil {
		return nil, fmt.Errorf("failed to get 24h candles: %w", errCandles)
	}

	for _, candleBundle := range candleBundles {
		candle := candleBundle.Candles[0]

		lastPrice := candle.Close
		openPrice := candle.Open
		volume := candle.Volume

		quoteVolume := roundTo(lastPrice*volume, 0)
		_, forceInclude := prioritySymbols[strings.ToUpper(candleBundle.Symbol)]
		if quoteVolume < 50000 && !forceInclude {
			continue
		}

		change24h := ((lastPrice - openPrice) / openPrice) * 100

		markets = append(markets, Market{
			Symbol:              candleBundle.Symbol,
			QuoteVolume:         quoteVolume,
			Change24h:           roundTo(change24h, 2),
			QuoteVolumePosition: -1,
			Change24hPosition:   -1,
		})
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].QuoteVolume > markets[j].QuoteVolume
	})
	for i := range markets {
		markets[i].QuoteVolumePosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change24h > markets[j].Change24h
	})
	for i := range markets {
		markets[i].Change24hPosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return (markets[i].QuoteVolumePosition + markets[i].Change24hPosition) < (markets[j].QuoteVolumePosition + markets[j].Change24hPosition)
	})

	const maxMarkets = 30
	markets = trimMarketsWithPriority(markets, prioritySymbols, maxMarkets)

	if err := computeShortTermChangesParallel(exchangeClient, markets); err != nil {
		return nil, err
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change1h > markets[j].Change1h
	})
	for i := range markets {
		markets[i].Change1hPosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change5m > markets[j].Change5m
	})
	for i := range markets {
		markets[i].Change5mPosition = i + 1
	}

	sort.SliceStable(markets, func(i, j int) bool {
		return markets[i].Change1hPosition < markets[j].Change1hPosition
	})

	return markets, nil
}

func TopOpportunities(exchangeClient exchange.ExchangeClient, candleStore store.CandleStore, limit int) ([]Opportunity, error) {
	markets, err := TopMarketPositions(exchangeClient)
	if err != nil {
		return nil, err
	}

	opportunities := make([]Opportunity, 0, len(markets))
	for _, rankedMarket := range markets {
		candles, err := candleStore.QueryCandles("bitvavo", rankedMarket.Symbol, "5m", 120)
		if err != nil {
			return nil, fmt.Errorf("failed to load 5m candles for %s: %w", rankedMarket.Symbol, err)
		}

		history := BuildSignalHistory(rankedMarket.Symbol, "5m", candles, 24)
		opportunities = append(opportunities, buildOpportunity(rankedMarket, history))
	}

	sort.SliceStable(opportunities, func(i, j int) bool {
		if opportunities[i].PriorityScore == opportunities[j].PriorityScore {
			return opportunities[i].QualityScore > opportunities[j].QualityScore
		}
		return opportunities[i].PriorityScore > opportunities[j].PriorityScore
	})

	if limit > 0 && len(opportunities) > limit {
		return append([]Opportunity(nil), opportunities[:limit]...), nil
	}

	return opportunities, nil
}

func AnalyzeSymbol(exchangeClient exchange.ExchangeClient, symbol string) (MarketAnalysis, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return MarketAnalysis{}, fmt.Errorf("symbol is required")
	}

	now := time.Now()
	var (
		candles24h []exchange.CandleBundle
		bundle1m   exchange.CandleBundle
		err24h     error
		err1m      error
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		candles24h, err24h = exchangeClient.CandlesLast24h()
	}()
	go func() {
		defer wg.Done()
		bundle1m, err1m = exchangeClient.Candles1mByDate(symbol, now)
	}()
	wg.Wait()

	if err24h != nil {
		return MarketAnalysis{}, fmt.Errorf("failed to get 24h candles: %w", err24h)
	}
	if err1m != nil {
		return MarketAnalysis{}, fmt.Errorf("failed to get 1m candles for %s: %w", symbol, err1m)
	}

	analysis := MarketAnalysis{
		Symbol:        symbol,
		Quality:       computeQualityBreakdownFromExchangeCandles(bundle1m.Candles),
		Strategies:    computeStrategyEvaluationsFromExchangeCandles(bundle1m.Candles),
		CandleCount1m: len(bundle1m.Candles),
	}

	if change1h, ok := computeWindowChange(bundle1m.Candles, time.Hour); ok {
		analysis.Change1h = roundTo(change1h, 2)
	}
	if change5m, ok := computeWindowChange(bundle1m.Candles, 5*time.Minute); ok {
		analysis.Change5m = roundTo(change5m, 2)
	}

	found24h := false
	for _, candleBundle := range candles24h {
		if !strings.EqualFold(strings.TrimSpace(candleBundle.Symbol), symbol) || len(candleBundle.Candles) == 0 {
			continue
		}
		candle := candleBundle.Candles[0]
		if candle.Open <= 0 || candle.Close <= 0 {
			continue
		}
		analysis.QuoteVolume = roundTo(candle.Close*candle.Volume, 0)
		analysis.Change24h = roundTo(((candle.Close-candle.Open)/candle.Open)*100, 2)
		found24h = true
		break
	}
	if !found24h {
		return MarketAnalysis{}, fmt.Errorf("symbol %s not found", symbol)
	}

	return analysis, nil
}

func BuildSignalHistory(symbol string, timeframe string, candles []exchange.Candle, limit int) SignalHistory {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	timeframe = strings.TrimSpace(timeframe)
	scoringCandles := toScoringCandles(candles)
	history := SignalHistory{
		Symbol:      symbol,
		Timeframe:   timeframe,
		CandleCount: len(scoringCandles),
		Profiles:    []SignalProfileHistory{},
	}
	if len(scoringCandles) == 0 {
		return history
	}

	profileHistories := strategy.EvaluateHistoryDefaults(scoringCandles, limit)
	history.Profiles = make([]SignalProfileHistory, 0, len(profileHistories))
	for _, profileHistory := range profileHistories {
		points := make([]SignalPoint, 0, len(profileHistory.Points))
		for _, point := range profileHistory.Points {
			points = append(points, SignalPoint{
				OpenTime:  point.OpenTime,
				CloseTime: point.CloseTime,
				State:     string(point.State),
				Score:     roundTo(point.Score, 2),
			})
		}
		history.Profiles = append(history.Profiles, SignalProfileHistory{
			Name:        profileHistory.Profile.Name,
			Label:       profileHistory.Profile.Label,
			Description: profileHistory.Profile.Description,
			Icon:        profileHistory.Profile.Icon,
			Color:       profileHistory.Profile.Color,
			Stats:       computeSignalProfileStats(points),
			Points:      points,
		})
	}

	return history
}

func roundTo(value float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(value*factor) / factor
}

func walletPrioritySymbols(exchangeClient exchange.ExchangeClient) (map[string]struct{}, error) {
	wallet, err := exchangeClient.Wallet()
	if err != nil {
		return nil, err
	}

	const minWalletValueEUR = 0.009
	priority := make(map[string]struct{})
	for _, asset := range wallet.Assets {
		symbol := strings.ToUpper(strings.TrimSpace(asset.Symbol))
		if symbol == "" || symbol == "EUR" {
			continue
		}
		if asset.Value > minWalletValueEUR {
			priority[symbol] = struct{}{}
		}
	}

	return priority, nil
}

func trimMarketsWithPriority(markets []Market, prioritySymbols map[string]struct{}, maxMarkets int) []Market {
	if len(markets) <= maxMarkets {
		return markets
	}

	selected := make([]Market, 0, maxMarkets)
	added := make(map[string]struct{}, maxMarkets)

	for _, market := range markets {
		if len(selected) >= maxMarkets {
			break
		}
		symbol := strings.ToUpper(strings.TrimSpace(market.Symbol))
		if _, wanted := prioritySymbols[symbol]; !wanted {
			continue
		}
		if _, exists := added[symbol]; exists {
			continue
		}
		selected = append(selected, market)
		added[symbol] = struct{}{}
	}

	for _, market := range markets {
		if len(selected) >= maxMarkets {
			break
		}
		symbol := strings.ToUpper(strings.TrimSpace(market.Symbol))
		if _, exists := added[symbol]; exists {
			continue
		}
		selected = append(selected, market)
		added[symbol] = struct{}{}
	}

	return selected
}

func computeShortTermChangesParallel(exchangeClient exchange.ExchangeClient, markets []Market) error {
	if len(markets) == 0 {
		return nil
	}

	const workerCount = 8
	jobs := make(chan int)
	var wg sync.WaitGroup

	var firstErr error
	var errMu sync.Mutex

	now := time.Now()

	worker := func() {
		defer wg.Done()
		for i := range jobs {
			candleBundle1m, err := exchangeClient.Candles1mByDate(markets[i].Symbol, now)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to get 1m candles for %s: %w", markets[i].Symbol, err)
				}
				errMu.Unlock()
				continue
			}

			candles := candleBundle1m.Candles
			quality := computeQualityBreakdownFromExchangeCandles(candles)
			markets[i].QualityScore = roundTo(quality.Score, 2)
			markets[i].QualityRSI = roundTo(quality.RSI, 2)
			markets[i].QualityMACD = roundTo(quality.MACD, 2)
			markets[i].QualityBollinger = roundTo(quality.Bollinger, 2)
			markets[i].QualityVolume = roundTo(quality.Volume, 2)
			markets[i].QualitySMA = roundTo(quality.SMA, 2)
			markets[i].Strategies = computeStrategyEvaluationsFromExchangeCandles(candles)
			if change1h, ok := computeWindowChange(candles, time.Hour); ok {
				markets[i].Change1h = roundTo(change1h, 2)
			}
			if change5m, ok := computeWindowChange(candles, 5*time.Minute); ok {
				markets[i].Change5m = roundTo(change5m, 2)
			}
		}
	}

	w := workerCount
	if len(markets) < w {
		w = len(markets)
	}
	for i := 0; i < w; i++ {
		wg.Add(1)
		go worker()
	}

	for i := range markets {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	return nil
}

func computeQualityBreakdownFromExchangeCandles(candles []exchange.Candle) scoring.QualityBreakdown {
	scoringCandles := toScoringCandles(candles)
	if len(scoringCandles) == 0 {
		return scoring.QualityBreakdown{}
	}

	return scoring.ComputeQualityBreakdown(scoringCandles, "short")
}

func computeStrategyEvaluationsFromExchangeCandles(candles []exchange.Candle) []StrategyEvaluation {
	scoringCandles := toScoringCandles(candles)
	if len(scoringCandles) == 0 {
		return nil
	}

	evaluations := strategy.EvaluateDefaults(scoringCandles)
	result := make([]StrategyEvaluation, 0, len(evaluations))
	for _, evaluation := range evaluations {
		result = append(result, StrategyEvaluation{
			Name:        evaluation.Profile.Name,
			Label:       evaluation.Profile.Label,
			Description: evaluation.Profile.Description,
			Icon:        evaluation.Profile.Icon,
			Color:       evaluation.Profile.Color,
			State:       string(evaluation.State),
			Score:       roundTo(evaluation.Score, 2),
			Reasons:     append([]string(nil), evaluation.Reasons...),
			Risks:       append([]string(nil), evaluation.Risks...),
		})
	}

	return result
}

func computeSignalProfileStats(points []SignalPoint) SignalProfileStats {
	stats := SignalProfileStats{
		LatestState:    "n/a",
		BarsSinceEntry: -1,
		BarsSinceExit:  -1,
	}
	if len(points) == 0 {
		return stats
	}

	var (
		totalScore        float64
		stableTransitions int
		holdRunLength     int
		holdRunCount      int
		holdRunBarsTotal  float64
		inTrade           bool
		latestEntryIndex  = -1
		latestExitIndex   = -1
	)

	for i, point := range points {
		totalScore += point.Score

		switch point.State {
		case string(strategy.StateEntry):
			stats.EntryCount++
			latestEntryIndex = i
		case string(strategy.StateHold):
			stats.HoldCount++
		case string(strategy.StateWatch):
			stats.WatchCount++
		case string(strategy.StateExit):
			stats.ExitCount++
			latestExitIndex = i
		case string(strategy.StateAvoid):
			stats.AvoidCount++
		}

		if point.State == string(strategy.StateHold) {
			holdRunLength++
		} else if holdRunLength > 0 {
			holdRunCount++
			holdRunBarsTotal += float64(holdRunLength)
			holdRunLength = 0
		}

		if i > 0 {
			stats.TransitionCount++
			previous := points[i-1]
			if previous.State == point.State {
				stableTransitions++
			}
			if previous.State != string(strategy.StateEntry) && point.State == string(strategy.StateEntry) {
				stats.EntryTransitionCount++
			}
		}

		tradeState := point.State == string(strategy.StateEntry) || point.State == string(strategy.StateHold)
		if tradeState {
			inTrade = true
		} else if inTrade {
			stats.ResolvedTradeCount++
			if point.State == string(strategy.StateExit) {
				stats.ExitAfterEntryCount++
			}
			inTrade = false
		}
	}

	if holdRunLength > 0 {
		holdRunCount++
		holdRunBarsTotal += float64(holdRunLength)
	}

	latest := points[len(points)-1]
	stats.PointCount = len(points)
	stats.LatestState = latest.State
	stats.LatestScore = roundTo(latest.Score, 2)
	stats.AverageScore = roundTo(totalScore/float64(len(points)), 2)
	stats.LatestStateAgeBars = 1
	for i := len(points) - 2; i >= 0; i-- {
		if points[i].State != latest.State {
			break
		}
		stats.LatestStateAgeBars++
	}
	if latestEntryIndex >= 0 {
		stats.BarsSinceEntry = len(points) - 1 - latestEntryIndex
	}
	if latestExitIndex >= 0 {
		stats.BarsSinceExit = len(points) - 1 - latestExitIndex
	}
	if len(points) > 1 {
		previous := points[len(points)-2]
		stats.JustChanged = previous.State != latest.State
		stats.JustEntered = latest.State == string(strategy.StateEntry) && previous.State != latest.State
		stats.JustExited = latest.State == string(strategy.StateExit) && previous.State != latest.State
	}

	if stats.TransitionCount > 0 {
		stats.EntryTransitionRate = roundTo((float64(stats.EntryTransitionCount)/float64(stats.TransitionCount))*100, 2)
		stats.StabilityRate = roundTo((float64(stableTransitions)/float64(stats.TransitionCount))*100, 2)
	}
	if holdRunCount > 0 {
		stats.AverageHoldBars = roundTo(holdRunBarsTotal/float64(holdRunCount), 2)
	}
	if stats.ResolvedTradeCount > 0 {
		stats.ExitAfterEntryRate = roundTo((float64(stats.ExitAfterEntryCount)/float64(stats.ResolvedTradeCount))*100, 2)
	}

	return stats
}

func buildOpportunity(market Market, history SignalHistory) Opportunity {
	leader := topStrategyEvaluation(market.Strategies)
	convergence := computeOpportunityConvergence(market.Strategies)
	freshness := computeOpportunityFreshness(history)
	priorityScore := computeOpportunityPriority(market, leader, convergence, freshness)
	priorityBand, primaryAction := classifyOpportunity(priorityScore, leader.State)
	summary := summarizeOpportunity(leader, convergence, freshness, priorityBand)

	reasons := append([]string(nil), leader.Reasons...)
	risks := append([]string(nil), leader.Risks...)
	if convergence.Consensus {
		reasons = append(reasons, "multi-profile alignment active")
	} else if convergence.ConstructiveAlignment {
		reasons = append(reasons, "multiple profiles remain constructive")
	}
	if freshness.HasFreshEntry {
		reasons = append(reasons, "recent entry transition detected")
	}
	if freshness.HasFreshExit {
		risks = append(risks, "recent exit transition detected")
	}

	return Opportunity{
		Symbol:        market.Symbol,
		PriorityScore: priorityScore,
		PriorityBand:  priorityBand,
		PrimaryAction: primaryAction,
		Summary:       summary,
		Leader:        leader,
		Convergence:   convergence,
		Freshness:     freshness,
		QuoteVolume:   market.QuoteVolume,
		QualityScore:  market.QualityScore,
		Change24h:     market.Change24h,
		Change1h:      market.Change1h,
		Change5m:      market.Change5m,
		Reasons:       uniqueStrings(reasons),
		Risks:         uniqueStrings(risks),
	}
}

func topStrategyEvaluation(strategies []StrategyEvaluation) StrategyEvaluation {
	if len(strategies) == 0 {
		return StrategyEvaluation{}
	}

	sorted := append([]StrategyEvaluation(nil), strategies...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Score == sorted[j].Score {
			return statePriority(sorted[i].State) > statePriority(sorted[j].State)
		}
		return sorted[i].Score > sorted[j].Score
	})

	return sorted[0]
}

func computeOpportunityConvergence(strategies []StrategyEvaluation) OpportunityConvergence {
	result := OpportunityConvergence{}
	for _, strategyEval := range strategies {
		switch strategyEval.State {
		case string(strategy.StateEntry), string(strategy.StateHold):
			result.ActiveProfiles++
			result.ConstructiveProfiles++
		case string(strategy.StateWatch):
			result.ConstructiveProfiles++
		}
	}
	result.Consensus = result.ActiveProfiles >= 2
	result.ConstructiveAlignment = result.ConstructiveProfiles >= 2
	return result
}

func computeOpportunityFreshness(history SignalHistory) OpportunityFreshness {
	result := OpportunityFreshness{
		YoungestEntryBars: -1,
		YoungestExitBars:  -1,
		YoungestStateBars: -1,
	}

	for _, profile := range history.Profiles {
		stats := profile.Stats
		if stats.JustChanged {
			result.ChangedProfiles++
		}
		if stats.JustEntered {
			result.HasFreshEntry = true
			result.FreshEntryProfiles = append(result.FreshEntryProfiles, profile.Label)
		}
		if stats.JustExited {
			result.HasFreshExit = true
			result.FreshExitProfiles = append(result.FreshExitProfiles, profile.Label)
		}
		if stats.BarsSinceEntry >= 0 && (result.YoungestEntryBars < 0 || stats.BarsSinceEntry < result.YoungestEntryBars) {
			result.YoungestEntryBars = stats.BarsSinceEntry
		}
		if stats.BarsSinceExit >= 0 && (result.YoungestExitBars < 0 || stats.BarsSinceExit < result.YoungestExitBars) {
			result.YoungestExitBars = stats.BarsSinceExit
		}
		if stats.LatestStateAgeBars > 0 && (result.YoungestStateBars < 0 || stats.LatestStateAgeBars < result.YoungestStateBars) {
			result.YoungestStateBars = stats.LatestStateAgeBars
		}
	}

	result.FreshEntryProfiles = uniqueStrings(result.FreshEntryProfiles)
	result.FreshExitProfiles = uniqueStrings(result.FreshExitProfiles)
	return result
}

func computeOpportunityPriority(market Market, leader StrategyEvaluation, convergence OpportunityConvergence, freshness OpportunityFreshness) float64 {
	priority := market.QualityScore*0.28 + leader.Score*0.30
	priority += float64(convergence.ActiveProfiles) * 12
	priority += float64(convergence.ConstructiveProfiles) * 4
	priority += clampPositive(market.Change1h*1.1, 0, 8)
	priority += clampPositive(market.Change5m*2.0, 0, 6)

	switch leader.State {
	case string(strategy.StateEntry):
		priority += 8
	case string(strategy.StateHold):
		priority += 4
	case string(strategy.StateWatch):
		priority += 1
	case string(strategy.StateExit):
		priority -= 16
	case string(strategy.StateAvoid):
		priority -= 10
	}

	if freshness.HasFreshEntry {
		priority += 12
	}
	if freshness.HasFreshExit {
		priority -= 8
	}
	if freshness.YoungestEntryBars >= 0 && freshness.YoungestEntryBars <= 2 {
		priority += 4
	}
	if freshness.YoungestStateBars > 6 {
		priority -= 3
	}

	return roundTo(clampPositive(priority, 0, 100), 2)
}

func classifyOpportunity(priorityScore float64, leaderState string) (string, string) {
	switch {
	case priorityScore >= 80 && (leaderState == string(strategy.StateEntry) || leaderState == string(strategy.StateHold)):
		return "actionable", "act-now"
	case priorityScore >= 65:
		return "strong-watch", "watch-closely"
	case priorityScore >= 50:
		return "watchlist", "prepare"
	default:
		return "defensive", "avoid"
	}
}

func summarizeOpportunity(leader StrategyEvaluation, convergence OpportunityConvergence, freshness OpportunityFreshness, priorityBand string) string {
	switch {
	case freshness.HasFreshEntry && convergence.Consensus:
		return "Fresh multi-profile entry detected"
	case freshness.HasFreshEntry && leader.State == string(strategy.StateEntry):
		return "Fresh entry led by the strongest profile"
	case freshness.HasFreshExit:
		return "Recent exit transition requires caution"
	case convergence.Consensus && (leader.State == string(strategy.StateEntry) || leader.State == string(strategy.StateHold)):
		return "High-conviction alignment remains active"
	case priorityBand == "strong-watch":
		return "Constructive setup worth close monitoring"
	case priorityBand == "watchlist":
		return "Setup is building but not fully confirmed"
	default:
		return "Context remains defensive or low conviction"
	}
}

func statePriority(state string) int {
	switch state {
	case string(strategy.StateEntry):
		return 5
	case string(strategy.StateHold):
		return 4
	case string(strategy.StateWatch):
		return 3
	case string(strategy.StateAvoid):
		return 2
	case string(strategy.StateExit):
		return 1
	default:
		return 0
	}
}

func clampPositive(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

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

func toScoringCandles(candles []exchange.Candle) []scoring.Candle {
	if len(candles) == 0 {
		return nil
	}

	sorted := make([]exchange.Candle, 0, len(candles))
	for _, candle := range candles {
		if candle.Open <= 0 || candle.High <= 0 || candle.Low <= 0 || candle.Close <= 0 {
			continue
		}
		sorted = append(sorted, candle)
	}
	if len(sorted) == 0 {
		return nil
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].OpenTime.Before(sorted[j].OpenTime)
	})

	scoringCandles := make([]scoring.Candle, 0, len(sorted))
	for _, candle := range sorted {
		scoringCandles = append(scoringCandles, scoring.Candle{
			Open:      candle.Open,
			High:      candle.High,
			Low:       candle.Low,
			Close:     candle.Close,
			Volume:    candle.Volume,
			OpenTime:  candle.OpenTime,
			CloseTime: candle.CloseTime,
		})
	}

	return scoringCandles
}

func computeWindowChange(candles []exchange.Candle, window time.Duration) (float64, bool) {
	if len(candles) < 2 {
		return 0, false
	}

	sorted := make([]exchange.Candle, 0, len(candles))
	for _, c := range candles {
		if c.Open <= 0 || c.Close <= 0 || c.OpenTime.IsZero() {
			continue
		}
		sorted = append(sorted, c)
	}
	if len(sorted) < 2 {
		return 0, false
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].OpenTime.Before(sorted[j].OpenTime)
	})

	latest := sorted[len(sorted)-1]
	latestTime := latest.CloseTime
	if latestTime.IsZero() {
		latestTime = latest.OpenTime.Add(time.Minute)
	}
	targetTime := latestTime.Add(-window)

	refIdx := -1
	for i := len(sorted) - 1; i >= 0; i-- {
		if !sorted[i].OpenTime.After(targetTime) {
			refIdx = i
			break
		}
	}
	if refIdx < 0 {
		return 0, false
	}

	ref := sorted[refIdx]
	if ref.Open <= 0 || latest.Close <= 0 {
		return 0, false
	}

	actualSpan := latestTime.Sub(ref.OpenTime)
	minSpan := window - 2*time.Minute
	if window >= time.Hour {
		minSpan = window - 10*time.Minute
	}
	if actualSpan < minSpan {
		return 0, false
	}

	change := ((latest.Close - ref.Open) / ref.Open) * 100
	return change, true
}
