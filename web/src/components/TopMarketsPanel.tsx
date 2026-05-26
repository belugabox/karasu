import { useCallback, useMemo, useState } from 'react'
import { usePolling } from '../hooks/usePolling'
import { type AlertEvent, type Market, type MarketAnalysis, type MarketSignalHistory, type MarketSortKey, type Opportunity, type SystemHealth } from '../models/market'
import { getMarketAnalysis, getMarketSignalHistory, getMarkets, getOpportunities, getRecentAlerts, getSystemHealth } from '../services/marketService'
import { formatNumber, formatSignedPercent } from '../utils/format'

const strategyProfiles = [
  { value: 'all', label: 'Tous les profils' },
  { value: 'intraday-momentum', label: 'Momentum intraday' },
  { value: 'swing-balance', label: 'Swing equilibre' },
  { value: 'trend-follow', label: 'Suivi de tendance' },
]

const strategyStates = [
  { value: 'all', label: 'Tous les etats' },
  { value: 'entry', label: 'Entree' },
  { value: 'hold', label: 'Maintien' },
  { value: 'watch', label: 'Surveillance' },
  { value: 'exit', label: 'Sortie' },
  { value: 'avoid', label: 'A eviter' },
]

const priorityBands = [
  { value: 'all', label: 'Toutes les priorites' },
  { value: 'actionable', label: 'Actionnable' },
  { value: 'strong-watch', label: 'Surveillance forte' },
  { value: 'watchlist', label: 'Liste de surveillance' },
  { value: 'defensive', label: 'Defensif' },
]

const strategyStatePriority: Record<string, number> = {
  entry: 5,
  hold: 4,
  watch: 3,
  avoid: 2,
  exit: 1,
}

function getStrategyScoreForSort(strategies: Market['strategies'], profileName: string): number {
  if (profileName === 'all') {
    return strategies.reduce((best, strategy) => Math.max(best, strategy.score), -1)
  }

  return strategies.find((strategy) => strategy.name === profileName)?.score ?? -1
}

function sortStrategiesForComparison(strategies: Market['strategies']): Market['strategies'] {
  return [...strategies].sort((left, right) => {
    if (right.score !== left.score) {
      return right.score - left.score
    }

    return (strategyStatePriority[right.state] ?? 0) - (strategyStatePriority[left.state] ?? 0)
  })
}

function getStrategyConvergence(strategies: Market['strategies']) {
  const active = strategies.filter((strategy) => strategy.state === 'entry' || strategy.state === 'hold')
  const constructive = strategies.filter(
    (strategy) => strategy.state === 'entry' || strategy.state === 'hold' || strategy.state === 'watch',
  )

  return {
    activeCount: active.length,
    constructiveCount: constructive.length,
    hasConsensus: active.length >= 2,
    hasConstructiveAlignment: constructive.length >= 2,
  }
}

function getConvergenceSortScore(strategies: Market['strategies']): number {
  const convergence = getStrategyConvergence(strategies)
  const rankedStrategies = sortStrategiesForComparison(strategies)
  const topScore = rankedStrategies[0]?.score ?? 0

  return convergence.activeCount * 1000 + convergence.constructiveCount * 100 + topScore
}

function formatBarsAgo(value: number): string {
  if (value < 0) {
    return 'non observe recemment'
  }

  if (value === 0) {
    return 'sur cette bougie'
  }

  if (value === 1) {
    return 'il y a 1 bougie'
  }

  return `il y a ${value} bougies`
}

function translateProfileLabel(label: string): string {
  switch (label) {
    case 'Intraday Momentum':
      return 'Momentum intraday'
    case 'Swing Balance':
      return 'Swing equilibre'
    case 'Trend Follow':
      return 'Suivi de tendance'
    default:
      return label
  }
}

function translateStateLabel(state: string): string {
  switch (state) {
    case 'entry':
      return 'entree'
    case 'hold':
      return 'maintien'
    case 'watch':
      return 'surveillance'
    case 'exit':
      return 'sortie'
    case 'avoid':
      return 'a eviter'
    default:
      return state
  }
}

function translatePriorityBand(band: string): string {
  switch (band) {
    case 'actionable':
      return 'actionnable'
    case 'strong-watch':
      return 'surveillance forte'
    case 'watchlist':
      return 'liste de surveillance'
    case 'defensive':
      return 'defensif'
    default:
      return band
  }
}

function translatePrimaryAction(action: string): string {
  switch (action) {
    case 'act-now':
      return 'agir maintenant'
    case 'watch-closely':
      return 'surveiller de pres'
    case 'prepare':
      return 'preparer'
    case 'avoid':
      return 'eviter'
    default:
      return action
  }
}

function translateOpportunitySummary(summary: string): string {
  switch (summary) {
    case 'Fresh multi-profile entry detected':
      return 'Nouvelle entree detectee avec alignement de plusieurs profils'
    case 'Fresh entry led by the strongest profile':
      return 'Nouvelle entree portee par le profil le plus fort'
    case 'Recent exit transition requires caution':
      return 'Une sortie recente impose de la prudence'
    case 'High-conviction alignment remains active':
      return 'L alignement a forte conviction reste actif'
    case 'Constructive setup worth close monitoring':
      return 'Configuration constructive a surveiller de pres'
    case 'Setup is building but not fully confirmed':
      return 'La configuration se met en place mais reste a confirmer'
    case 'Context remains defensive or low conviction':
      return 'Le contexte reste defensif ou peu convaincant'
    default:
      return summary
  }
}

function translateReason(reason: string): string {
  switch (reason) {
    case 'multi-profile alignment active':
      return 'alignement multi-profils actif'
    case 'multiple profiles remain constructive':
      return 'plusieurs profils restent constructifs'
    case 'recent entry transition detected':
      return 'transition recente vers entree detectee'
    case 'momentum confirmed':
      return 'momentum confirme'
    case 'recent price thrust matches profile':
      return 'l acceleration recente des prix correspond au profil'
    case 'macd momentum confirmed':
      return 'momentum MACD confirme'
    case 'volume participation supportive':
      return 'le volume soutient le mouvement'
    case 'trend structure aligned':
      return 'la structure de tendance est alignee'
    case 'price location remains tradable':
      return 'la position du prix reste exploitable'
    case 'rsi balance is constructive':
      return 'l equilibre RSI est constructif'
    default:
      return reason
  }
}

