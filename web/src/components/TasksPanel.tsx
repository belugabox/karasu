import { useCallback, useMemo, useState } from 'react'
import { usePolling } from '../hooks/usePolling'
import type { BackfillJob } from '../models/task'
import { getBackfillJobs, launchBackfillAllDays } from '../services/taskService'
import { formatDurationMs } from '../utils/format'

function translateJobState(state: string): string {
  switch (state) {
    case 'queued':
      return 'en file'
    case 'running':
      return 'en cours'
    case 'done':
      return 'termine'
    case 'failed':
      return 'echoue'
    default:
      return state || '-'
  }
}

export function TasksPanel() {
  const [jobs, setJobs] = useState<BackfillJob[]>([])
  const [loading, setLoading] = useState(false)
  const [launching, setLaunching] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')

  const loadJobs = useCallback(async () => {
    setLoading(true)
    try {
      const nextJobs = await getBackfillJobs(80)
      setJobs(nextJobs)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'echec du chargement des taches')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(loadJobs, 5_000)

  const launchBackfill = useCallback(async () => {
    setLaunching(true)
    setMessage('')
    try {
      const job = await launchBackfillAllDays(7)
      setJobs((current) => [job, ...current.filter((item) => item.id !== job.id)])
      setMessage(`Backfill lance : ${job.id}`)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'echec du lancement du backfill')
    } finally {
      setLaunching(false)
    }
  }, [])

  const runningJobs = useMemo(
    () => jobs.filter((job) => job.state === 'running' || job.state === 'queued').length,
    [jobs],
  )
  const completedJobs = useMemo(() => jobs.filter((job) => job.state === 'done').length, [jobs])
  const failedJobs = useMemo(() => jobs.filter((job) => job.state === 'failed').length, [jobs])

  return (
    <section className="panel">
      <div className="panel-head">
        <div>
          <h2>Taches</h2>
          <p>Suivi des jobs de backfill avec etat, duree et details d execution.</p>
        </div>
        <div className="panel-head-actions">
          <button
            type="button"
            className="action-button"
            onClick={launchBackfill}
            disabled={launching}
          >
            {launching ? 'Lancement en cours...' : 'Lancer un backfill 7j (tous les symboles)'}
          </button>
          <span className="muted-text">{loading ? 'Actualisation...' : `${jobs.length} taches`}</span>
        </div>
      </div>

      {message && <p className="success-text">{message}</p>}
      {error && <p className="error-text">{error}</p>}

      <div className="kpi-grid tasks-kpis">
        <article className="kpi-card">
          <span>En cours</span>
          <h3>{runningJobs}</h3>
        </article>
        <article className="kpi-card">
          <span>Terminees</span>
          <h3>{completedJobs}</h3>
        </article>
        <article className="kpi-card">
          <span>Echouees</span>
          <h3>{failedJobs}</h3>
        </article>
      </div>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Job</th>
              <th>Etat</th>
              <th>Symboles</th>
              <th>Cree</th>
              <th>Demarre</th>
              <th>Termine</th>
              <th>Duree</th>
              <th>Details</th>
            </tr>
          </thead>
          <tbody>
            {jobs.map((job) => {
              const startedMs = job.startedAt ? Date.parse(job.startedAt) : 0
              const endedMs = job.endedAt ? Date.parse(job.endedAt) : 0
              const durationMs =
                job.report?.durationMs ??
                (startedMs > 0 ? (endedMs > 0 ? endedMs - startedMs : Date.now() - startedMs) : 0)

              return (
                <tr key={job.id}>
                  <td>{job.id}</td>
                  <td>
                    <span className="state-pill">{translateJobState(job.state)}</span>
                  </td>
                  <td>{job.symbols.join(', ') || '-'}</td>
                  <td>{job.createdAt ? new Date(job.createdAt).toLocaleString() : '-'}</td>
                  <td>{job.startedAt ? new Date(job.startedAt).toLocaleString() : '-'}</td>
                  <td>{job.endedAt ? new Date(job.endedAt).toLocaleString() : '-'}</td>
                  <td>{formatDurationMs(durationMs)}</td>
                  <td>
                    {job.error
                      ? `Erreur : ${job.error}`
                      : job.report
                        ? `5m persistes : ${job.report.persisted5m} | 1m recuperes : ${job.report.fetched1mCandles} | chunks : ${job.report.chunks}`
                        : job.reason || '-'}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </section>
  )
}
