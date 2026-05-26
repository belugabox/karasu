import { normalizeBackfillJob, type BackfillJob } from '../models/task'
import { fetchJson } from './http'

type BackfillJobsResponse = {
  jobs?: unknown[]
}

type BackfillLaunchResponse = {
  job?: unknown
}

export async function getBackfillJobs(limit = 80): Promise<BackfillJob[]> {
  const params = new URLSearchParams({ limit: String(limit) })
  const data = await fetchJson<BackfillJobsResponse>(`/api/backfill-jobs?${params.toString()}`)

  return Array.isArray(data.jobs) ? data.jobs.map(normalizeBackfillJob) : []
}

export async function launchBackfillAllDays(days = 7): Promise<BackfillJob> {
  const now = new Date()
  const from = new Date(now.getTime() - days * 24 * 60 * 60 * 1000)
  const params = new URLSearchParams({
    from: from.toISOString(),
    to: now.toISOString(),
  })

  const data = await fetchJson<BackfillLaunchResponse>(`/api/backfill-5m?${params.toString()}`, {
    method: 'POST',
  })

  const job = data.job ? normalizeBackfillJob(data.job) : undefined
  if (!job?.id) {
    throw new Error('missing backfill job id')
  }

  return job
}
