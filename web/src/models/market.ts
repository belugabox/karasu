import { toBoolean, toNumber, toStringValue } from '../utils/parse'

export type MarketSortKey = 'priority' | 'score' | 'strategyScore' | 'convergence' | 'change24h' | 'change1h' | 'change5m' | 'quoteVolume'

export type StrategyEvaluation = {
  name: string
  label: string
  description: string
  icon: string
  color: string
  state: string
  score: number
  reasons: string[]
  risks: string[]
}

export type StrategySignalPoint = {
  openTime: string
  closeTime: string
  state: string
  score: number
}

export type StrategySignalStats = {
  pointCount: number
  latestState: string
  latestScore: number
  latestStateAgeBars: number
  averageScore: number
  entryCount: number
  holdCount: number
  watchCount: number
  exitCount: number
  avoidCount: number
  barsSinceEntry: number
  barsSinceExit: number
  justChanged: boolean
  justEntered: boolean
  justExited: boolean
  transitionCount: number
  entryTransitionCount: number
  entryTransitionRate: number
  averageHoldBars: number
  resolvedTradeCount: number
  exitAfterEntryCount: number
  exitAfterEntryRate: number
  stabilityRate: number
}

export type StrategySignalHistory = {
  name: string
  label: string
  description: string
  icon: string
  color: string
  stats: StrategySignalStats
  points: StrategySignalPoint[]
}

export type OpportunityConvergence = {
  activeProfiles: number
  constructiveProfiles: number
  consensus: boolean
  constructiveAlignment: boolean
}

export type OpportunityFreshness = {
  changedProfiles: number
  hasFreshEntry: boolean
  hasFreshExit: boolean
  freshEntryProfiles: string[]
  freshExitProfiles: string[]
  youngestEntryBars: number
  youngestExitBars: number
  youngestStateBars: number
}

export type Opportunity = {
  symbol: string
  priorityScore: number
  priorityBand: string
  primaryAction: string
  summary: string
  leader: StrategyEvaluation
  convergence: OpportunityConvergence
  freshness: OpportunityFreshness
  quoteVolume: number
  qualityScore: number
  change24h: number
  change1h: number
  change5m: number
  reasons: string[]
  risks: string[]
}

export type Market = {
  symbol: string
  strategies: StrategyEvaluation[]
  quoteVolume: number
  qualityScore: number
  qualityRsi: number
  qualityMacd: number
  qualityBollinger: number
  qualityVolume: number
  qualitySma: number
  change24h: number
  change1h: number
  change5m: number
}

export type MarketAnalysis = {
  symbol: string
  quoteVolume: number
  change24h: number
  change1h: number
  change5m: number
  candleCount1m: number
  qualityScore: number
  qualityRsi: number
  qualityMacd: number
  qualityBollinger: number
  qualityVolume: number
  qualitySma: number
  strategies: StrategyEvaluation[]
}

export type MarketSignalHistory = {
  symbol: string
  timeframe: string
  candleCount: number
  profiles: StrategySignalHistory[]
}

export type SystemHealth = {
  generatedAt: string
  isHealthy: boolean
  issues: string[]
  universeSymbols: number
  topSymbols: number
  liveSymbols: number
  liveLastUpdatedAt: string
  liveFresh: boolean
  staleThresholdMin: number
  topSymbolsStale5m: number
  topStaleExamples: string[]
  storeReadErrors: number
  backfillQueueDepth: number
  backfillQueueCap: number
  backfillQueuedJobs: number
  backfillRunningJobs: number
  backfillFailedJobs24h: number
}

export type AlertEvent = {
  id: string
  key: string
  category: string
  severity: string
  message: string
  source: string
  symbol: string
  active: boolean
  count: number
  firstSeen: string
  lastSeen: string
}

function normalizeStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => toStringValue(item)).filter(Boolean) : []
}

