import { useCallback, useMemo, useState } from 'react'
import { usePolling } from '../hooks/usePolling'
import { type AlertEvent } from '../models/market'
import { getRecentAlerts } from '../services/marketService'

type SeverityFilter = 'all' | 'critical' | 'warning' | 'info'
type StateFilter = 'all' | 'active' | 'resolved'

const severityOptions = [
  { value: 'all', label: 'Toutes severites' },
  { value: 'critical', label: 'Critique' },
  { value: 'warning', label: 'Alerte' },
  { value: 'info', label: 'Info' },
]

const stateOptions = [
  { value: 'all', label: 'Tous les etats' },
  { value: 'active', label: 'Actifs' },
  { value: 'resolved', label: 'Resolus' },
]

function formatDuration(from: string, to: string): string {
  try {
    const fromDate = new Date(from)
    const toDate = new Date(to)
    const diffMs = toDate.getTime() - fromDate.getTime()
    const diffSecs = Math.floor(diffMs / 1000)
    const diffMins = Math.floor(diffSecs / 60)
    const diffHours = Math.floor(diffMins / 60)
    const diffDays = Math.floor(diffHours / 24)

    if (diffDays > 0) return `${diffDays}j`
    if (diffHours > 0) return `${diffHours}h`
    if (diffMins > 0) return `${diffMins}m`
    return `${diffSecs}s`
  } catch {
    return 'N/A'
  }
}

function getSeverityColor(severity: string): string {
  switch (severity) {
    case 'critical':
      return '#b91c1c'
    case 'warning':
      return '#d97706'
    case 'info':
      return '#0f3c5b'
    default:
      return '#666'
  }
}

export function AlertsPanel() {
  const [alerts, setAlerts] = useState<AlertEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [severityFilter, setSeverityFilter] = useState<SeverityFilter>('all')
  const [stateFilter, setStateFilter] = useState<StateFilter>('all')

  const loadAlerts = useCallback(async () => {
    setLoading(true)
    try {
      const allAlerts = await getRecentAlerts(500, false)
      setAlerts(allAlerts)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'echec du chargement des alertes')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(loadAlerts, 30_000)

  const filteredAlerts = useMemo(() => {
    return alerts.filter((alert) => {
      if (severityFilter !== 'all' && alert.severity !== severityFilter) {
        return false
      }
      if (stateFilter === 'active' && !alert.active) {
        return false
      }
      if (stateFilter === 'resolved' && alert.active) {
        return false
      }
      return true
    })
  }, [alerts, severityFilter, stateFilter])

  const stats = useMemo(() => {
    const criticalActive = alerts.filter((a) => a.active && a.severity === 'critical').length
    const warningActive = alerts.filter((a) => a.active && a.severity === 'warning').length
    const totalActive = alerts.filter((a) => a.active).length
    const totalResolved = alerts.filter((a) => !a.active).length

    return { criticalActive, warningActive, totalActive, totalResolved }
  }, [alerts])

  return (
    <section className="panel">
      <div className="panel-head">
        <div>
          <h2>Alertes recentes</h2>
          <p>Historique complet des alertes dedoublonnees de l systeme (exchange, health, backfill).</p>
        </div>
        <div className="panel-head-actions">
          <button type="button" className="action-button" onClick={loadAlerts} disabled={loading}>
            {loading ? 'Actualisation...' : 'Rafraichir'}
          </button>
        </div>
      </div>

      {error && <p className="error-text">{error}</p>}

      <div className="kpi-grid" style={{ gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', marginBottom: '14px' }}>
        <article className="kpi-card">
          <span>Critiques actives</span>
          <h3 style={{ color: stats.criticalActive > 0 ? '#b91c1c' : '#047857' }}>{stats.criticalActive}</h3>
        </article>
        <article className="kpi-card">
          <span>Alertes actives</span>
          <h3 style={{ color: stats.warningActive > 0 ? '#d97706' : '#047857' }}>{stats.warningActive}</h3>
        </article>
        <article className="kpi-card">
          <span>Total actifs</span>
          <h3>{stats.totalActive}</h3>
        </article>
        <article className="kpi-card">
          <span>Resolus</span>
          <h3>{stats.totalResolved}</h3>
        </article>
      </div>

      <div style={{ display: 'flex', gap: '10px', marginBottom: '14px', flexWrap: 'wrap' }}>
        <select
          value={severityFilter}
          onChange={(e) => setSeverityFilter(e.target.value as SeverityFilter)}
          className="select-input"
        >
          {severityOptions.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
        <select
          value={stateFilter}
          onChange={(e) => setStateFilter(e.target.value as StateFilter)}
          className="select-input"
        >
          {stateOptions.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>

      {filteredAlerts.length === 0 ? (
        <p className="empty-text">Aucune alerte correspondant aux filtres.</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Severite</th>
                <th>Etat</th>
                <th>Message</th>
                <th>Categorie</th>
                <th>Source</th>
                <th>Compte</th>
                <th>Duree</th>
              </tr>
            </thead>
            <tbody>
              {filteredAlerts.map((alert) => (
                <tr key={alert.id}>
                  <td>
                    <span
                      style={{
                        display: 'inline-block',
                        padding: '2px 8px',
                        borderRadius: '4px',
                        backgroundColor: getSeverityColor(alert.severity),
                        color: '#fff',
                        fontSize: '0.78rem',
                        fontWeight: 600,
                      }}
                    >
                      {alert.severity === 'critical' ? 'CRITIQUE' : alert.severity === 'warning' ? 'ALERTE' : 'INFO'}
                    </span>
                  </td>
                  <td>{alert.active ? '🔴 Actif' : '✓ Resolu'}</td>
                  <td>{alert.message}</td>
                  <td>{alert.category}</td>
                  <td>{alert.source}</td>
                  <td style={{ textAlign: 'center' }}>x{alert.count}</td>
                  <td>{formatDuration(alert.firstSeen, alert.lastSeen)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {filteredAlerts.length > 0 && (
        <p
          className="section-description"
          style={{ marginTop: '10px', fontSize: '0.82rem', color: '#666' }}
        >
          Affichage de {filteredAlerts.length} alerte(s) sur {alerts.length} total. Dernière actualisation:{' '}
          {new Date().toLocaleTimeString('fr-FR')}
        </p>
      )}
    </section>
  )
}
