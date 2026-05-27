import { useCallback, useMemo, useState } from 'react'
import { usePolling } from '../hooks/usePolling'
import { emptyWallet } from '../models/wallet'
import type { Opportunity } from '../models/market'
import { getOpportunities } from '../services/marketService'
import { getWallet } from '../services/walletService'
import { formatNumber, formatSignedPercent } from '../utils/format'
import { translatePriorityBand, translateStateLabel } from '../utils/market-translations'

type WalletSortKey = 'value' | 'pnl' | 'score' | 'change24h' | 'change1h'

type AssetMetrics = {
  effectiveValue: number
  effectiveCostBasis: number
  pnlValue: number
  pnlPercent: number
}

function getChangeClassName(change: number | undefined): string {
  if (change === undefined) return 'col-hidden-mobile'
  return `col-hidden-mobile ${change < 0 ? 'negative' : 'positive'}`
}

export function WalletPanel() {
  const [wallet, setWallet] = useState(emptyWallet)
  const [opportunities, setOpportunities] = useState<Opportunity[]>([])
  const [loading, setLoading] = useState(false)
  const [opportunitiesLoading, setOpportunitiesLoading] = useState(false)
  const [error, setError] = useState('')
  const [opportunitiesError, setOpportunitiesError] = useState('')
  const [includeStakingInPnl, setIncludeStakingInPnl] = useState(true)
  const [walletSortKey, setWalletSortKey] = useState<WalletSortKey>('value')

  const loadWallet = useCallback(async () => {
    setLoading(true)
    try {
      const nextWallet = await getWallet()
      setWallet(nextWallet)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'echec du chargement du portefeuille')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(loadWallet, 10_000)

  const loadOpportunities = useCallback(async () => {
    setOpportunitiesLoading(true)
    try {
      const nextOpportunities = await getOpportunities(40)
      setOpportunities(nextOpportunities)
      setOpportunitiesError('')
    } catch (err) {
      setOpportunitiesError(err instanceof Error ? err.message : 'echec du chargement des opportunites')
    } finally {
      setOpportunitiesLoading(false)
    }
  }, [])

  usePolling(loadOpportunities, 60_000)

  const visibleAssets = useMemo(
    () => wallet.assets.filter((asset) => asset.value >= 0.01),
    [wallet.assets],
  )

  const pnlMetrics = useMemo(() => {
    const bySymbol = new Map<string, AssetMetrics>()
    let effectiveTotalValue = 0

    for (const asset of wallet.assets) {
      const totalQty = asset.amount + asset.inOrder + asset.stakingAmount
      const effectiveQty = includeStakingInPnl ? totalQty : asset.amount + asset.inOrder

      let effectiveValue = asset.value
      let effectiveCostBasis = asset.costBasisValue

      if (totalQty > 0) {
        const ratio = Math.max(0, Math.min(1, effectiveQty / totalQty))
        effectiveValue = asset.value * ratio
        effectiveCostBasis = asset.costBasisValue * ratio
      }

      const pnlValue = effectiveValue - effectiveCostBasis
      const pnlPercent = effectiveCostBasis > 0 ? (pnlValue / effectiveCostBasis) * 100 : 0
      bySymbol.set(asset.symbol.toUpperCase(), {
        effectiveValue,
        effectiveCostBasis,
        pnlValue,
        pnlPercent,
      })

      effectiveTotalValue += effectiveValue
    }

    const globalPnlValue = effectiveTotalValue - wallet.netDepositsValue
    const globalPnlPercent =
      wallet.netDepositsValue > 0 ? (globalPnlValue / wallet.netDepositsValue) * 100 : 0

    return {
      bySymbol,
      effectiveTotalValue,
      globalPnlValue,
      globalPnlPercent,
    }
  }, [wallet.assets, wallet.netDepositsValue, includeStakingInPnl])

  const heldOpportunityMap = useMemo(() => {
    const bySymbol = new Map<string, Opportunity>()
    for (const opportunity of opportunities) {
      bySymbol.set(opportunity.symbol.toUpperCase(), opportunity)
    }
    return bySymbol
  }, [opportunities])

  const walletDecisions = useMemo(() => {
    const reduce: Array<{ symbol: string; value: number; reason: string }> = []
    const watch: Array<{ symbol: string; value: number; reason: string }> = []
    const reinforce: Array<{ symbol: string; value: number; reason: string }> = []

    for (const asset of visibleAssets) {
      const symbol = asset.symbol.toUpperCase()
      const opportunity = heldOpportunityMap.get(symbol)
      if (!opportunity) {
        continue
      }

      const isDefensive =
        opportunity.priorityBand === 'defensive' ||
        opportunity.leader.state === 'exit' ||
        opportunity.leader.state === 'avoid' ||
        opportunity.freshness.hasFreshExit

      const isActionable =
        opportunity.priorityBand === 'actionable' &&
        (opportunity.leader.state === 'entry' || opportunity.leader.state === 'hold')

      if (isDefensive) {
        reduce.push({
          symbol,
          value: asset.value,
          reason: `profil leader ${opportunity.leader.state}, priorite ${opportunity.priorityBand}`,
        })
        continue
      }

      if (isActionable && opportunity.freshness.hasFreshEntry) {
        reinforce.push({
          symbol,
          value: asset.value,
          reason: 'entree fraiche et priorite actionnable',
        })
        continue
      }

      watch.push({
        symbol,
        value: asset.value,
        reason: `etat ${opportunity.leader.state}, priorite ${opportunity.priorityBand}`,
      })
    }

    const sortByValueDesc = (left: { value: number }, right: { value: number }) => right.value - left.value
    reduce.sort(sortByValueDesc)
    watch.sort(sortByValueDesc)
    reinforce.sort(sortByValueDesc)

    return {
      reduce,
      watch,
      reinforce,
      sellNowCount: reduce.length,
      hasSellSignal: reduce.length > 0,
    }
  }, [heldOpportunityMap, visibleAssets])

  const decisionMap = useMemo(() => {
    const map = new Map<string, 'reduce' | 'watch' | 'reinforce'>()
    const entries: Array<['reduce' | 'watch' | 'reinforce', typeof walletDecisions.reduce]> = [
      ['reduce', walletDecisions.reduce],
      ['watch', walletDecisions.watch],
      ['reinforce', walletDecisions.reinforce],
    ]
    for (const [decision, items] of entries) {
      for (const item of items) {
        map.set(item.symbol, decision)
      }
    }
    return map
  }, [walletDecisions])

  const sortedVisibleAssets = useMemo(() => {
    return [...visibleAssets].sort((a, b) => {
      const aMetrics = pnlMetrics.bySymbol.get(a.symbol.toUpperCase())
      const bMetrics = pnlMetrics.bySymbol.get(b.symbol.toUpperCase())
      const aOpp = heldOpportunityMap.get(a.symbol.toUpperCase())
      const bOpp = heldOpportunityMap.get(b.symbol.toUpperCase())

      switch (walletSortKey) {
        case 'pnl':
          return (bMetrics?.pnlPercent ?? b.pnlPercent) - (aMetrics?.pnlPercent ?? a.pnlPercent)
        case 'score':
          return (bOpp?.qualityScore ?? -1) - (aOpp?.qualityScore ?? -1)
        case 'change24h':
          return (bOpp?.change24h ?? 0) - (aOpp?.change24h ?? 0)
        case 'change1h':
          return (bOpp?.change1h ?? 0) - (aOpp?.change1h ?? 0)
        case 'value':
        default:
          return b.value - a.value
      }
    })
  }, [visibleAssets, walletSortKey, pnlMetrics.bySymbol, heldOpportunityMap])

  return (
    <section className="panel">
      <div className="panel-head">
        <div>
          <h2>Portefeuille</h2>
          <p>Vue synthetique du portefeuille, des soldes et de la performance.</p>
        </div>
        <div className="panel-head-actions">
          <label className="inline-label" htmlFor="sort-wallet">Trier par</label>
          <select
            id="sort-wallet"
            className="select-input"
            value={walletSortKey}
            onChange={(event) => setWalletSortKey(event.target.value as WalletSortKey)}
          >
            <option value="value">Valeur</option>
            <option value="pnl">PnL %</option>
            <option value="score">Score</option>
            <option value="change24h">24h %</option>
            <option value="change1h">1h %</option>
          </select>
          <button
            type="button"
            className="action-button"
            onClick={() => setIncludeStakingInPnl((previous) => !previous)}
          >
            {includeStakingInPnl ? 'PnL inclut le staking' : 'PnL ignore le staking'}
          </button>
          <span className="muted-text">{loading ? 'Actualisation...' : 'En direct'}</span>
        </div>
      </div>

      {error && <p className="error-text">{error}</p>}
      {opportunitiesError && <p className="error-text">{opportunitiesError}</p>}

      <div className="kpi-grid">
        <article className="kpi-card">
          <span>Valeur totale</span>
          <h3>{formatNumber(wallet.totalValue, 2)} EUR</h3>
        </article>
        <article className="kpi-card">
          <span>Cash</span>
          <h3>{formatNumber(wallet.cashValue, 2)} EUR</h3>
        </article>
        <article className="kpi-card">
          <span>Actifs</span>
          <h3>{formatNumber(wallet.assetValue, 2)} EUR</h3>
        </article>
        <article className="kpi-card">
          <span>Depots nets</span>
          <h3>{formatNumber(wallet.netDepositsValue, 2)} EUR</h3>
        </article>
        <article className="kpi-card">
          <span>PnL global</span>
          <h3 className={pnlMetrics.globalPnlValue >= 0 ? 'positive' : 'negative'}>
            {pnlMetrics.globalPnlValue >= 0 ? '+' : ''}
            {formatNumber(pnlMetrics.globalPnlValue, 2)} EUR ({formatSignedPercent(pnlMetrics.globalPnlPercent)})
          </h3>
        </article>
      </div>

      <section className="analysis-card" style={{ marginBottom: '14px' }}>
        <h4>Decision portefeuille</h4>
        <p className="card-description">
          Reponse operationnelle a "Faut-il vendre quelque chose ?" basee sur les opportunites etats/fraicheur du backend. Ce module aide a prioriser, sans constituer un conseil financier.
        </p>
        <div className="analysis-tag-list" style={{ marginBottom: '10px' }}>
          <span className={`analysis-tag ${walletDecisions.hasSellSignal ? 'risk-tag' : 'reason-tag'}`}>
            Vente immediate: {walletDecisions.hasSellSignal ? 'oui, verification recommandee' : 'non, aucun signal defensif fort'}
          </span>
          <span className="analysis-tag">A alleger: {walletDecisions.reduce.length}</span>
          <span className="analysis-tag">A surveiller: {walletDecisions.watch.length}</span>
          <span className="analysis-tag">A renforcer: {walletDecisions.reinforce.length}</span>
          <span className="analysis-tag">Scanner: {opportunitiesLoading ? 'actualisation...' : `${opportunities.length} opportunites`}</span>
        </div>

        {walletDecisions.reduce.length > 0 && (
          <div style={{ marginBottom: '10px' }}>
            <p className="scanner-list-label">Positions a alleger en priorite</p>
            <div className="analysis-tag-list">
              {walletDecisions.reduce.slice(0, 6).map((item) => (
                <span key={`reduce-${item.symbol}`} className="analysis-tag risk-tag">
                  {item.symbol} ({formatNumber(item.value, 2)} EUR): {item.reason}
                </span>
              ))}
            </div>
          </div>
        )}

        {walletDecisions.watch.length > 0 && (
          <div style={{ marginBottom: '10px' }}>
            <p className="scanner-list-label">Positions a surveiller</p>
            <div className="analysis-tag-list">
              {walletDecisions.watch.slice(0, 6).map((item) => (
                <span key={`watch-${item.symbol}`} className="analysis-tag">
                  {item.symbol} ({formatNumber(item.value, 2)} EUR): {item.reason}
                </span>
              ))}
            </div>
          </div>
        )}

        {walletDecisions.reinforce.length > 0 && (
          <div>
            <p className="scanner-list-label">Positions potentiellement a renforcer</p>
            <div className="analysis-tag-list">
              {walletDecisions.reinforce.slice(0, 6).map((item) => (
                <span key={`reinforce-${item.symbol}`} className="analysis-tag reason-tag">
                  {item.symbol} ({formatNumber(item.value, 2)} EUR): {item.reason}
                </span>
              ))}
            </div>
          </div>
        )}
      </section>

      {visibleAssets.length === 0 ? (
        <p className="empty-text">Aucune donnee portefeuille pour le moment.</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Symbole</th>
                <th>Quantite</th>
                <th>En ordre</th>
                <th>Staking</th>
                <th>PRU</th>
                <th>PnL</th>
                <th>Valeur</th>
                <th className="col-hidden-mobile">Score</th>
                <th className="col-hidden-mobile">24h</th>
                <th className="col-hidden-mobile">1h</th>
                <th className="col-hidden-mobile">Signal</th>
              </tr>
            </thead>
            <tbody>
              {sortedVisibleAssets.map((asset) => {
                const metrics = pnlMetrics.bySymbol.get(asset.symbol.toUpperCase())
                const pnlValue = metrics?.pnlValue ?? asset.pnlValue
                const pnlPercent = metrics?.pnlPercent ?? asset.pnlPercent
                const opportunity = heldOpportunityMap.get(asset.symbol.toUpperCase())
                const decision = decisionMap.get(asset.symbol.toUpperCase())

                return (
                  <tr
                    key={asset.symbol}
                    className={decision ? `wallet-row wallet-${decision}` : 'wallet-row'}
                  >
                    <td>{asset.symbol}</td>
                    <td>{formatNumber(asset.amount, 8)}</td>
                    <td>{formatNumber(asset.inOrder, 8)}</td>
                    <td>{formatNumber(asset.stakingAmount, 8)}</td>
                    <td>{formatNumber(asset.costBasisValue, 2)} EUR</td>
                    <td className={pnlValue >= 0 ? 'positive' : 'negative'}>
                      {pnlValue >= 0 ? '+' : ''}
                      {formatNumber(pnlValue, 2)} EUR ({formatSignedPercent(pnlPercent)})
                    </td>
                    <td>{formatNumber(asset.value, 2)} EUR</td>
                    <td className="col-hidden-mobile">
                      {opportunity ? formatNumber(opportunity.qualityScore, 1) : <span className="muted-text">-</span>}
                    </td>
                    <td className={getChangeClassName(opportunity?.change24h)}>
                      {opportunity ? formatSignedPercent(opportunity.change24h) : <span className="muted-text">-</span>}
                    </td>
                    <td className={getChangeClassName(opportunity?.change1h)}>
                      {opportunity ? formatSignedPercent(opportunity.change1h) : <span className="muted-text">-</span>}
                    </td>
                    <td className="col-hidden-mobile">
                      {opportunity ? (
                        <div className="strategy-inline-list">
                          <span className={`state-pill strategy-state ${opportunity.leader.state}`}>
                            {translateStateLabel(opportunity.leader.state)}
                          </span>
                          <span className={`priority-pill ${opportunity.priorityBand}`}>
                            {translatePriorityBand(opportunity.priorityBand)}
                          </span>
                        </div>
                      ) : (
                        <span className="muted-text">-</span>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