export function normalizeStrategyEvaluation(raw: unknown): StrategyEvaluation {
  const r = (raw as Record<string, unknown>) || {}

  return {
    name: toStringValue(r.name ?? r.Name),
    label: toStringValue(r.label ?? r.Label),
    description: toStringValue(r.description ?? r.Description),
    icon: toStringValue(r.icon ?? r.Icon),
    color: toStringValue(r.color ?? r.Color),
    state: toStringValue(r.state ?? r.State),
    score: toNumber(r.score ?? r.Score),
    reasons: normalizeStringArray(r.reasons ?? r.Reasons),
    risks: normalizeStringArray(r.risks ?? r.Risks),
  }
}

export function normalizeStrategySignalPoint(raw: unknown): StrategySignalPoint {
  const r = (raw as Record<string, unknown>) || {}

  return {
    openTime: toStringValue(r.openTime ?? r.OpenTime),
    closeTime: toStringValue(r.closeTime ?? r.CloseTime),
    state: toStringValue(r.state ?? r.State),
    score: toNumber(r.score ?? r.Score),
  }
}

export function normalizeStrategySignalStats(raw: unknown): StrategySignalStats {
  const r = (raw as Record<string, unknown>) || {}

  return {
    pointCount: toNumber(r.pointCount ?? r.PointCount),
    latestState: toStringValue(r.latestState ?? r.LatestState),
    latestScore: toNumber(r.latestScore ?? r.LatestScore),
    latestStateAgeBars: toNumber(r.latestStateAgeBars ?? r.LatestStateAgeBars),
    averageScore: toNumber(r.averageScore ?? r.AverageScore),
    entryCount: toNumber(r.entryCount ?? r.EntryCount),
    holdCount: toNumber(r.holdCount ?? r.HoldCount),
    watchCount: toNumber(r.watchCount ?? r.WatchCount),
    exitCount: toNumber(r.exitCount ?? r.ExitCount),
    avoidCount: toNumber(r.avoidCount ?? r.AvoidCount),
    barsSinceEntry: toNumber(r.barsSinceEntry ?? r.BarsSinceEntry),
    barsSinceExit: toNumber(r.barsSinceExit ?? r.BarsSinceExit),
    justChanged: toBoolean(r.justChanged ?? r.JustChanged),
    justEntered: toBoolean(r.justEntered ?? r.JustEntered),
    justExited: toBoolean(r.justExited ?? r.JustExited),
    transitionCount: toNumber(r.transitionCount ?? r.TransitionCount),
    entryTransitionCount: toNumber(r.entryTransitionCount ?? r.EntryTransitionCount),
    entryTransitionRate: toNumber(r.entryTransitionRate ?? r.EntryTransitionRate),
    averageHoldBars: toNumber(r.averageHoldBars ?? r.AverageHoldBars),
    resolvedTradeCount: toNumber(r.resolvedTradeCount ?? r.ResolvedTradeCount),
    exitAfterEntryCount: toNumber(r.exitAfterEntryCount ?? r.ExitAfterEntryCount),
    exitAfterEntryRate: toNumber(r.exitAfterEntryRate ?? r.ExitAfterEntryRate),
    stabilityRate: toNumber(r.stabilityRate ?? r.StabilityRate),
  }
}

export function normalizeStrategySignalHistory(raw: unknown): StrategySignalHistory {
  const r = (raw as Record<string, unknown>) || {}
  const pointsRaw = r.points ?? r.Points
  const statsRaw = r.stats ?? r.Stats

  return {
    name: toStringValue(r.name ?? r.Name),
    label: toStringValue(r.label ?? r.Label),
    description: toStringValue(r.description ?? r.Description),
    icon: toStringValue(r.icon ?? r.Icon),
    color: toStringValue(r.color ?? r.Color),
    stats: normalizeStrategySignalStats(statsRaw),
    points: Array.isArray(pointsRaw) ? pointsRaw.map(normalizeStrategySignalPoint) : [],
  }
}

export function normalizeOpportunityConvergence(raw: unknown): OpportunityConvergence {
  const r = (raw as Record<string, unknown>) || {}

  return {
    activeProfiles: toNumber(r.activeProfiles ?? r.ActiveProfiles),
    constructiveProfiles: toNumber(r.constructiveProfiles ?? r.ConstructiveProfiles),
    consensus: toBoolean(r.consensus ?? r.Consensus),
    constructiveAlignment: toBoolean(r.constructiveAlignment ?? r.ConstructiveAlignment),
  }
}