function translateRisk(risk: string): string {
  switch (risk) {
    case 'recent exit transition detected':
      return 'transition recente vers sortie detectee'
    case 'insufficient history':
      return 'historique insuffisant'
    case 'recent price thrust is insufficient':
      return 'l acceleration recente des prix est insuffisante'
    case 'macd momentum too weak':
      return 'le momentum MACD est trop faible'
    case 'volume confirmation missing':
      return 'la confirmation par le volume manque'
    case 'trend structure below profile floor':
      return 'la structure de tendance est sous le seuil du profil'
    case 'price location too stretched or weak':
      return 'la position du prix est trop etiree ou trop faible'
    case 'rsi balance is weak':
      return 'l equilibre RSI est faible'
    default:
      return risk
  }
}

function translateHealthIssue(issue: string): string {
  if (issue === 'flux live 1m stale') {
    return 'flux live 1m en retard'
  }
  return issue
}

function translateAlertSeverity(severity: string): string {
  switch (severity) {
    case 'critical':
      return 'critique'
    case 'warning':
      return 'alerte'
    case 'info':
      return 'info'
    default:
      return severity
  }
}

export function TopMarketsPanel() {
  const [opportunities, setOpportunities] = useState<Opportunity[]>([])
  const [opportunitiesLoading, setOpportunitiesLoading] = useState(false)
  const [opportunitiesError, setOpportunitiesError] = useState('')
  const [markets, setMarkets] = useState<Market[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [sortKey, setSortKey] = useState<MarketSortKey>('priority')
  const [selectedSymbol, setSelectedSymbol] = useState('')
  const [analysis, setAnalysis] = useState<MarketAnalysis | null>(null)
  const [analysisLoading, setAnalysisLoading] = useState(false)
  const [analysisError, setAnalysisError] = useState('')
  const [signalHistory, setSignalHistory] = useState<MarketSignalHistory | null>(null)
  const [signalHistoryLoading, setSignalHistoryLoading] = useState(false)
  const [signalHistoryError, setSignalHistoryError] = useState('')
  const [systemHealth, setSystemHealth] = useState<SystemHealth | null>(null)
  const [systemHealthLoading, setSystemHealthLoading] = useState(false)
  const [systemHealthError, setSystemHealthError] = useState('')
  const [recentAlerts, setRecentAlerts] = useState<AlertEvent[]>([])
  const [recentAlertsLoading, setRecentAlertsLoading] = useState(false)
  const [recentAlertsError, setRecentAlertsError] = useState('')
  const [strategyProfileFilter, setStrategyProfileFilter] = useState('all')
  const [strategyStateFilter, setStrategyStateFilter] = useState('all')
  const [freshOnly, setFreshOnly] = useState(false)
  const [consensusOnly, setConsensusOnly] = useState(false)
  const [priorityBandFilter, setPriorityBandFilter] = useState('all')

  const selectedProfileLabel = useMemo(() => {
    return strategyProfiles.find((profile) => profile.value === strategyProfileFilter)?.label ?? 'profil selectionne'
  }, [strategyProfileFilter])

  const opportunityBySymbol = useMemo(() => {
    return new Map(opportunities.map((opportunity) => [opportunity.symbol, opportunity]))
  }, [opportunities])

  const loadOpportunities = useCallback(async () => {
    setOpportunitiesLoading(true)
    try {
      const nextOpportunities = await getOpportunities(18)
      setOpportunities(nextOpportunities)
      setOpportunitiesError('')
    } catch (err) {
      setOpportunitiesError(err instanceof Error ? err.message : 'echec du chargement des opportunites')
    } finally {
      setOpportunitiesLoading(false)
    }
  }, [])

  usePolling(loadOpportunities, 60_000)

  const loadMarkets = useCallback(async () => {
    setLoading(true)
    try {
      const nextMarkets = await getMarkets()
      setMarkets(nextMarkets)
      setSelectedSymbol((current) => {
        if (current !== '' && nextMarkets.some((market) => market.symbol === current)) {
          return current
        }
        return nextMarkets[0]?.symbol ?? ''
      })
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'echec du chargement des marches')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(loadMarkets, 60_000)

  const loadAnalysis = useCallback(async () => {
    if (selectedSymbol === '') {
      setAnalysis(null)
      return
    }

    setAnalysisLoading(true)
    try {
      const nextAnalysis = await getMarketAnalysis(selectedSymbol)
      setAnalysis(nextAnalysis)
      setAnalysisError('')
    } catch (err) {
      setAnalysisError(err instanceof Error ? err.message : 'echec du chargement de l analyse du marche')
    } finally {
      setAnalysisLoading(false)
    }
  }, [selectedSymbol])

  usePolling(loadAnalysis, 60_000, selectedSymbol !== '')

  const loadSignalHistory = useCallback(async () => {
    if (selectedSymbol === '') {
      setSignalHistory(null)
      return
    }

    setSignalHistoryLoading(true)
    try {
      const nextHistory = await getMarketSignalHistory(selectedSymbol, 24)
      setSignalHistory(nextHistory)
      setSignalHistoryError('')
    } catch (err) {
      setSignalHistoryError(err instanceof Error ? err.message : 'echec du chargement de l historique des signaux')
    } finally {
      setSignalHistoryLoading(false)
    }
  }, [selectedSymbol])

  usePolling(loadSignalHistory, 60_000, selectedSymbol !== '')

  const loadSystemHealth = useCallback(async () => {
    setSystemHealthLoading(true)
    try {
      const nextHealth = await getSystemHealth(20)
      setSystemHealth(nextHealth)
      setSystemHealthError('')
    } catch (err) {
      setSystemHealthError(err instanceof Error ? err.message : 'echec du chargement de la sante systeme')
    } finally {
      setSystemHealthLoading(false)
    }
  }, [])

  usePolling(loadSystemHealth, 30_000)

  const loadRecentAlerts = useCallback(async () => {
    setRecentAlertsLoading(true)
    try {
      const alerts = await getRecentAlerts(20, false)
      setRecentAlerts(alerts)
      setRecentAlertsError('')
    } catch (err) {
      setRecentAlertsError(err instanceof Error ? err.message : 'echec du chargement des alertes recentes')
    } finally {
      setRecentAlertsLoading(false)
    }
  }, [])

  usePolling(loadRecentAlerts, 30_000)

  const sortedMarkets = useMemo(() => {
    const cloned = markets.filter((market) => {
      if (strategyProfileFilter === 'all' && strategyStateFilter === 'all') {
        return true
      }

      return market.strategies.some((strategy) => {
        const profileMatches = strategyProfileFilter === 'all' || strategy.name === strategyProfileFilter
        const stateMatches = strategyStateFilter === 'all' || strategy.state === strategyStateFilter
        return profileMatches && stateMatches
      })
    })

    cloned.sort((left, right) => {
      const leftStrategyScore = getStrategyScoreForSort(left.strategies, strategyProfileFilter)
      const rightStrategyScore = getStrategyScoreForSort(right.strategies, strategyProfileFilter)
      const leftPriority = opportunityBySymbol.get(left.symbol)?.priorityScore ?? -1
      const rightPriority = opportunityBySymbol.get(right.symbol)?.priorityScore ?? -1

      switch (sortKey) {
        case 'priority':
          return rightPriority - leftPriority
        case 'strategyScore':
          return rightStrategyScore - leftStrategyScore
        case 'convergence':
          return getConvergenceSortScore(right.strategies) - getConvergenceSortScore(left.strategies)
        case 'change24h':
          return right.change24h - left.change24h
        case 'change1h':
          return right.change1h - left.change1h
        case 'change5m':
          return right.change5m - left.change5m
        case 'quoteVolume':
          return right.quoteVolume - left.quoteVolume
        case 'score':
        default:
          return right.qualityScore - left.qualityScore
      }
    })

    return cloned
  }, [markets, opportunityBySymbol, sortKey, strategyProfileFilter, strategyStateFilter])

  const historyTimeframeLabel = signalHistory?.timeframe ?? '5m'

  const scannerOpportunities = useMemo(() => {
    return opportunities.filter((opportunity) => {
      if (priorityBandFilter !== 'all' && opportunity.priorityBand !== priorityBandFilter) {
        return false
      }
      if (freshOnly && !opportunity.freshness.hasFreshEntry) {
        return false
      }
      if (consensusOnly && !opportunity.convergence.consensus) {
        return false
      }
      return true
    })
  }, [opportunities, priorityBandFilter, freshOnly, consensusOnly])

  const scannerOverview = useMemo(() => {
    return scannerOpportunities.reduce(
      (accumulator, opportunity) => {
        if (opportunity.priorityBand === 'actionable') {
          accumulator.actionableCount++
        }
        if (opportunity.freshness.hasFreshEntry) {
          accumulator.freshEntryCount++
        }
        if (opportunity.convergence.consensus) {
          accumulator.consensusCount++
        }
        if (opportunity.priorityBand === 'defensive') {
          accumulator.defensiveCount++
        }
        return accumulator
      },
      {
        actionableCount: 0,
        freshEntryCount: 0,
        consensusCount: 0,
        defensiveCount: 0,
      },
    )
  }, [scannerOpportunities])

  const marketOverview = useMemo(() => {
    const topMarkets = sortedMarkets.slice(0, 30)
    const summary = topMarkets.reduce(
      (accumulator, market) => {
        const convergence = getStrategyConvergence(market.strategies)
        const rankedStrategies = sortStrategiesForComparison(market.strategies)
        const leader = rankedStrategies[0]

        if (convergence.hasConsensus) {
          accumulator.consensusCount++
        }
        if (!convergence.hasConsensus && convergence.hasConstructiveAlignment) {
          accumulator.constructiveCount++
        }
        if (leader?.state === 'entry') {
          accumulator.entryLeaderCount++
        }
        if (leader?.state === 'exit' || leader?.state === 'avoid') {
          accumulator.defensiveLeaderCount++
        }

        return accumulator
      },
      {
        consensusCount: 0,
        constructiveCount: 0,
        entryLeaderCount: 0,
        defensiveLeaderCount: 0,
      },
    )

    return {
      ...summary,
      visibleCount: topMarkets.length,
    }
  }, [sortedMarkets])

  const selectedMarketComparison = useMemo(() => {
    if (!analysis) {
      return null
    }

    const sortedStrategies = sortStrategiesForComparison(analysis.strategies)
    const leader = sortedStrategies[0]
    const runnerUp = sortedStrategies[1]
    const activeCandidate = sortedStrategies.find((strategy) => strategy.state === 'entry' || strategy.state === 'hold')
    const stableProfile = signalHistory?.profiles.reduce((best, profile) => {
      if (!best || profile.stats.stabilityRate > best.stats.stabilityRate) {
        return profile
      }
      return best
    }, signalHistory.profiles[0])
    const fastestEntryProfile = signalHistory?.profiles.reduce((best, profile) => {
      if (!best || profile.stats.entryTransitionRate > best.stats.entryTransitionRate) {
        return profile
      }
      return best
    }, signalHistory.profiles[0])
    const convergence = getStrategyConvergence(analysis.strategies)

    return {
      leader,
      runnerUp,
      scoreGap: leader && runnerUp ? leader.score - runnerUp.score : 0,
      activeCandidate,
      stableProfile,
      fastestEntryProfile,
      convergence,
    }
  }, [analysis, signalHistory])

  const recentSignalAlerts = useMemo(() => {
    if (!signalHistory) {
      return [] as string[]
    }

    const alerts: string[] = []

    signalHistory.profiles.forEach((profile) => {
      const previous = profile.points[profile.points.length - 2]
      const latest = profile.points[profile.points.length - 1]
      if (!previous || !latest) {
        return
      }

      if (previous.state !== 'entry' && latest.state === 'entry') {
        alerts.push(`${translateProfileLabel(profile.label)} vient de passer en entree.`)
      }

      if ((previous.state === 'entry' || previous.state === 'hold') && latest.state === 'exit') {
        alerts.push(`${translateProfileLabel(profile.label)} vient de passer d une configuration active a une sortie.`)
      }

      if (previous.state === 'hold' && latest.state === 'watch') {
        alerts.push(`${translateProfileLabel(profile.label)} est passe de maintien a surveillance.`)
      }
    })

    if (analysis) {
      const convergence = getStrategyConvergence(analysis.strategies)
      if (convergence.hasConsensus) {
        alerts.unshift(`Consensus : ${convergence.activeCount} profils sont actuellement en entree ou en maintien.`)
      } else if (convergence.hasConstructiveAlignment) {
        alerts.unshift(`Alignement constructif : ${convergence.constructiveCount} profils sont au minimum en surveillance.`)
      }
    }

    return alerts
  }, [analysis, signalHistory])

  return (
    <section className="panel">
      {error && opportunitiesError && (
        <div className="degraded-banner">
          Connexion au backend impossible — les données affichées peuvent être obsolètes ou indisponibles.
        </div>
      )}
      <section className="scanner-panel">
        <div className="panel-head scanner-head">
          <div>
            <h2>Scanner d opportunites</h2>
            <p>
              Les opportunites priorisees par le backend combinent qualite, profil leader, convergence, fraicheur et momentum court terme pour faire ressortir les configurations les plus exploitables.
            </p>
          </div>
          <div className="panel-head-actions">
            <select
              aria-label="Filtrer le scanner par niveau de priorite"
              className="select-input"
              value={priorityBandFilter}
              onChange={(event) => setPriorityBandFilter(event.target.value)}
            >
              {priorityBands.map((band) => (
                <option key={band.value} value={band.value}>
                  {band.label}
                </option>
              ))}
            </select>
            <label className="toggle-chip">
              <input type="checkbox" checked={freshOnly} onChange={(event) => setFreshOnly(event.target.checked)} />
              <span>Entrees fraiches uniquement</span>
            </label>
            <label className="toggle-chip">
              <input type="checkbox" checked={consensusOnly} onChange={(event) => setConsensusOnly(event.target.checked)} />
              <span>Consensus uniquement</span>
            </label>
            <span className="muted-text">{opportunitiesLoading ? 'Actualisation du scanner...' : `${scannerOpportunities.length} opportunites`}</span>
          </div>
        </div>

        <div className="section-description compact-description">
          <p>
            Sante systeme : {systemHealthLoading
              ? 'actualisation en cours'
              : systemHealth
                ? (systemHealth.isHealthy ? 'etat sain' : 'etat degrade')
                : 'indisponible'}.
            {' '}
            {systemHealth
              ? `Univers ${systemHealth.universeSymbols} | Top ${systemHealth.topSymbols} | Live ${systemHealth.liveSymbols} | Backfill en cours ${systemHealth.backfillRunningJobs}`
              : 'Les indicateurs de fraicheur et de backfill seront affiches ici.'}
          </p>
          {systemHealth && systemHealth.issues.length > 0 && (
            <div className="analysis-tag-list">
              {systemHealth.issues.map((issue) => (
                <span key={issue} className="analysis-tag risk-tag">{translateHealthIssue(issue)}</span>
              ))}
            </div>
          )}
          {systemHealth && systemHealth.topStaleExamples.length > 0 && (
            <div className="analysis-tag-list">
              {systemHealth.topStaleExamples.map((item) => (
                <span key={item} className="analysis-tag">retard {item}</span>
              ))}
            </div>
          )}
          {systemHealthError && <p className="error-text">{systemHealthError}</p>}
        </div>

        <div className="section-description compact-description">
          <p>
            Alertes recentes : {recentAlertsLoading ? 'actualisation en cours' : `${recentAlerts.length} evenement(s)`}.
          </p>
          {recentAlertsError && <p className="error-text">{recentAlertsError}</p>}
          {recentAlerts.length > 0 && (
            <div className="analysis-tag-list">
              {recentAlerts.slice(0, 8).map((alert) => (
                <span key={alert.id} className={`analysis-tag ${alert.severity === 'critical' ? 'risk-tag' : alert.severity === 'warning' ? 'risk-tag' : ''}`}>
                  {translateAlertSeverity(alert.severity)} | {alert.active ? 'actif' : 'resolu'} | {alert.message} (x{alert.count})
                </span>
              ))}
            </div>
          )}
        </div>

        <div className="overview-grid scanner-overview-grid">
          <article className="overview-card">
            <span>Actionnables maintenant</span>
            <strong>{scannerOverview.actionableCount}</strong>
            <p>Configurations de plus haute priorite suffisamment solides pour une revue immediate.</p>
          </article>
          <article className="overview-card">
            <span>Entrees fraiches</span>
            <strong>{scannerOverview.freshEntryCount}</strong>
            <p>Opportunites ou au moins un profil vient de passer en entree sur la derniere bougie stockee.</p>
          </article>
          <article className="overview-card">
            <span>Configurations en consensus</span>
            <strong>{scannerOverview.consensusCount}</strong>
            <p>Opportunites ou plusieurs profils sont deja actifs en meme temps.</p>
          </article>
          <article className="overview-card">
            <span>Configurations defensives</span>
            <strong>{scannerOverview.defensiveCount}</strong>
            <p>Entrees ecartees par un contexte faible, des sorties recentes ou un leadership peu convaincant.</p>
          </article>
        </div>

        {opportunitiesError && <p className="error-text">{opportunitiesError}</p>}

        {scannerOpportunities.length > 0 ? (
          <div className="scanner-card-grid">
            {scannerOpportunities.map((opportunity) => (
              <article key={opportunity.symbol} className={`scanner-card priority-${opportunity.priorityBand}`}>
                <div className="scanner-card-head">
                  <div>
                    <div className="scanner-title-row">
                      <h3>{opportunity.symbol}</h3>
                      <span className={`priority-pill ${opportunity.priorityBand}`}>{translatePriorityBand(opportunity.priorityBand)}</span>
                      {opportunity.freshness.hasFreshEntry && <span className="fresh-badge entry">entree fraiche</span>}
                      {opportunity.freshness.hasFreshExit && <span className="fresh-badge exit">sortie fraiche</span>}
                    </div>
                    <p className="card-description">{translateOpportunitySummary(opportunity.summary)}</p>
                  </div>
                  <button type="button" className="action-button" onClick={() => setSelectedSymbol(opportunity.symbol)}>
                    Ouvrir
                  </button>
                </div>

                <div className="scanner-metric-grid">
                  <div>
                    <span>Score de priorite</span>
                    <strong>{formatNumber(opportunity.priorityScore, 1)}</strong>
                  </div>
                  <div>
                    <span>Action principale</span>
                    <strong>{translatePrimaryAction(opportunity.primaryAction)}</strong>
                  </div>
                  <div>
                    <span>Profil leader</span>
                    <strong>{translateProfileLabel(opportunity.leader.label || 'n/a')}</strong>
                  </div>
                  <div>
                    <span>Etat leader</span>
                    <strong className={opportunity.leader.state === 'entry' || opportunity.leader.state === 'hold' ? 'positive' : opportunity.leader.state === 'exit' || opportunity.leader.state === 'avoid' ? 'negative' : ''}>
                      {translateStateLabel(opportunity.leader.state || 'n/a')}
                    </strong>
                  </div>
                </div>

                <div className="analysis-tag-list">
                  <span className="analysis-tag">qualite : {formatNumber(opportunity.qualityScore, 1)}</span>
                  <span className="analysis-tag">1h: {formatSignedPercent(opportunity.change1h, 2)}</span>
                  <span className="analysis-tag">5m: {formatSignedPercent(opportunity.change5m, 2)}</span>
                  <span className="analysis-tag">profils actifs : {opportunity.convergence.activeProfiles}</span>
                  <span className="analysis-tag">profils constructifs : {opportunity.convergence.constructiveProfiles}</span>
                  <span className="analysis-tag">entree la plus recente : {formatBarsAgo(opportunity.freshness.youngestEntryBars)}</span>
                </div>

                <div className="scanner-foot-grid">
                  <div>
                    <p className="scanner-list-label">Raisons</p>
                    <div className="analysis-tag-list">
                      {opportunity.reasons.length > 0 ? opportunity.reasons.map((reason) => (
                        <span key={`${opportunity.symbol}-${reason}`} className="analysis-tag reason-tag">{translateReason(reason)}</span>
                      )) : <span className="analysis-tag">Aucun catalyseur explicite</span>}
                    </div>
                  </div>
                  <div>
                    <p className="scanner-list-label">Risques</p>
                    <div className="analysis-tag-list">
                      {opportunity.risks.length > 0 ? opportunity.risks.map((risk) => (
                        <span key={`${opportunity.symbol}-${risk}`} className="analysis-tag risk-tag">{translateRisk(risk)}</span>
                      )) : <span className="analysis-tag">Aucun risque majeur detecte</span>}
                    </div>
                  </div>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <p className="empty-text">Aucune opportunite ne correspond aux filtres actuels du scanner.</p>
        )}
      </section>

      <div className="panel-head">
        <div>
          <h2>Marches</h2>
          <p>Marches classes par qualite, momentum et etat strategique pour passer plus vite du signal brut a une lecture decisionnelle.</p>
        </div>
        <div className="panel-head-actions">
          <label className="inline-label" htmlFor="sort-markets">
            Trier par
          </label>
          <select
            id="sort-markets"
            className="select-input"
            value={sortKey}
            onChange={(event) => setSortKey(event.target.value as MarketSortKey)}
          >
            <option value="priority">Priorite backend</option>
            <option value="score">Score</option>
            <option value="strategyScore">Score strategie</option>
            <option value="convergence">Convergence</option>
            <option value="change24h">24h %</option>
            <option value="change1h">1h %</option>
            <option value="change5m">5m %</option>
            <option value="quoteVolume">Volume</option>
          </select>
          <select
            aria-label="Filtrer par profil de strategie"
            className="select-input"
            value={strategyProfileFilter}
            onChange={(event) => setStrategyProfileFilter(event.target.value)}
          >
            {strategyProfiles.map((profile) => (
              <option key={profile.value} value={profile.value}>
                {profile.label}
              </option>
            ))}
          </select>
          <select
            aria-label="Filtrer par etat de strategie"
            className="select-input"
            value={strategyStateFilter}
            onChange={(event) => setStrategyStateFilter(event.target.value)}
          >
            {strategyStates.map((state) => (
              <option key={state.value} value={state.value}>
                {state.label}
              </option>
            ))}
          </select>
          <span className="muted-text">{loading ? 'Actualisation...' : `${markets.length} marches`}</span>
        </div>
      </div>

      <div className="section-description">
        <p>
          Le filtre de profil permet de cibler un horizon de trading, puis le tri par score de strategie ou priorite backend met en avant les marches les plus pertinents.
        </p>
      </div>

      <div className="state-legend">
        <span className="state-pill strategy-state entry">entree : nouveau setup exploitable</span>
        <span className="state-pill strategy-state hold">maintien : tendance encore constructive</span>
        <span className="state-pill strategy-state watch">surveillance : interessant mais non confirme</span>
        <span className="state-pill strategy-state exit">sortie : deterioration ou invalidation du setup</span>
        <span className="state-pill strategy-state avoid">a eviter : contexte faible ou peu convaincant</span>
      </div>

      <div className="overview-grid">
        <article className="overview-card">
          <span>Marches en consensus</span>
          <strong>{marketOverview.consensusCount}</strong>
          <p>Marches ou au moins 2 profils sont deja en entree ou en maintien.</p>
        </article>
        <article className="overview-card">
          <span>Alignement constructif</span>
          <strong>{marketOverview.constructiveCount}</strong>
          <p>Marches ou au moins 2 profils sont en surveillance ou mieux, sans consensus actif complet.</p>
        </article>
        <article className="overview-card">
          <span>Marches menes par une entree</span>
          <strong>{marketOverview.entryLeaderCount}</strong>
          <p>Marches dont le profil leader signale deja une entree.</p>
        </article>
        <article className="overview-card">
          <span>Leaders defensifs</span>
          <strong>{marketOverview.defensiveLeaderCount}</strong>
          <p>Marches dont le profil le plus fort reste en evitement ou en sortie, ce qui invite a la prudence.</p>
        </article>
      </div>

      <div className="section-description compact-description">
        <p>
          Le resume ci-dessus suit les marches visibles apres filtres. Le tri par convergence ou priorite backend met en avant les configurations les plus alignees.
        </p>
      </div>

      {error && <p className="error-text">{error}</p>}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Symbole</th>
              <th>Score</th>
              <th>24h</th>
              <th>1h</th>
              <th>5m</th>
              <th>Volume</th>
              <th>Strategies</th>
            </tr>
          </thead>
          <tbody>
            {sortedMarkets.slice(0, 30).map((market) => (
              (() => {
                const convergence = getStrategyConvergence(market.strategies)
                const priority = opportunityBySymbol.get(market.symbol)

                return (
                  <tr
                    key={market.symbol}
                    className={[
                      'market-row',
                      market.symbol === selectedSymbol ? 'active' : '',
                      convergence.hasConsensus ? 'convergence-strong' : '',
                      !convergence.hasConsensus && convergence.hasConstructiveAlignment ? 'convergence-watch' : '',
                    ].filter(Boolean).join(' ')}
                    onClick={() => setSelectedSymbol(market.symbol)}
                  >
                <td>{market.symbol}</td>
                <td>
                  {formatNumber(market.qualityScore, 1)}
                  <span className="score-details">
                    RSI {formatNumber(market.qualityRsi, 0)} | MACD {formatNumber(market.qualityMacd, 0)} | BB {formatNumber(market.qualityBollinger, 0)}
                    {strategyProfileFilter !== 'all' && (
                      <>
                        {' '}| {selectedProfileLabel} {formatNumber(market.strategies.find((strategy) => strategy.name === strategyProfileFilter)?.score ?? 0, 1)}
                      </>
                    )}
                    {priority && (
                      <>
                        {' '}| Priorite {formatNumber(priority.priorityScore, 1)}
                      </>
                    )}
                  </span>
                </td>
                <td className={market.change24h >= 0 ? 'positive' : 'negative'}>
                  {formatSignedPercent(market.change24h)}
                </td>
                <td className={market.change1h >= 0 ? 'positive' : 'negative'}>
                  {formatSignedPercent(market.change1h)}
                </td>
                <td className={market.change5m >= 0 ? 'positive' : 'negative'}>
                  {formatSignedPercent(market.change5m)}
                </td>
                <td>{formatNumber(market.quoteVolume, 2)}</td>
                <td>
                  {(() => {
                    const rankedStrategies = sortStrategiesForComparison(market.strategies)
                    const leader = rankedStrategies[0]
                    const runnerUp = rankedStrategies[1]

                    if (!leader) {
                      return null
                    }

                    return (
                      <p className="strategy-row-summary">
                        Leader {translateProfileLabel(leader.label)} en <strong>{translateStateLabel(leader.state)}</strong>
                        {runnerUp ? ` | ecart ${formatNumber(leader.score - runnerUp.score, 1)}` : ''}
                        {priority ? ` | ${translatePriorityBand(priority.priorityBand)}` : ''}
                        {priority?.freshness.hasFreshEntry ? ' | entree fraiche' : ''}
                        {convergence.hasConsensus ? ` | consensus ${convergence.activeCount}/3 actifs` : ''}
                        {!convergence.hasConsensus && convergence.hasConstructiveAlignment ? ` | ${convergence.constructiveCount}/3 constructifs` : ''}
                      </p>
                    )
                  })()}
                  <div className="strategy-inline-list">
                    {market.strategies.map((strategy) => (
                      <span key={`${market.symbol}-${strategy.name}`} className={`state-pill strategy-state ${strategy.state}`}>
                        {translateProfileLabel(strategy.label)} : {translateStateLabel(strategy.state)}
                      </span>
                    ))}
                  </div>
                </td>
                  </tr>
                )
              })()
            ))}
          </tbody>
        </table>
      </div>

      <section className="market-analysis-panel">
        <div className="market-analysis-head">
          <div>
            <h3>{selectedSymbol || 'Analyse de marche'}</h3>
            <p>Vue strategique detaillee du marche selectionne, avec score en direct et historique recent pour comprendre si un signal est frais, stable ou en deterioration.</p>
          </div>
          <span className="muted-text">
            {analysisLoading ? 'Actualisation de l analyse...' : analysis ? `${analysis.candleCount1m} bougies en direct` : 'Aucun marche selectionne'}
          </span>
        </div>

        {analysisError && <p className="error-text">{analysisError}</p>}

        {analysis ? (
          <>
            <div className="kpi-grid market-analysis-kpis">
              <article className="kpi-card">
                <span>Score de qualite</span>
                <h3>{formatNumber(analysis.qualityScore, 2)}</h3>
              </article>
              <article className="kpi-card">
                <span>Variation 24h</span>
                <h3 className={analysis.change24h >= 0 ? 'positive' : 'negative'}>{formatSignedPercent(analysis.change24h)}</h3>
              </article>
              <article className="kpi-card">
                <span>Variation 1h</span>
                <h3 className={analysis.change1h >= 0 ? 'positive' : 'negative'}>{formatSignedPercent(analysis.change1h)}</h3>
              </article>
              <article className="kpi-card">
                <span>Variation 5m</span>
                <h3 className={analysis.change5m >= 0 ? 'positive' : 'negative'}>{formatSignedPercent(analysis.change5m)}</h3>
              </article>
            </div>

            <div className="market-analysis-grid">
              <article className="analysis-card">
                <h4>Detail des indicateurs</h4>
                <p className="card-description">Ces scores bruts expliquent pourquoi le score de qualite agrege est eleve ou faible.</p>
                <div className="analysis-metric-list">
                  <span>RSI {formatNumber(analysis.qualityRsi, 2)}</span>
                  <span>MACD {formatNumber(analysis.qualityMacd, 2)}</span>
                  <span>Bollinger {formatNumber(analysis.qualityBollinger, 2)}</span>
                  <span>Volume {formatNumber(analysis.qualityVolume, 2)}</span>
                  <span>SMA {formatNumber(analysis.qualitySma, 2)}</span>
                </div>
              </article>

              <article className="analysis-card">
                <h4>Comparaison des profils</h4>
                <p className="card-description">
                  Cette carte compare les trois profils de strategie du marche selectionne pour voir rapidement quel horizon est le plus fort, le plus stable et le plus reactif.
                </p>
                {selectedMarketComparison?.leader ? (
                  <div className="comparison-grid">
                    <article className="comparison-metric-card">
                      <span>Leader actuel</span>
                      <strong>{translateProfileLabel(selectedMarketComparison.leader.label)}</strong>
                      <p>
                        Etat {translateStateLabel(selectedMarketComparison.leader.state)}, score {formatNumber(selectedMarketComparison.leader.score, 2)}
                        {selectedMarketComparison.runnerUp ? `, ecart ${formatNumber(selectedMarketComparison.scoreGap, 2)} vs ${translateProfileLabel(selectedMarketComparison.runnerUp.label)}` : ''}
                      </p>
                    </article>
                    <article className="comparison-metric-card">
                      <span>Meilleur setup actif</span>
                      <strong>{selectedMarketComparison.activeCandidate ? translateProfileLabel(selectedMarketComparison.activeCandidate.label) : 'Aucun profil actif'}</strong>
                      <p>
                        {selectedMarketComparison.activeCandidate
                          ? `${translateStateLabel(selectedMarketComparison.activeCandidate.state)} avec un score de ${formatNumber(selectedMarketComparison.activeCandidate.score, 2)}`
                          : 'Aucun profil n est actuellement en entree ou en maintien.'}
                      </p>
                    </article>
                    <article className="comparison-metric-card">
                      <span>Profil le plus stable</span>
                      <strong>{selectedMarketComparison.stableProfile ? translateProfileLabel(selectedMarketComparison.stableProfile.label) : 'Pas encore d historique'}</strong>
                      <p>
                        {selectedMarketComparison.stableProfile
                          ? `${formatNumber(selectedMarketComparison.stableProfile.stats.stabilityRate, 1)}% de stabilite d etat sur les recentes bougies ${historyTimeframeLabel}.`
                          : 'Historique stocke insuffisant pour comparer la stabilite.'}
                      </p>
                    </article>
                    <article className="comparison-metric-card">
                      <span>Convergence des profils</span>
                      <strong>
                        {selectedMarketComparison.convergence.hasConsensus
                          ? 'Alignement a forte conviction'
                          : selectedMarketComparison.convergence.hasConstructiveAlignment
                            ? 'Constructif mais mitige'
                            : 'Faible accord'}
                      </strong>
                      <p>
                        {selectedMarketComparison.convergence.hasConsensus
                          ? `${selectedMarketComparison.convergence.activeCount} profils sont deja en entree ou en maintien.`
                          : selectedMarketComparison.convergence.hasConstructiveAlignment
                            ? `${selectedMarketComparison.convergence.constructiveCount} profils sont au minimum en surveillance, mais moins de 2 sont actifs.`
                            : 'La plupart des profils restent faibles, defensifs ou divergents.'}
                      </p>
                    </article>
                    <article className="comparison-metric-card">
                      <span>Le plus rapide vers l entree</span>
                      <strong>{selectedMarketComparison.fastestEntryProfile ? translateProfileLabel(selectedMarketComparison.fastestEntryProfile.label) : 'Pas encore d historique'}</strong>
                      <p>
                        {selectedMarketComparison.fastestEntryProfile
                          ? `${formatNumber(selectedMarketComparison.fastestEntryProfile.stats.entryTransitionRate, 1)}% des changements d etat ont bascule vers une entree.`
                          : 'Historique stocke insuffisant pour comparer la vitesse d entree.'}
                      </p>
                    </article>
                  </div>
                ) : (
                  <p className="empty-text">Aucune comparaison de strategie n est disponible pour ce marche pour le moment.</p>
                )}
              </article>

              <article className="analysis-card">
                <h4>Alertes de signal</h4>
                <p className="card-description">
                  Les transitions recentes aident a voir si le setup se renforce, perd du momentum ou s aligne sur plusieurs profils.
                </p>
                {recentSignalAlerts.length > 0 ? (
                  <div className="alert-list">
                    {recentSignalAlerts.map((alert) => (
                      <p key={alert} className="signal-alert">{alert}</p>
                    ))}
                  </div>
                ) : (
                  <p className="empty-text">Aucune transition notable n a ete detectee dans les dernieres bougies stockees.</p>
                )}
              </article>

              <article className="analysis-card">
                <h4>Etats strategiques</h4>
                <p className="card-description">Chaque profil transforme les memes indicateurs en un etat de decision adapte a son horizon de trading.</p>
                <div className="strategy-detail-list">
                  {analysis.strategies.map((strategy) => (
                    <div key={strategy.name} className="strategy-detail-item">
                      <div className="strategy-detail-head">
                        <strong>{translateProfileLabel(strategy.label)}</strong>
                        <span className={`state-pill strategy-state ${strategy.state}`}>{translateStateLabel(strategy.state)}</span>
                      </div>
                      <p className="muted-text">Score {formatNumber(strategy.score, 2)}</p>
                      <div className="analysis-tag-list">
                        {strategy.reasons.map((reason) => (
                          <span key={`${strategy.name}-${reason}`} className="analysis-tag reason-tag">{translateReason(reason)}</span>
                        ))}
                        {strategy.risks.map((risk) => (
                          <span key={`${strategy.name}-${risk}`} className="analysis-tag risk-tag">{translateRisk(risk)}</span>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </article>

              <article className="analysis-card">
                <div className="market-analysis-head compact">
                  <div>
                    <h4>Historique des signaux</h4>
                    <p>Etats strategiques recents issus des bougies 5m stockees. Cela aide a distinguer un signal tout juste apparu d un profil constructif depuis plusieurs bougies.</p>
                  </div>
                  <span className="muted-text">
                    {signalHistoryLoading ? 'Actualisation de l historique...' : signalHistory ? `${signalHistory.candleCount} bougies` : 'Aucun historique'}
                  </span>
                </div>
                {signalHistoryError && <p className="error-text">{signalHistoryError}</p>}
                {signalHistory ? (
                  <div className="signal-history-list">
                    <div className="signal-history-summary">
                      {signalHistory.profiles.map((profileStat) => (
                        <article key={profileStat.name} className="signal-summary-card">
                          <div className="signal-summary-head">
                            <h5>{translateProfileLabel(profileStat.label)}</h5>
                            {profileStat.stats.justEntered && <span className="fresh-badge entry">entree fraiche</span>}
                            {profileStat.stats.justExited && <span className="fresh-badge exit">sortie fraiche</span>}
                            {!profileStat.stats.justEntered && !profileStat.stats.justExited && profileStat.stats.justChanged && (
                              <span className="fresh-badge neutral">nouvel etat</span>
                            )}
                          </div>
                          <p className="card-description">
                            Dernier etat {translateStateLabel(profileStat.stats.latestState)}, score moyen {formatNumber(profileStat.stats.averageScore, 2)} sur la fenetre recente.
                            Les transitions vers l entree et la duree des phases de maintien sont calculees par le backend a partir des bougies {historyTimeframeLabel} stockees.
                          </p>
                          <div className="signal-summary-metrics">
                            <span>
                              Taux de transition vers entree <strong>{formatNumber(profileStat.stats.entryTransitionRate, 1)}%</strong>
                            </span>
                            <span>
                              Duree moyenne de maintien <strong>{formatNumber(profileStat.stats.averageHoldBars, 1)} bougies</strong>
                            </span>
                            <span>
                              Sortie apres entree <strong>{formatNumber(profileStat.stats.exitAfterEntryRate, 1)}%</strong>
                            </span>
                            <span>
                              Stabilite de l etat <strong>{formatNumber(profileStat.stats.stabilityRate, 1)}%</strong>
                            </span>
                            <span>
                              Age de l etat courant <strong>{formatNumber(profileStat.stats.latestStateAgeBars, 0)} bougies</strong>
                            </span>
                          </div>
                          <div className="analysis-tag-list">
                            <span className={`analysis-tag state-count-tag ${profileStat.stats.latestState}`}>dernier : {translateStateLabel(profileStat.stats.latestState)}</span>
                            <span className="analysis-tag">entree : {profileStat.stats.entryCount}</span>
                            <span className="analysis-tag">maintien : {profileStat.stats.holdCount}</span>
                            <span className="analysis-tag">surveillance : {profileStat.stats.watchCount}</span>
                            <span className="analysis-tag">sortie : {profileStat.stats.exitCount}</span>
                            <span className="analysis-tag">a eviter : {profileStat.stats.avoidCount}</span>
                            <span className="analysis-tag">trades resolus : {profileStat.stats.resolvedTradeCount}</span>
                            <span className="analysis-tag">derniere entree : {formatBarsAgo(profileStat.stats.barsSinceEntry)}</span>
                            <span className="analysis-tag">derniere sortie : {formatBarsAgo(profileStat.stats.barsSinceExit)}</span>
                          </div>
                        </article>
                      ))}
                    </div>
                    {signalHistory.profiles.map((profile) => {
                      const latest = profile.points[profile.points.length - 1]
                      const stateCounts = profile.points.reduce<Record<string, number>>((counts, point) => {
                        counts[point.state] = (counts[point.state] ?? 0) + 1
                        return counts
                      }, {})

                      return (
                        <div key={profile.name} className="strategy-detail-item">
                          <div className="strategy-detail-head">
                            <div className="strategy-detail-title">
                              <strong>{translateProfileLabel(profile.label)}</strong>
                              {profile.stats.justEntered && <span className="fresh-badge entry">entree fraiche</span>}
                              {profile.stats.justExited && <span className="fresh-badge exit">sortie fraiche</span>}
                              {!profile.stats.justEntered && !profile.stats.justExited && profile.stats.justChanged && (
                                <span className="fresh-badge neutral">nouvel etat</span>
                              )}
                            </div>
                            {latest && <span className={`state-pill strategy-state ${latest.state}`}>{translateStateLabel(latest.state)}</span>}
                          </div>
                          <p className="card-description">
                            Chronologie des etats recents pour {translateProfileLabel(profile.label)}. Un badge represente une bougie {historyTimeframeLabel} stockee. Le survol d un badge affiche l horodatage et le score.
                          </p>
                          <div className="analysis-tag-list">
                            <span className="analysis-tag">age de l etat : {profile.stats.latestStateAgeBars} bougies</span>
                            <span className="analysis-tag">derniere entree : {formatBarsAgo(profile.stats.barsSinceEntry)}</span>
                            <span className="analysis-tag">derniere sortie : {formatBarsAgo(profile.stats.barsSinceExit)}</span>
                            {Object.entries(stateCounts).map(([state, count]) => (
                              <span key={`${profile.name}-${state}`} className={`analysis-tag state-count-tag ${state}`}>
                                {translateStateLabel(state)} : {count}
                              </span>
                            ))}
                          </div>
                          <div className="signal-point-list">
                            {profile.points.map((point) => (
                              <span
                                key={`${profile.name}-${point.closeTime}`}
                                className={`state-pill strategy-state ${point.state}`}
                                title={`${new Date(point.closeTime).toLocaleString()} | Score ${formatNumber(point.score, 2)}`}
                              >
                                {translateStateLabel(point.state)}
                              </span>
                            ))}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                ) : (
                  <p className="empty-text">Aucun historique de signaux n est disponible pour le marche selectionne.</p>
                )}
              </article>
            </div>
          </>
        ) : (
          <p className="empty-text">Selectionner un marche pour charger l analyse strategique detaillee.</p>
        )}
      </section>
    </section>
  )
}
