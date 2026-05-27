import { useMemo } from 'react'
import { type Market, type MarketSortKey, type Opportunity } from '../models/market'
import { formatNumber, formatSignedPercent } from '../utils/format'
import { formatProfileDisplay, translatePriorityBand, translateProfileLabel, translateStateLabel } from '../utils/market-translations'

const strategyProfiles = [
  { value: 'all', label: 'Tous les profils' },
  { value: 'intraday-momentum', label: 'Pulse' },
  { value: 'swing-balance', label: 'Balance' },
  { value: 'trend-follow', label: 'Trend' },
]

const strategyStates = [
  { value: 'all', label: 'Tous les etats' },
  { value: 'entry', label: 'Entree' },
  { value: 'hold', label: 'Maintien' },
  { value: 'watch', label: 'Surveillance' },
  { value: 'exit', label: 'Sortie' },
  { value: 'avoid', label: 'A eviter' },
]

const strategyStatePriority: Record<string, number> = {
  entry: 5,
  hold: 4,
  watch: 3,
  avoid: 2,
  exit: 1,
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

function sortStrategiesForComparison(strategies: Market['strategies']): Market['strategies'] {
  return [...strategies].sort((left, right) => {
    if (right.score !== left.score) {
      return right.score - left.score
    }
    return (strategyStatePriority[right.state] ?? 0) - (strategyStatePriority[left.state] ?? 0)
  })
}

function getConvergenceSortScore(strategies: Market['strategies']): number {
  const convergence = getStrategyConvergence(strategies)
  const rankedStrategies = sortStrategiesForComparison(strategies)
  const topScore = rankedStrategies[0]?.score ?? 0
  return convergence.activeCount * 1000 + convergence.constructiveCount * 100 + topScore
}

function getStrategyScoreForSort(strategies: Market['strategies'], profileName: string): number {
  if (profileName === 'all') {
    return strategies.reduce((best, strategy) => Math.max(best, strategy.score), -1)
  }
  return strategies.find((strategy) => strategy.name === profileName)?.score ?? -1
}

type MarketsTableSectionProps = {
  markets: Market[]
  opportunities: Opportunity[]
  loading: boolean
  error: string
  sortKey: MarketSortKey
  strategyProfileFilter: string
  strategyStateFilter: string
  selectedSymbol: string
  onSortKeyChange: (key: MarketSortKey) => void
  onStrategyProfileFilterChange: (value: string) => void
  onStrategyStateFilterChange: (value: string) => void
  onSelectSymbol: (symbol: string) => void
}

export function MarketsTableSection({
  markets,
  opportunities,
  loading,
  error,
  sortKey,
  strategyProfileFilter,
  strategyStateFilter,
  selectedSymbol,
  onSortKeyChange,
  onStrategyProfileFilterChange,
  onStrategyStateFilterChange,
  onSelectSymbol,
}: MarketsTableSectionProps) {
  const opportunityBySymbol = useMemo(() => {
    return new Map(opportunities.map((opportunity) => [opportunity.symbol, opportunity]))
  }, [opportunities])

  const selectedProfileLabel = useMemo(() => {
    return strategyProfiles.find((profile) => profile.value === strategyProfileFilter)?.label ?? 'profil selectionne'
  }, [strategyProfileFilter])

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
      { consensusCount: 0, constructiveCount: 0, entryLeaderCount: 0, defensiveLeaderCount: 0 },
    )
    return { ...summary, visibleCount: topMarkets.length }
  }, [sortedMarkets])

  return (
    <>
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
            onChange={(event) => onSortKeyChange(event.target.value as MarketSortKey)}
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
            onChange={(event) => onStrategyProfileFilterChange(event.target.value)}
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
            onChange={(event) => onStrategyStateFilterChange(event.target.value)}
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
            {sortedMarkets.slice(0, 30).map((market) => {
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
                  onClick={() => onSelectSymbol(market.symbol)}
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
                          Leader <span style={{ color: leader.color || undefined }}>{formatProfileDisplay(leader.label, leader.icon)}</span> en <strong>{translateStateLabel(leader.state)}</strong>
                          {runnerUp ? ` | ecart ${formatNumber(leader.score - runnerUp.score, 1)}` : ''}
                          {priority ? ` | ${translatePriorityBand(priority.priorityBand)}` : ''}
                          {priority?.freshness.hasFreshEntry ? ' | entree fraiche' : ''}
                          {convergence.hasConsensus ? ` | consensus ${convergence.activeCount}/3 actifs` : ''}
                          {!convergence.hasConsensus && convergence.hasConstructiveAlignment ? ` | ${convergence.constructiveCount}/3 constructifs` : ''}
                          {leader.description ? ` | ${leader.description}` : ''}
                        </p>
                      )
                    })()}
                    <div className="strategy-inline-list">
                      {market.strategies.map((strategy) => (
                        <span key={`${market.symbol}-${strategy.name}`} className={`state-pill strategy-state ${strategy.state}`} style={{ borderColor: strategy.color || undefined }}>
                          {formatProfileDisplay(translateProfileLabel(strategy.label), strategy.icon)} : {translateStateLabel(strategy.state)}
                        </span>
                      ))}
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </>
  )
}