export function normalizeOpportunityFreshness(raw: unknown): OpportunityFreshness {
  const r = (raw as Record<string, unknown>) || {}

  return {
    changedProfiles: toNumber(r.changedProfiles ?? r.ChangedProfiles),
    hasFreshEntry: toBoolean(r.hasFreshEntry ?? r.HasFreshEntry),
    hasFreshExit: toBoolean(r.hasFreshExit ?? r.HasFreshExit),
    freshEntryProfiles: normalizeStringArray(r.freshEntryProfiles ?? r.FreshEntryProfiles),
    freshExitProfiles: normalizeStringArray(r.freshExitProfiles ?? r.FreshExitProfiles),
    youngestEntryBars: toNumber(r.youngestEntryBars ?? r.YoungestEntryBars),
    youngestExitBars: toNumber(r.youngestExitBars ?? r.YoungestExitBars),
    youngestStateBars: toNumber(r.youngestStateBars ?? r.YoungestStateBars),
  }
}

export function normalizeOpportunity(raw: unknown): Opportunity {
  const r = (raw as Record<string, unknown>) || {}

  return {
    symbol: toStringValue(r.symbol ?? r.Symbol),
    priorityScore: toNumber(r.priorityScore ?? r.PriorityScore),
    priorityBand: toStringValue(r.priorityBand ?? r.PriorityBand),
    primaryAction: toStringValue(r.primaryAction ?? r.PrimaryAction),
    summary: toStringValue(r.summary ?? r.Summary),
    leader: normalizeStrategyEvaluation(r.leader ?? r.Leader),
    convergence: normalizeOpportunityConvergence(r.convergence ?? r.Convergence),
    freshness: normalizeOpportunityFreshness(r.freshness ?? r.Freshness),
    quoteVolume: toNumber(r.quoteVolume ?? r.QuoteVolume),
    qualityScore: toNumber(r.qualityScore ?? r.QualityScore),
    change24h: toNumber(r.change24h ?? r.Change24h),
    change1h: toNumber(r.change1h ?? r.Change1h),
    change5m: toNumber(r.change5m ?? r.Change5m),
    reasons: normalizeStringArray(r.reasons ?? r.Reasons),
    risks: normalizeStringArray(r.risks ?? r.Risks),
  }
}

export function normalizeMarket(raw: unknown): Market {
  const r = (raw as Record<string, unknown>) || {}
  const strategiesRaw = r.strategies ?? r.Strategies

  return {
    symbol: toStringValue(r.symbol ?? r.Symbol),
    strategies: Array.isArray(strategiesRaw) ? strategiesRaw.map(normalizeStrategyEvaluation) : [],
    quoteVolume: toNumber(r.quoteVolume ?? r.QuoteVolume),
    qualityScore: toNumber(r.qualityScore ?? r.QualityScore),
    qualityRsi: toNumber(r.qualityRsi ?? r.QualityRSI),
    qualityMacd: toNumber(r.qualityMacd ?? r.QualityMACD),
    qualityBollinger: toNumber(r.qualityBollinger ?? r.QualityBollinger),
    qualityVolume: toNumber(r.qualityVolume ?? r.QualityVolume),
    qualitySma: toNumber(r.qualitySma ?? r.QualitySMA),
    change24h: toNumber(r.change24h ?? r.Change24h),
    change1h: toNumber(r.change1h ?? r.Change1h),
    change5m: toNumber(r.change5m ?? r.Change5m),
  }
}

