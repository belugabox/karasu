import { useCallback, useState } from 'react'
import { usePolling } from '../hooks/usePolling'
import { type AlertEvent, type SystemHealth } from '../models/market'
import { type AlertsPage, getRecentAlerts, getSystemHealth } from '../services/marketService'
import { translateAlertSeverity, translateHealthIssue } from '../utils/market-translations'

const PAGE_SIZE = 20

export function SystemPanel() {
  const [systemHealth, setSystemHealth] = useState<SystemHealth | null>(null)
  const [systemHealthLoading, setSystemHealthLoading] = useState(false)
  const [systemHealthError, setSystemHealthError] = useState('')

  const [alerts, setAlerts] = useState<AlertEvent[]>([])
  const [alertsTotal, setAlertsTotal] = useState(0)
  const [alertsOffset, setAlertsOffset] = useState(0)
  const [alertsLoading, setAlertsLoading] = useState(false)
  const [alertsError, setAlertsError] = useState('')
  const [activeOnly, setActiveOnly] = useState(false)

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

  const loadAlerts = useCallback(async (offset: number, onlyActive: boolean) => {
    setAlertsLoading(true)
    try {
      const page: AlertsPage = await getRecentAlerts(PAGE_SIZE, onlyActive, offset)
      setAlerts(page.alerts)
      setAlertsTotal(page.total)
      setAlertsError('')
    } catch (err) {
      setAlertsError(err instanceof Error ? err.message : 'echec du chargement des alertes')
    } finally {
      setAlertsLoading(false)
    }
  }, [])

  const reloadAlerts = useCallback(() => {
    loadAlerts(alertsOffset, activeOnly)
  }, [loadAlerts, alertsOffset, activeOnly])

  usePolling(reloadAlerts, 30_000)

  function goToPage(newOffset: number) {
    setAlertsOffset(newOffset)
    loadAlerts(newOffset, activeOnly)
  }

  function toggleActiveOnly(value: boolean) {
    setActiveOnly(value)
    setAlertsOffset(0)
    loadAlerts(0, value)
  }

  const totalPages = Math.ceil(alertsTotal / PAGE_SIZE)
  const currentPage = Math.floor(alertsOffset / PAGE_SIZE) + 1

  return (
    <section className="panel">
      <div className="panel-head">
        <div>
          <h2>Santé système</h2>
          <p>Etat du pipeline d ingestion, fraicheur des flux et alertes persistantes.</p>
        </div>
        <span className="muted-text">
          {systemHealthLoading ? 'Actualisation...' : systemHealth ? (systemHealth.isHealthy ? 'etat sain' : 'etat degrade') : 'indisponible'}
        </span>
      </div>

      {systemHealthError && <p className="error-text">{systemHealthError}</p>}

      {systemHealth && (
        <>
          <div className="overview-grid">
            <article className="overview-card">
              <span>Univers</span>
              <strong>{systemHealth.universeSymbols}</strong>
              <p>Nombre total de symboles dans l univers surveille.</p>
            </article>
            <article className="overview-card">
              <span>Top</span>
              <strong>{systemHealth.topSymbols}</strong>
              <p>Symboles retenus dans le top du classement.</p>
            </article>
            <article className="overview-card">
              <span>Live</span>
              <strong>{systemHealth.liveSymbols}</strong>
              <p>Symboles avec flux live actif.</p>
            </article>
            <article className="overview-card">
              <span>Backfill en cours</span>
              <strong>{systemHealth.backfillRunningJobs}</strong>
              <p>Jobs de backfill en cours d execution.</p>
            </article>
          </div>

          {systemHealth.issues.length > 0 && (
            <div className="section-description compact-description">
              <p>Problemes detectes :</p>
              <div className="analysis-tag-list">
                {systemHealth.issues.map((issue) => (
                  <span key={issue} className="analysis-tag risk-tag">{translateHealthIssue(issue)}</span>
                ))}
              </div>
            </div>
          )}

          {systemHealth.topStaleExamples.length > 0 && (
            <div className="section-description compact-description">
              <p>Exemples de retard :</p>
              <div className="analysis-tag-list">
                {systemHealth.topStaleExamples.map((item) => (
                  <span key={item} className="analysis-tag">retard {item}</span>
                ))}
              </div>
            </div>
          )}
        </>
      )}

      <div className="panel-head" style={{ marginTop: '2rem' }}>
        <div>
          <h2>Alertes</h2>
          <p>Historique des alertes persistantes. Les alertes sont conservees 30 jours.</p>
        </div>
        <div className="panel-head-actions">
          <label className="toggle-chip">
            <input
              type="checkbox"
              checked={activeOnly}
              onChange={(event) => toggleActiveOnly(event.target.checked)}
            />
            <span>Actives uniquement</span>
          </label>
          <span className="muted-text">
            {alertsLoading ? 'Actualisation...' : `${alertsTotal} alerte(s) au total`}
          </span>
        </div>
      </div>

      {alertsError && <p className="error-text">{alertsError}</p>}

      {alerts.length > 0 ? (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Severite</th>
                <th>Categorie</th>
                <th>Message</th>
                <th>Source</th>
                <th>Etat</th>
                <th>Occurrences</th>
                <th>Premiere apparition</th>
                <th>Derniere apparition</th>
              </tr>
            </thead>
            <tbody>
              {alerts.map((alert) => (
                <tr key={alert.id}>
                  <td>
                    <span className={`analysis-tag ${alert.severity === 'critical' || alert.severity === 'warning' ? 'risk-tag' : ''}`}>
                      {translateAlertSeverity(alert.severity)}
                    </span>
                  </td>
                  <td>{alert.category}</td>
                  <td>{alert.message}</td>
                  <td>{alert.source}{alert.symbol ? ` (${alert.symbol})` : ''}</td>
                  <td>
                    <span className={`analysis-tag ${alert.active ? 'risk-tag' : ''}`}>
                      {alert.active ? 'actif' : 'resolu'}
                    </span>
                  </td>
                  <td>{alert.count}</td>
                  <td>{new Date(alert.firstSeen).toLocaleString()}</td>
                  <td>{new Date(alert.lastSeen).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        !alertsLoading && <p className="empty-text">Aucune alerte correspondant aux filtres actuels.</p>
      )}

      {totalPages > 1 && (
        <div className="panel-head-actions" style={{ marginTop: '1rem', justifyContent: 'center' }}>
          <button
            type="button"
            className="action-button"
            disabled={currentPage <= 1}
            onClick={() => goToPage(alertsOffset - PAGE_SIZE)}
          >
            Precedent
          </button>
          <span className="muted-text">Page {currentPage} / {totalPages}</span>
          <button
            type="button"
            className="action-button"
            disabled={currentPage >= totalPages}
            onClick={() => goToPage(alertsOffset + PAGE_SIZE)}
          >
            Suivant
          </button>
        </div>
      )}
    </section>
  )
}
