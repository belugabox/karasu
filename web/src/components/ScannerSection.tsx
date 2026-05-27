import { type Opportunity } from '../models/market'
import { formatNumber, formatSignedPercent } from '../utils/format'
import {
  formatBarsAgo,
  formatProfileDisplay,
  translateOpportunitySummary,
  translatePrimaryAction,
  translatePriorityBand,
  translateReason,
  translateRisk,
  translateStateLabel,
} from '../utils/market-translations'

const priorityBands = [
  { value: 'all', label: 'Toutes les priorites' },
  { value: 'actionable', label: 'Actionnable' },
  { value: 'strong-watch', label: 'Surveillance forte' },
  { value: 'watchlist', label: 'Liste de surveillance' },
  { value: 'defensive', label: 'Defensif' },
]

type ScannerSectionProps = {
  opportunities: Opportunity[]
  loading: boolean
  error: string
  priorityBandFilter: string
  freshOnly: boolean
  consensusOnly: boolean
  onPriorityBandFilterChange: (value: string) => void
  onFreshOnlyChange: (value: boolean) => void
  onConsensusOnlyChange: (value: boolean) => void
  onSelectSymbol: (symbol: string) => void
}

export function ScannerSection({
  opportunities,
  loading,
  error,
  priorityBandFilter,
  freshOnly,
  consensusOnly,
  onPriorityBandFilterChange,
  onFreshOnlyChange,
  onConsensusOnlyChange,
  onSelectSymbol,
}: ScannerSectionProps) {
  const filtered = opportunities.filter((opportunity) => {
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

  const overview = filtered.reduce(
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
    { actionableCount: 0, freshEntryCount: 0, consensusCount: 0, defensiveCount: 0 },
  )

  return (
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
            onChange={(event) => onPriorityBandFilterChange(event.target.value)}
          >
            {priorityBands.map((band) => (
              <option key={band.value} value={band.value}>
                {band.label}
              </option>
            ))}
          </select>
          <label className="toggle-chip">
            <input type="checkbox" checked={freshOnly} onChange={(event) => onFreshOnlyChange(event.target.checked)} />
            <span>Entrees fraiches uniquement</span>
          </label>
          <label className="toggle-chip">
            <input type="checkbox" checked={consensusOnly} onChange={(event) => onConsensusOnlyChange(event.target.checked)} />
            <span>Consensus uniquement</span>
          </label>
          <span className="muted-text">{loading ? 'Actualisation du scanner...' : `${filtered.length} opportunites`}</span>
        </div>
      </div>

      <div className="overview-grid scanner-overview-grid">
        <article className="overview-card">
          <span>Actionnables maintenant</span>
          <strong>{overview.actionableCount}</strong>
          <p>Configurations de plus haute priorite suffisamment solides pour une revue immediate.</p>
        </article>
        <article className="overview-card">
          <span>Entrees fraiches</span>
          <strong>{overview.freshEntryCount}</strong>
          <p>Opportunites ou au moins un profil vient de passer en entree sur la derniere bougie stockee.</p>
        </article>
        <article className="overview-card">
          <span>Configurations en consensus</span>
          <strong>{overview.consensusCount}</strong>
          <p>Opportunites ou plusieurs profils sont deja actifs en meme temps.</p>
        </article>
        <article className="overview-card">
          <span>Configurations defensives</span>
          <strong>{overview.defensiveCount}</strong>
          <p>Entrees ecartees par un contexte faible, des sorties recentes ou un leadership peu convaincant.</p>
        </article>
      </div>

      {error && <p className="error-text">{error}</p>}

      {filtered.length > 0 ? (
        <div className="scanner-card-grid">
          {filtered.map((opportunity) => (
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
                <button type="button" className="action-button" onClick={() => onSelectSymbol(opportunity.symbol)}>
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
                  <strong style={{ color: opportunity.leader.color || undefined }}>
                    {formatProfileDisplay(opportunity.leader.label || 'n/a', opportunity.leader.icon)}
                  </strong>
                </div>
                <div>
                  <span>Etat leader</span>
                  <strong className={opportunity.leader.state === 'entry' || opportunity.leader.state === 'hold' ? 'positive' : opportunity.leader.state === 'exit' || opportunity.leader.state === 'avoid' ? 'negative' : ''}>
                    {translateStateLabel(opportunity.leader.state || 'n/a')}
                  </strong>
                </div>
              </div>

              <div className="analysis-tag-list">
                {opportunity.leader.description && <span className="analysis-tag">profil : {opportunity.leader.description}</span>}
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
  )
}