export function normalizeMarketAnalysis(raw: unknown): MarketAnalysis {
  const r = (raw as Record<string, unknown>) || {}
  const qualityRaw = ((r.quality ?? r.Quality) as Record<string, unknown> | undefined) || {}
  const strategiesRaw = r.strategies ?? r.Strategies

  return {
    symbol: toStringValue(r.symbol ?? r.Symbol),
    quoteVolume: toNumber(r.quoteVolume ?? r.QuoteVolume),
    change24h: toNumber(r.change24h ?? r.Change24h),
    change1h: toNumber(r.change1h ?? r.Change1h),
    change5m: toNumber(r.change5m ?? r.Change5m),
    candleCount1m: toNumber(r.candleCount1m ?? r.CandleCount1m),
    qualityScore: toNumber(qualityRaw.score ?? qualityRaw.Score),
    qualityRsi: toNumber(qualityRaw.rsi ?? qualityRaw.RSI),
    qualityMacd: toNumber(qualityRaw.macd ?? qualityRaw.MACD),
    qualityBollinger: toNumber(qualityRaw.bollinger ?? qualityRaw.Bollinger),
    qualityVolume: toNumber(qualityRaw.volume ?? qualityRaw.Volume),
    qualitySma: toNumber(qualityRaw.sma ?? qualityRaw.SMA),
    strategies: Array.isArray(strategiesRaw) ? strategiesRaw.map(normalizeStrategyEvaluation) : [],
  }
}

export function normalizeMarketSignalHistory(raw: unknown): MarketSignalHistory {
  const r = (raw as Record<string, unknown>) || {}
  const profilesRaw = r.profiles ?? r.Profiles

  return {
    symbol: toStringValue(r.symbol ?? r.Symbol),
    timeframe: toStringValue(r.timeframe ?? r.Timeframe),
    candleCount: toNumber(r.candleCount ?? r.CandleCount),
    profiles: Array.isArray(profilesRaw) ? profilesRaw.map(normalizeStrategySignalHistory) : [],
  }
}

export function normalizeSystemHealth(raw: unknown): SystemHealth {
  const r = (raw as Record<string, unknown>) || {}

  return {
    generatedAt: toStringValue(r.generatedAt ?? r.GeneratedAt),
    isHealthy: toBoolean(r.isHealthy ?? r.IsHealthy),
    issues: normalizeStringArray(r.issues ?? r.Issues),
    universeSymbols: toNumber(r.universeSymbols ?? r.UniverseSymbols),
    topSymbols: toNumber(r.topSymbols ?? r.TopSymbols),
    liveSymbols: toNumber(r.liveSymbols ?? r.LiveSymbols),
    liveLastUpdatedAt: toStringValue(r.liveLastUpdatedAt ?? r.LiveLastUpdatedAt),
    liveFresh: toBoolean(r.liveFresh ?? r.LiveFresh),
    staleThresholdMin: toNumber(r.staleThresholdMin ?? r.StaleThresholdMin),
    topSymbolsStale5m: toNumber(r.topSymbolsStale5m ?? r.TopSymbolsStale5m),
    topStaleExamples: normalizeStringArray(r.topStaleExamples ?? r.TopStaleExamples),
    storeReadErrors: toNumber(r.storeReadErrors ?? r.StoreReadErrors),
    backfillQueueDepth: toNumber(r.backfillQueueDepth ?? r.BackfillQueueDepth),
    backfillQueueCap: toNumber(r.backfillQueueCap ?? r.BackfillQueueCap),
    backfillQueuedJobs: toNumber(r.backfillQueuedJobs ?? r.BackfillQueuedJobs),
    backfillRunningJobs: toNumber(r.backfillRunningJobs ?? r.BackfillRunningJobs),
    backfillFailedJobs24h: toNumber(r.backfillFailedJobs24h ?? r.BackfillFailedJobs24),
  }
}

export function normalizeAlertEvent(raw: unknown): AlertEvent {
  const r = (raw as Record<string, unknown>) || {}

  return {
    id: toStringValue(r.id ?? r.ID),
    key: toStringValue(r.key ?? r.Key),
    category: toStringValue(r.category ?? r.Category),
    severity: toStringValue(r.severity ?? r.Severity),
    message: toStringValue(r.message ?? r.Message),
    source: toStringValue(r.source ?? r.Source),
    symbol: toStringValue(r.symbol ?? r.Symbol),
    active: toBoolean(r.active ?? r.Active),
    count: toNumber(r.count ?? r.Count),
    firstSeen: toStringValue(r.firstSeen ?? r.FirstSeen),
    lastSeen: toStringValue(r.lastSeen ?? r.LastSeen),
  }
}
