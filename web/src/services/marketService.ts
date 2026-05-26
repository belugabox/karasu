import {
  normalizeMarket,
  normalizeMarketAnalysis,
  normalizeMarketSignalHistory,
  normalizeOpportunity,
  normalizeSystemHealth,
  normalizeAlertEvent,
  type AlertEvent,
  type Market,
  type MarketAnalysis,
  type MarketSignalHistory,
  type Opportunity,
  type SystemHealth,
} from '../models/market'
import { fetchJson } from './http'

export async function getMarkets(): Promise<Market[]> {
  const data = await fetchJson<unknown[]>('/api/markets')
  return Array.isArray(data) ? data.map(normalizeMarket) : []
}

export async function getMarketAnalysis(symbol: string): Promise<MarketAnalysis> {
  const data = await fetchJson<unknown>(`/api/markets/${encodeURIComponent(symbol)}/analysis`)
  return normalizeMarketAnalysis(data)
}

export async function getMarketSignalHistory(symbol: string, limit = 40): Promise<MarketSignalHistory> {
  const params = new URLSearchParams({ limit: String(limit) })
  const data = await fetchJson<unknown>(`/api/markets/${encodeURIComponent(symbol)}/signals?${params.toString()}`)
  return normalizeMarketSignalHistory(data)
}

export async function getOpportunities(limit = 15): Promise<Opportunity[]> {
  const params = new URLSearchParams({ limit: String(limit) })
  const data = await fetchJson<unknown[]>(`/api/opportunities?${params.toString()}`)
  return Array.isArray(data) ? data.map(normalizeOpportunity) : []
}

export async function getSystemHealth(staleThresholdMin = 20): Promise<SystemHealth> {
  const params = new URLSearchParams({ staleThresholdMin: String(staleThresholdMin) })
  const data = await fetchJson<unknown>(`/api/system-health?${params.toString()}`)
  return normalizeSystemHealth(data)
}

export async function getRecentAlerts(limit = 50, activeOnly = false): Promise<AlertEvent[]> {
  const params = new URLSearchParams({
    limit: String(limit),
    activeOnly: activeOnly ? 'true' : 'false',
  })
  const data = await fetchJson<{ alerts?: unknown[] }>(`/api/alerts/recent?${params.toString()}`)
  return Array.isArray(data?.alerts) ? data.alerts.map(normalizeAlertEvent) : []
}
