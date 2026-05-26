import { toNumber } from './parse'

export function formatNumber(value: unknown, decimals = 2): string {
  return toNumber(value).toFixed(decimals)
}

export function formatSignedPercent(value: number, decimals = 2): string {
  const formatted = formatNumber(value, decimals)
  return `${value >= 0 ? '+' : ''}${formatted}%`
}

export function formatDurationMs(value: number): string {
  if (!Number.isFinite(value) || value < 0) {
    return '-'
  }

  if (value < 1000) {
    return `${Math.round(value)}ms`
  }

  const seconds = value / 1000
  if (seconds < 60) {
    return `${seconds.toFixed(1)}s`
  }

  const minutes = Math.floor(seconds / 60)
  const remaining = Math.round(seconds % 60)
  return `${minutes}m ${remaining}s`
}
