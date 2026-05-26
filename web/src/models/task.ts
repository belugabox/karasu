import { toNumber, toStringValue } from '../utils/parse'

export type BackfillJobReport = {
  symbols: number
  chunks: number
  fetched1mCandles: number
  aggregated5m: number
  filtered5m: number
  persisted5m: number
  durationMs: number
}

export type BackfillJob = {
  id: string
  symbols: string[]
  from: string
  to: string
  reason: string
  state: string
  createdAt: string
  startedAt?: string
  endedAt?: string
  report?: BackfillJobReport
  error?: string
}

export function normalizeBackfillJob(raw: unknown): BackfillJob {
  const r = (raw as Record<string, unknown>) || {}
  const reportRaw = ((r.report as Record<string, unknown> | undefined) || {}) as Record<
    string,
    unknown
  >

  return {
    id: toStringValue(r.id),
    symbols: Array.isArray(r.symbols)
      ? r.symbols.map((symbol) => toStringValue(symbol)).filter((symbol) => symbol.length > 0)
      : [],
    from: toStringValue(r.from),
    to: toStringValue(r.to),
    reason: toStringValue(r.reason),
    state: toStringValue(r.state),
    createdAt: toStringValue(r.createdAt),
    startedAt: toStringValue(r.startedAt) || undefined,
    endedAt: toStringValue(r.endedAt) || undefined,
    error: toStringValue(r.error) || undefined,
    report: r.report
      ? {
          symbols: toNumber(reportRaw.symbols),
          chunks: toNumber(reportRaw.chunks),
          fetched1mCandles: toNumber(reportRaw.fetched1mCandles),
          aggregated5m: toNumber(reportRaw.aggregated5m),
          filtered5m: toNumber(reportRaw.filtered5m),
          persisted5m: toNumber(reportRaw.persisted5m),
          durationMs: toNumber(reportRaw.durationMs),
        }
      : undefined,
  }
}
