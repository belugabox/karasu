import { useMemo } from 'react'
import { type Market, type MarketAnalysis, type MarketSignalHistory } from '../models/market'
import { formatNumber, formatSignedPercent } from '../utils/format'
import { formatBarsAgo, translateProfileLabel, translateReason, translateRisk, translateStateLabel } from '../utils/market-translations'

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
  const statePriority: Record<string, number> = { entry: 5, hold: 4, watch: 3, avoid: 2, exit: 1 }
  return [...strategies].sort((left, right) => {
    if (right.score !== left.score) {
      return right.score - left.score
    }
    return (statePriority[right.state] ?? 0) - (statePriority[left.state] ?? 0)
  })
}

type MarketAnalysisSectionProps = {
  selectedSymbol: string
  analysis: MarketAnalysis | null
  analysisLoading: boolean
  analysisError: string
  signalHistory: MarketSignalHistory | null
  signalHistoryLoading: boolean
  signalHistoryError: string
}

export function MarketAnalysisSection({
  selectedSymbol,
  analysis,
  analysisLoading,
  analysisError,
  signalHistory,
  signalHistoryLoading,
  signalHistoryError,
}: MarketAnalysisSectionProps) {
  const historyTimeframeLabel = signalHistory?.timeframe ?? '5m'

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
  )
}
