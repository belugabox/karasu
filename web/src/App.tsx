import { useEffect, useMemo, useState, type MouseEvent } from 'react'
import './App.css'

type Market = {
  symbol: string
  quoteVolume: number
  quoteVolumePosition: number
  qualityScore: number
  qualityRsi: number
  qualityMacd: number
  qualityBollinger: number
  qualityVolume: number
  qualitySma: number
  change24h: number
  change24hPosition: number
  change1h: number
  change1hPosition: number
  change5m: number
  change5mPosition: number
}

type LiveCandle = {
  symbol: string
  open: number
  high: number
  low: number
  close: number
  volume: number
  openTime: string
  closeTime: string
  updatedAt: string
}

type Candle = {
  open: number
  high: number
  low: number
  close: number
  volume: number
  openTime: string
  closeTime: string
}

type SparklineProps = {
  values: number[]
  stroke: string
  fill: string
  height?: number
  label: string
}

type CandlestickChartProps = {
  candles: Candle[]
  hoveredIndex: number | null
  onHoverIndexChange: (index: number | null) => void
}

type ChartWindow = {
  candles: number
  label: string
}

type BackfillJobReport = {
  symbols: number
  chunks: number
  fetched1mCandles: number
  aggregated5m: number
  filtered5m: number
  persisted5m: number
  durationMs: number
}

type BackfillJob = {
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

type DailySymbolActivity = {
  day: string
  symbolCount: number
  candleCount: number
}

type WalletAsset = {
  symbol: string
  amount: number
  inOrder: number
  stakingAmount: number
  costBasisValue: number
  pnlValue: number
  pnlPercent: number
  value: number
}

type Wallet = {
  totalValue: number
  cashValue: number
  assetValue: number
  netDepositsValue: number
  pnlValue: number
  pnlPercent: number
  assets: WalletAsset[]
}

type AppTab = 'overview' | 'markets' | 'backtest' | 'tasks' | 'heatmap' | 'wallet'

type MarketSortKey = 'score' | 'change24h' | 'change1h' | 'change5m' | 'quoteVolume'

const chartWindows: ChartWindow[] = [
  { candles: 48, label: '4h' },
  { candles: 96, label: '8h' },
  { candles: 192, label: '16h' },
  { candles: 500, label: 'max' },
]

function toNumber(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : 0
  }
  return 0
}

function toStringValue(value: unknown): string {
  if (typeof value === 'string') {
    return value
  }
  if (typeof value === 'number') {
    return String(value)
  }
  return ''
}

function normalizeMarket(raw: unknown): Market {
  const r = (raw as Record<string, unknown>) || {}
  return {
    symbol: toStringValue(r.symbol ?? r.Symbol),
    quoteVolume: toNumber(r.quoteVolume ?? r.QuoteVolume),
    quoteVolumePosition: toNumber(r.quoteVolumePosition ?? r.QuoteVolumePosition),
    qualityScore: toNumber(r.qualityScore ?? r.QualityScore),
    qualityRsi: toNumber(r.qualityRsi ?? r.QualityRSI),
    qualityMacd: toNumber(r.qualityMacd ?? r.QualityMACD),
    qualityBollinger: toNumber(r.qualityBollinger ?? r.QualityBollinger),
    qualityVolume: toNumber(r.qualityVolume ?? r.QualityVolume),
    qualitySma: toNumber(r.qualitySma ?? r.QualitySMA),
    change24h: toNumber(r.change24h ?? r.Change24h),
    change24hPosition: toNumber(r.change24hPosition ?? r.Change24hPosition),
    change1h: toNumber(r.change1h ?? r.Change1h),
    change1hPosition: toNumber(r.change1hPosition ?? r.Change1hPosition),
    change5m: toNumber(r.change5m ?? r.Change5m),
    change5mPosition: toNumber(r.change5mPosition ?? r.Change5mPosition),
  }
}

function normalizeLiveCandle(raw: unknown): LiveCandle {
  const r = (raw as Record<string, unknown>) || {}
  return {
    symbol: toStringValue(r.symbol ?? r.Symbol),
    open: toNumber(r.open ?? r.Open),
    high: toNumber(r.high ?? r.High),
    low: toNumber(r.low ?? r.Low),
    close: toNumber(r.close ?? r.Close),
    volume: toNumber(r.volume ?? r.Volume),
    openTime: toStringValue(r.openTime ?? r.OpenTime),
    closeTime: toStringValue(r.closeTime ?? r.CloseTime),
    updatedAt: toStringValue(r.updatedAt ?? r.UpdatedAt),
  }
}

function normalizeCandle(raw: unknown): Candle {
  const r = (raw as Record<string, unknown>) || {}
  return {
    open: toNumber(r.open ?? r.Open),
    high: toNumber(r.high ?? r.High),
    low: toNumber(r.low ?? r.Low),
    close: toNumber(r.close ?? r.Close),
    volume: toNumber(r.volume ?? r.Volume),
    openTime: toStringValue(r.openTime ?? r.OpenTime),
    closeTime: toStringValue(r.closeTime ?? r.CloseTime),
  }
}

function formatNumber(value: unknown, decimals: number): string {
  return toNumber(value).toFixed(decimals)
}

function normalizeBackfillJob(raw: unknown): BackfillJob {
  const r = (raw as Record<string, unknown>) || {}
  const reportRaw = ((r.report as Record<string, unknown> | undefined) || {}) as Record<string, unknown>
  return {
    id: toStringValue(r.id),
    symbols: Array.isArray(r.symbols)
      ? r.symbols.map((s) => toStringValue(s)).filter((s) => s.length > 0)
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

function normalizeDailySymbolActivity(raw: unknown): DailySymbolActivity {
  const r = (raw as Record<string, unknown>) || {}
  return {
    day: toStringValue(r.day ?? r.Day),
    symbolCount: toNumber(r.symbolCount ?? r.SymbolCount),
    candleCount: toNumber(r.candleCount ?? r.CandleCount),
  }
}

function normalizeWalletAsset(raw: unknown): WalletAsset {
  const r = (raw as Record<string, unknown>) || {}
  return {
    symbol: toStringValue(r.symbol ?? r.Symbol),
    amount: toNumber(r.amount ?? r.Amount),
    inOrder: toNumber(r.inOrder ?? r.InOrder),
    stakingAmount: toNumber(r.stakingAmount ?? r.StakingAmount),
    costBasisValue: toNumber(r.costBasisValue ?? r.CostBasisValue),
    pnlValue: toNumber(r.pnlValue ?? r.PnLValue),
    pnlPercent: toNumber(r.pnlPercent ?? r.PnLPercent),
    value: toNumber(r.value ?? r.Value),
  }
}

function normalizeWallet(raw: unknown): Wallet {
  const r = (raw as Record<string, unknown>) || {}
  const assets = Array.isArray(r.assets ?? r.Assets)
    ? (r.assets ?? r.Assets).map(normalizeWalletAsset)
    : []
  return {
    totalValue: toNumber(r.totalValue ?? r.TotalValue),
    cashValue: toNumber(r.cashValue ?? r.CashValue),
    assetValue: toNumber(r.assetValue ?? r.AssetValue),
    netDepositsValue: toNumber(r.netDepositsValue ?? r.NetDepositsValue),
    pnlValue: toNumber(r.pnlValue ?? r.PnLValue),
    pnlPercent: toNumber(r.pnlPercent ?? r.PnLPercent),
    assets,
  }
}

function formatDurationMs(value: number): string {
  if (!Number.isFinite(value) || value < 0) {
    return '-'
  }
  if (value < 1000) {
    return `${Math.round(value)}ms`
  }
  const sec = value / 1000
  if (sec < 60) {
    return `${sec.toFixed(1)}s`
  }
  const min = Math.floor(sec / 60)
  const remSec = Math.round(sec % 60)
  return `${min}m ${remSec}s`
}

function Sparkline({ values, stroke, fill, height = 72, label }: SparklineProps) {
  const width = 240
  const safeValues = values.filter((value) => Number.isFinite(value))

  if (safeValues.length === 0) {
    return <div className="sparkline-empty">No data for {label}</div>
  }

  const min = Math.min(...safeValues)
  const max = Math.max(...safeValues)
  const span = max - min || 1
  const step = safeValues.length > 1 ? width / (safeValues.length - 1) : width

  const points = safeValues
    .map((value, index) => {
      const x = index * step
      const normalized = (value - min) / span
      const y = height - normalized * (height - 8)
      return `${x},${y}`
    })
    .join(' ')

  const areaPoints = `0,${height} ${points} ${width},${height}`

  return (
    <div className="sparkline-wrap" aria-label={label} role="img">
      <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none">
        <polygon points={areaPoints} fill={fill} />
        <polyline points={points} fill="none" stroke={stroke} strokeWidth="2.5" />
      </svg>
    </div>
  )
}

function CandlestickChart({
  candles,
  hoveredIndex,
  onHoverIndexChange,
}: CandlestickChartProps) {
  const width = 960
  const height = 340

  const safeCandles = candles.filter((candle) => {
    return [candle.open, candle.high, candle.low, candle.close].every((value) =>
      Number.isFinite(value),
    )
  })

  if (safeCandles.length === 0) {
    return <div className="sparkline-empty">No candle data available</div>
  }

  const lows = safeCandles.map((candle) => candle.low)
  const highs = safeCandles.map((candle) => candle.high)
  const min = Math.min(...lows)
  const max = Math.max(...highs)
  const span = max - min || 1
  const innerTop = 18
  const innerBottom = 28
  const drawableHeight = height - innerTop - innerBottom
  const step = width / safeCandles.length
  const bodyWidth = Math.max(4, step * 0.58)

  const mapY = (value: number) => {
    const normalized = (value - min) / span
    return innerTop + (1 - normalized) * drawableHeight
  }

  const handleMouseMove = (event: MouseEvent<SVGSVGElement>) => {
    const rect = event.currentTarget.getBoundingClientRect()
    const offsetX = event.clientX - rect.left
    const ratio = offsetX / rect.width
    const nextIndex = Math.max(
      0,
      Math.min(safeCandles.length - 1, Math.floor(ratio * safeCandles.length)),
    )
    onHoverIndexChange(nextIndex)
  }

  const hover =
    hoveredIndex !== null && hoveredIndex >= 0 && hoveredIndex < safeCandles.length
      ? safeCandles[hoveredIndex]
      : safeCandles[safeCandles.length - 1]
  const hoverIndex =
    hoveredIndex !== null && hoveredIndex >= 0 && hoveredIndex < safeCandles.length
      ? hoveredIndex
      : safeCandles.length - 1

  const hoverX = hoverIndex * step + step / 2
  const hoverOpenY = mapY(hover.open)
  const hoverCloseY = mapY(hover.close)
  const hoverHighY = mapY(hover.high)
  const hoverLowY = mapY(hover.low)

  return (
    <div className="candlestick-wrap">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
        onMouseMove={handleMouseMove}
        onMouseLeave={() => onHoverIndexChange(null)}
      >
        <line
          x1="0"
          x2={width}
          y1={innerTop}
          y2={innerTop}
          className="chart-grid-line"
        />
        <line
          x1="0"
          x2={width}
          y1={innerTop + drawableHeight / 2}
          y2={innerTop + drawableHeight / 2}
          className="chart-grid-line"
        />
        <line
          x1="0"
          x2={width}
          y1={innerTop + drawableHeight}
          y2={innerTop + drawableHeight}
          className="chart-grid-line"
        />

        {safeCandles.map((candle, index) => {
          const centerX = index * step + step / 2
          const openY = mapY(candle.open)
          const closeY = mapY(candle.close)
          const highY = mapY(candle.high)
          const lowY = mapY(candle.low)
          const isBullish = candle.close >= candle.open
          const bodyTop = Math.min(openY, closeY)
          const bodyHeight = Math.max(1, Math.abs(closeY - openY))
          const bodyLeft = centerX - bodyWidth / 2

          return (
            <g key={candle.openTime}>
              <line
                x1={centerX}
                x2={centerX}
                y1={highY}
                y2={lowY}
                className={isBullish ? 'candle-wick bullish' : 'candle-wick bearish'}
              />
              <rect
                x={bodyLeft}
                y={bodyTop}
                width={bodyWidth}
                height={bodyHeight}
                rx="1.5"
                className={isBullish ? 'candle-body bullish' : 'candle-body bearish'}
              />
            </g>
          )
        })}

        <line
          x1={hoverX}
          x2={hoverX}
          y1={innerTop}
          y2={innerTop + drawableHeight}
          className="chart-hover-line"
        />
        <circle cx={hoverX} cy={hoverCloseY} r="3.75" className="chart-hover-dot" />
        <circle cx={hoverX} cy={hoverOpenY} r="2.25" className="chart-hover-open-dot" />
        <circle cx={hoverX} cy={hoverHighY} r="2.25" className="chart-hover-high-dot" />
        <circle cx={hoverX} cy={hoverLowY} r="2.25" className="chart-hover-low-dot" />
      </svg>

      <div className="candlestick-axis">
        <span>{new Date(safeCandles[0].openTime).toLocaleString()}</span>
        <span>{new Date(safeCandles[safeCandles.length - 1].openTime).toLocaleString()}</span>
      </div>

      <div className="candlestick-tooltip">
        <div>
          <p>{new Date(hover.openTime).toLocaleString()}</p>
          <strong>
            O {formatNumber(hover.open, 2)} H {formatNumber(hover.high, 2)} L{' '}
            {formatNumber(hover.low, 2)} C {formatNumber(hover.close, 2)}
          </strong>
        </div>
        <span>{hover.close >= hover.open ? 'bullish' : 'bearish'}</span>
      </div>
    </div>
  )
}

function App() {
  const [markets, setMarkets] = useState<Market[]>([])
  const [liveCandles, setLiveCandles] = useState<LiveCandle[]>([])
  const [history5m, setHistory5m] = useState<Candle[]>([])
  const [marketMiniCharts, setMarketMiniCharts] = useState<Record<string, number[]>>({})
  const [selectedSymbol, setSelectedSymbol] = useState('BTC')
  const [chartWindow, setChartWindow] = useState(96)
  const [hoveredChartIndex, setHoveredChartIndex] = useState<number | null>(null)
  const [lastRefresh, setLastRefresh] = useState<string>('')
  const [error, setError] = useState<string>('')
  const [isBacktesting, setIsBacktesting] = useState(false)
  const [backtestMessage, setBacktestMessage] = useState<string>('')
  const [activeTab, setActiveTab] = useState<AppTab>('overview')
  const [backfillJobs, setBackfillJobs] = useState<BackfillJob[]>([])
  const [jobsLoading, setJobsLoading] = useState(false)
  const [isBackfillingAll, setIsBackfillingAll] = useState(false)
  const [tasksMessage, setTasksMessage] = useState<string>('')
  const [dailyActivity, setDailyActivity] = useState<DailySymbolActivity[]>([])
  const [activityLoading, setActivityLoading] = useState(false)
  const [wallet, setWallet] = useState<Wallet>({
    totalValue: 0,
    cashValue: 0,
    assetValue: 0,
    netDepositsValue: 0,
    pnlValue: 0,
    pnlPercent: 0,
    assets: [],
  })
  const [walletLoading, setWalletLoading] = useState(false)
  const [includeStakingInPnl, setIncludeStakingInPnl] = useState(true)
  const [marketSortKey, setMarketSortKey] = useState<MarketSortKey>('score')

  const topSymbols = useMemo(
    () => markets.slice(0, 20).map((m) => m.symbol),
    [markets],
  )

  useEffect(() => {
    const loadMarkets = async () => {
      try {
        const res = await fetch('/api/markets')
        if (!res.ok) {
          throw new Error(`api error (${res.status})`)
        }
        const data = (await res.json()) as unknown[]
        setMarkets(Array.isArray(data) ? data.map(normalizeMarket) : [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'failed to load markets')
      }
    }

    loadMarkets()
    const id = window.setInterval(loadMarkets, 60_000)
    return () => window.clearInterval(id)
  }, [])

  useEffect(() => {
    if (topSymbols.length === 0) {
      return
    }

    const loadLive = async () => {
      try {
        const params = new URLSearchParams({
          symbols: topSymbols.join(','),
          limit: String(topSymbols.length),
        })
        const res = await fetch(`/api/live-1m?${params.toString()}`)
        if (!res.ok) {
          throw new Error(`api error (${res.status})`)
        }
        const body = (await res.json()) as { candles?: unknown[] }
        const normalized = Array.isArray(body.candles)
          ? body.candles.map(normalizeLiveCandle)
          : []
        setLiveCandles(normalized)
        setLastRefresh(new Date().toISOString())
      } catch (err) {
        setError(err instanceof Error ? err.message : 'failed to load live data')
      }
    }

    loadLive()
    const id = window.setInterval(loadLive, 10_000)
    return () => window.clearInterval(id)
  }, [topSymbols])

  useEffect(() => {
    if (topSymbols.length === 0) {
      return
    }

    let cancelled = false

    const loadMiniCharts = async () => {
      const targets = topSymbols.slice(0, 20)
      const entries = await Promise.all(
        targets.map(async (symbol) => {
          try {
            const params = new URLSearchParams({
              symbol,
              limit: '16',
            })
            const res = await fetch(`/api/candles-5m?${params.toString()}`)
            if (!res.ok) {
              return [symbol, []] as const
            }
            const body = (await res.json()) as { candles?: unknown[] }
            const candles = Array.isArray(body.candles)
              ? body.candles.map(normalizeCandle)
              : []
            return [symbol, candles.map((candle) => candle.close)] as const
          } catch {
            return [symbol, []] as const
          }
        }),
      )

      if (!cancelled) {
        setMarketMiniCharts(Object.fromEntries(entries))
      }
    }

    loadMiniCharts()
    return () => {
      cancelled = true
    }
  }, [topSymbols])

  useEffect(() => {
    const loadHistory = async () => {
      try {
        const params = new URLSearchParams({
          symbol: selectedSymbol,
          limit: '500',
        })
        const res = await fetch(`/api/candles-5m?${params.toString()}`)
        if (!res.ok) {
          throw new Error(`api error (${res.status})`)
        }
        const body = (await res.json()) as { candles?: unknown[] }
        const normalized = Array.isArray(body.candles)
          ? body.candles.map(normalizeCandle)
          : []
        setHistory5m(normalized)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'failed to load 5m history')
      }
    }

    loadHistory()
  }, [selectedSymbol])

  useEffect(() => {
    setHoveredChartIndex(null)
  }, [selectedSymbol, chartWindow])

  useEffect(() => {
    if (activeTab !== 'tasks' && !isBacktesting) {
      return
    }

    let cancelled = false

    const loadBackfillJobs = async () => {
      setJobsLoading(true)
      try {
        const params = new URLSearchParams({ limit: '80' })
        const res = await fetch(`/api/backfill-jobs?${params.toString()}`)
        if (!res.ok) {
          throw new Error(`api error (${res.status})`)
        }
        const body = (await res.json()) as { jobs?: unknown[] }
        const jobs = Array.isArray(body.jobs) ? body.jobs.map(normalizeBackfillJob) : []
        if (!cancelled) {
          setBackfillJobs(jobs)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'failed to load backfill jobs')
        }
      } finally {
        if (!cancelled) {
          setJobsLoading(false)
        }
      }
    }

    loadBackfillJobs()
    const id = window.setInterval(loadBackfillJobs, 5000)
    return () => {
      cancelled = true
      window.clearInterval(id)
    }
  }, [activeTab, isBacktesting])

  useEffect(() => {
    if (activeTab !== 'heatmap') {
      return
    }

    let cancelled = false

    const loadActivity = async () => {
      setActivityLoading(true)
      try {
        const params = new URLSearchParams({
          days: '182',
          timeframe: '5m',
        })
        const res = await fetch(`/api/activity/daily-symbols?${params.toString()}`)
        if (!res.ok) {
          throw new Error(`api error (${res.status})`)
        }
        const body = (await res.json()) as { days?: unknown[] }
        const normalized = Array.isArray(body.days) ? body.days.map(normalizeDailySymbolActivity) : []
        if (!cancelled) {
          setDailyActivity(normalized)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'failed to load daily activity')
        }
      } finally {
        if (!cancelled) {
          setActivityLoading(false)
        }
      }
    }

    loadActivity()
    const id = window.setInterval(loadActivity, 20_000)
    return () => {
      cancelled = true
      window.clearInterval(id)
    }
  }, [activeTab])

  useEffect(() => {
    if (activeTab !== 'wallet') {
      return
    }

    let cancelled = false

    const loadWallet = async () => {
      setWalletLoading(true)
      try {
        const res = await fetch('/api/wallet')
        if (!res.ok) {
          throw new Error(`api error (${res.status})`)
        }
        const data = await res.json()
        if (!cancelled) {
          setWallet(normalizeWallet(data))
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'failed to load wallet')
        }
      } finally {
        if (!cancelled) {
          setWalletLoading(false)
        }
      }
    }

    loadWallet()
    const id = window.setInterval(loadWallet, 10_000)
    return () => {
      cancelled = true
      window.clearInterval(id)
    }
  }, [activeTab])

  const handleBacktest = async () => {
    setIsBacktesting(true)
    setBacktestMessage('')
    try {
      const now = new Date()
      const sevenDaysAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)

      const params = new URLSearchParams({
        symbol: selectedSymbol,
        from: sevenDaysAgo.toISOString(),
        to: now.toISOString(),
      })

      const res = await fetch(`/api/backtest-symbol?${params.toString()}`, {
        method: 'POST',
      })

      if (!res.ok) {
        throw new Error(`API error (${res.status})`)
      }

      const data = (await res.json()) as { job?: { id?: string } }
      const jobId = data.job?.id
      if (!jobId) {
        throw new Error('missing backfill job id')
      }

      setActiveTab('tasks')

      setBacktestMessage(`Backfill en cours pour ${selectedSymbol}... (job: ${jobId})`)

      let done = false
      for (let attempt = 0; attempt < 90; attempt++) {
        await new Promise((resolve) => window.setTimeout(resolve, 2000))
        const statusParams = new URLSearchParams({ jobId })
        const statusRes = await fetch(`/api/backfill-status?${statusParams.toString()}`)
        if (!statusRes.ok) {
          continue
        }
        const statusBody = (await statusRes.json()) as {
          job?: { state?: string; report?: { persisted5m?: number }; error?: string }
        }
        const state = statusBody.job?.state
        if (state === 'done') {
          const persisted = statusBody.job?.report?.persisted5m ?? 0
          setBacktestMessage(`✓ Backfill terminé pour ${selectedSymbol} (${persisted} bougies 5m persistées)`)
          done = true
          break
        }
        if (state === 'failed') {
          throw new Error(statusBody.job?.error || 'backfill failed')
        }
      }

      if (!done) {
        setBacktestMessage(`Backfill toujours en cours pour ${selectedSymbol}. Consultez le statut plus tard.`)
        return
      }

      // Recharger les données historiques
      const historyParams = new URLSearchParams({
        symbol: selectedSymbol,
        limit: '500',
      })
      const historyRes = await fetch(`/api/candles-5m?${historyParams.toString()}`)
      if (historyRes.ok) {
        const body = (await historyRes.json()) as { candles?: unknown[] }
        const normalized = Array.isArray(body.candles)
          ? body.candles.map(normalizeCandle)
          : []
        setHistory5m(normalized)
      }
    } catch (err) {
      setBacktestMessage(`✗ Backtesting failed: ${err instanceof Error ? err.message : 'Unknown error'}`)
    } finally {
      setIsBacktesting(false)
    }
  }

  const handleBackfillAllSymbols = async () => {
    setIsBackfillingAll(true)
    setTasksMessage('')
    try {
      const now = new Date()
      const sevenDaysAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
      const params = new URLSearchParams({
        from: sevenDaysAgo.toISOString(),
        to: now.toISOString(),
      })

      const res = await fetch(`/api/backfill-5m?${params.toString()}`, {
        method: 'POST',
      })
      if (!res.ok) {
        throw new Error(`API error (${res.status})`)
      }

      const body = (await res.json()) as { job?: unknown }
      const job = body.job ? normalizeBackfillJob(body.job) : undefined
      if (!job?.id) {
        throw new Error('missing backfill job id')
      }

      setBackfillJobs((prev) => [job, ...prev.filter((j) => j.id !== job.id)])
      setTasksMessage(`✓ Backfill global lancé (job: ${job.id})`)
    } catch (err) {
      setTasksMessage(`✗ Impossible de lancer le backfill global: ${err instanceof Error ? err.message : 'Unknown error'}`)
    } finally {
      setIsBackfillingAll(false)
    }
  }

  const selectedLive = liveCandles.find((c) => c.symbol === selectedSymbol)
  const selectedTrendValues = history5m.slice(-24).map((candle) => candle.close)
  const topMomentumValues = markets.slice(0, 20).map((market) => market.change24h)
  const selectedChartCandles = history5m.slice(-chartWindow)
  const activeChartIndex =
    hoveredChartIndex !== null && hoveredChartIndex >= 0 && hoveredChartIndex < selectedChartCandles.length
      ? hoveredChartIndex
      : selectedChartCandles.length - 1
  const activeChartCandle = selectedChartCandles[activeChartIndex]
  const selectedChartStart = selectedChartCandles[0]
  const selectedChartEnd = selectedChartCandles[selectedChartCandles.length - 1]
  const chartReturnPct =
    selectedChartStart && selectedChartEnd
      ? ((selectedChartEnd.close - selectedChartStart.open) / selectedChartStart.open) * 100
      : 0
  const chartHigh = selectedChartCandles.length
    ? Math.max(...selectedChartCandles.map((candle) => candle.high))
    : 0
  const chartLow = selectedChartCandles.length
    ? Math.min(...selectedChartCandles.map((candle) => candle.low))
    : 0
  const chartVolume = selectedChartCandles.reduce((total, candle) => total + candle.volume, 0)
  const runningJobs = backfillJobs.filter((job) => job.state === 'running' || job.state === 'queued')
  const completedJobs = backfillJobs.filter((job) => job.state === 'done')
  const failedJobs = backfillJobs.filter((job) => job.state === 'failed')
  const maxDailySymbolCount = dailyActivity.reduce((max, d) => Math.max(max, d.symbolCount), 0)
  const dailyRows = useMemo(() => {
    if (dailyActivity.length === 0) {
      return [] as DailySymbolActivity[][]
    }
    const rows: DailySymbolActivity[][] = []
    let index = 0
    while (index < dailyActivity.length) {
      rows.push(dailyActivity.slice(index, index + 7))
      index += 7
    }
    return rows
  }, [dailyActivity])
  const market24hBySymbol = useMemo(() => {
    const map = new Map<string, number>()
    for (const m of markets) {
      map.set(m.symbol.toUpperCase(), m.change24h)
    }
    return map
  }, [markets])
  const liveMiniChartsBySymbol = useMemo(() => {
    const map = new Map<string, number[]>()
    for (const candle of liveCandles) {
      const values = [candle.open, candle.high, candle.low, candle.close].filter((value) =>
        Number.isFinite(value),
      )
      if (values.length > 0) {
        map.set(candle.symbol.toUpperCase(), values)
      }
    }
    return map
  }, [liveCandles])
  const walletPnlMetrics = useMemo(() => {
    const metricsBySymbol = new Map<string, { effectiveValue: number; effectiveCostBasis: number; pnlValue: number; pnlPercent: number }>()

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
      metricsBySymbol.set(asset.symbol.toUpperCase(), {
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
      bySymbol: metricsBySymbol,
      effectiveTotalValue,
      globalPnlValue,
      globalPnlPercent,
    }
  }, [wallet.assets, wallet.netDepositsValue, includeStakingInPnl])
  const walletPnl24h = useMemo(() => {
    const pnlValue = wallet.assets.reduce((sum, asset) => {
      const metrics = walletPnlMetrics.bySymbol.get(asset.symbol.toUpperCase())
      if (!metrics) {
        return sum
      }
      const symbol = asset.symbol.toUpperCase()
      const trend24h = market24hBySymbol.get(symbol)
      if (trend24h === undefined || !Number.isFinite(trend24h)) {
        return sum
      }

      const ratio = trend24h / 100
      const denom = 1 + ratio
      if (denom <= 0) {
        return sum
      }

      const previousValue = metrics.effectiveValue / denom
      return sum + (metrics.effectiveValue - previousValue)
    }, 0)

    const previousTotal = walletPnlMetrics.effectiveTotalValue - pnlValue
    const pnlPercent = previousTotal > 0 ? (pnlValue / previousTotal) * 100 : 0

    return { value: pnlValue, percent: pnlPercent }
  }, [wallet.assets, market24hBySymbol, walletPnlMetrics])
  const visibleWalletAssets = useMemo(
    () => wallet.assets.filter((asset) => asset.value >= 0.01),
    [wallet.assets],
  )
  const sortedMarkets = useMemo(() => {
    const cloned = [...markets]
    cloned.sort((left, right) => {
      switch (marketSortKey) {
        case 'change24h':
          return right.change24h - left.change24h
        case 'change1h':
          return right.change1h - left.change1h
        case 'change5m':
          return right.change5m - left.change5m
        case 'quoteVolume':
          return right.quoteVolume - left.quoteVolume
        case 'score':
        default:
          return right.qualityScore - left.qualityScore
      }
    })
    return cloned
  }, [markets, marketSortKey])

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Karasu Live Feed</p>
          <h1>1m live cache + 5m database</h1>
        </div>
        <div className="status-group">
          <label htmlFor="symbol">Symbol</label>
          <select
            id="symbol"
            value={selectedSymbol}
            onChange={(e) => setSelectedSymbol(e.target.value)}
          >
            {topSymbols.map((symbol) => (
              <option key={symbol} value={symbol}>
                {symbol}
              </option>
            ))}
          </select>
        </div>
      </header>

      <nav className="tabs" aria-label="Navigation principale">
        <button
          type="button"
          className={activeTab === 'overview' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('overview')}
        >
          Vue d&apos;ensemble
        </button>
        <button
          type="button"
          className={activeTab === 'markets' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('markets')}
        >
          Marchés
        </button>
        <button
          type="button"
          className={activeTab === 'backtest' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('backtest')}
        >
          Backtesting
        </button>
        <button
          type="button"
          className={activeTab === 'tasks' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('tasks')}
        >
          Tâches
        </button>
        <button
          type="button"
          className={activeTab === 'heatmap' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('heatmap')}
        >
          Heatmap
        </button>
        <button
          type="button"
          className={activeTab === 'wallet' ? 'tab-button active' : 'tab-button'}
          onClick={() => setActiveTab('wallet')}
        >
          Wallet
        </button>
      </nav>

      {activeTab === 'overview' && (
      <>
      <section className="kpi-grid">
        <article className="card">
          <p>Live close (1m)</p>
          <h2>{selectedLive ? formatNumber(selectedLive.close, 4) : '-'}</h2>
        </article>
        <article className="card">
          <p>Live volume (1m)</p>
          <h2>{selectedLive ? formatNumber(selectedLive.volume, 2) : '-'}</h2>
        </article>
        <article className="card">
          <p>Candles stored (5m)</p>
          <h2>{history5m.length}</h2>
        </article>
        <article className="card">
          <p>Last refresh</p>
          <h2>{lastRefresh ? new Date(lastRefresh).toLocaleTimeString() : '-'}</h2>
        </article>
      </section>

      <section className="mini-charts">
        <article className="panel chart-panel">
          <div className="chart-head">
            <div>
              <h3>{selectedSymbol} trend</h3>
              <p>5m closes on the selected symbol</p>
            </div>
            <span>{selectedTrendValues.length} points</span>
          </div>
          <Sparkline
            values={selectedTrendValues}
            stroke="#2563eb"
            fill="rgba(37, 99, 235, 0.12)"
            label={`${selectedSymbol} 5m trend`}
          />
        </article>

        <article className="panel chart-panel">
          <div className="chart-head">
            <div>
              <h3>Top market momentum</h3>
              <p>24h change for the current top list</p>
            </div>
            <span>{topMomentumValues.length} points</span>
          </div>
          <Sparkline
            values={topMomentumValues}
            stroke="#dc2626"
            fill="rgba(220, 38, 38, 0.12)"
            label="Top market momentum"
          />
        </article>
      </section>
      </>
      )}

      {error && <p className="error">{error}</p>}

      {activeTab === 'markets' && (
      <section className="panel-grid">
        <article className="panel">
          <div className="markets-head">
            <h3>Top markets</h3>
            <label className="markets-sort">
              <span>Trier par</span>
              <select
                value={marketSortKey}
                onChange={(event) => setMarketSortKey(event.target.value as MarketSortKey)}
              >
                <option value="score">Score</option>
                <option value="change24h">24h %</option>
                <option value="change1h">1h %</option>
                <option value="change5m">5m %</option>
                <option value="quoteVolume">Volume</option>
              </select>
            </label>
          </div>
          <div className="markets-legend" aria-label="Légende des scores">
            <div className="markets-legend-levels">
              <span className="market-score-pill excellent">75-100 Excellent</span>
              <span className="market-score-pill good">60-74 Bon</span>
              <span className="market-score-pill neutral">40-59 Neutre</span>
              <span className="market-score-pill weak">0-39 Faible</span>
            </div>
            <div className="markets-legend-details">
              <span><strong>RSI</strong>: momentum relatif</span>
              <span><strong>MACD</strong>: tendance + accélération</span>
              <span><strong>BB</strong>: position dans les Bollinger bands</span>
              <span><strong>Vol</strong>: volume vs moyenne</span>
              <span><strong>SMA</strong>: tendance de fond moyennes mobiles</span>
            </div>
          </div>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Symbol</th>
                  <th>Score</th>
                  <th>24h %</th>
                  <th>1h %</th>
                  <th>5m %</th>
                  <th>Trend</th>
                </tr>
              </thead>
              <tbody>
                {sortedMarkets.slice(0, 20).map((market) => {
                  const sparklineValues =
                    marketMiniCharts[market.symbol] && marketMiniCharts[market.symbol].length > 0
                      ? marketMiniCharts[market.symbol]
                      : liveMiniChartsBySymbol.get(market.symbol.toUpperCase()) ?? []

                  return (
                  <tr key={market.symbol}>
                    <td>{market.symbol}</td>
                    <td>
                      <div className="market-score-stack">
                        <span
                          className={
                            market.qualityScore >= 75
                              ? 'market-score-pill excellent'
                              : market.qualityScore >= 60
                                ? 'market-score-pill good'
                                : market.qualityScore >= 40
                                  ? 'market-score-pill neutral'
                                  : 'market-score-pill weak'
                          }
                        >
                          {formatNumber(market.qualityScore, 1)}
                        </span>
                        <div className="market-score-details">
                          <span>RSI {formatNumber(market.qualityRsi, 0)}</span>
                          <span>MACD {formatNumber(market.qualityMacd, 0)}</span>
                          <span>BB {formatNumber(market.qualityBollinger, 0)}</span>
                          <span>Vol {formatNumber(market.qualityVolume, 0)}</span>
                          <span>SMA {formatNumber(market.qualitySma, 0)}</span>
                        </div>
                      </div>
                    </td>
                    <td>{formatNumber(market.change24h, 2)}</td>
                    <td>{formatNumber(market.change1h, 2)}</td>
                    <td>{formatNumber(market.change5m, 2)}</td>
                    <td className="market-trend-cell">
                      <Sparkline
                        values={sparklineValues}
                        stroke="#0f766e"
                        fill="rgba(15, 118, 110, 0.12)"
                        height={36}
                        label={`${market.symbol} mini trend`}
                      />
                    </td>
                  </tr>
                )})}
              </tbody>
            </table>
          </div>
        </article>

        <article className="panel">
          <h3>{selectedSymbol} history (5m)</h3>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Open time</th>
                  <th>Open</th>
                  <th>High</th>
                  <th>Low</th>
                  <th>Close</th>
                </tr>
              </thead>
              <tbody>
                {history5m.slice(-30).reverse().map((candle) => (
                  <tr key={`${selectedSymbol}-${candle.openTime}`}>
                    <td>{new Date(candle.openTime).toLocaleString()}</td>
                    <td>{formatNumber(candle.open, 4)}</td>
                    <td>{formatNumber(candle.high, 4)}</td>
                    <td>{formatNumber(candle.low, 4)}</td>
                    <td>{formatNumber(candle.close, 4)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </article>
      </section>
      )}

      {activeTab === 'backtest' && (
      <section className="panel backtest-panel">
        <div className="backtest-head">
          <div>
            <h3>{selectedSymbol} backtesting view</h3>
            <p>
              Candles closes from the persisted 5m history, with hover inspection and quick zoom
              ranges.
            </p>
          </div>
          <div className="backtest-controls">
            {chartWindows.map((window) => (
              <button
                key={window.candles}
                type="button"
                className={window.candles === chartWindow ? 'range-button active' : 'range-button'}
                onClick={() => setChartWindow(window.candles)}
              >
                {window.label}
              </button>
            ))}
            <button
              type="button"
              className="backtest-button"
              onClick={handleBacktest}
              disabled={isBacktesting}
            >
              {isBacktesting ? 'Backtesting...' : '⟳ Backfill last 7d'}
            </button>
          </div>
        </div>

        {backtestMessage && (
          <div className={`backtest-message ${backtestMessage.startsWith('✓') ? 'success' : 'error'}`}>
            {backtestMessage}
          </div>
        )}

        <div className="backtest-stats">
          <div>
            <span>Range return</span>
            <strong>{formatNumber(chartReturnPct, 2)}%</strong>
          </div>
          <div>
            <span>Range high</span>
            <strong>{formatNumber(chartHigh, 2)}</strong>
          </div>
          <div>
            <span>Range low</span>
            <strong>{formatNumber(chartLow, 2)}</strong>
          </div>
          <div>
            <span>Range volume</span>
            <strong>{formatNumber(chartVolume, 2)}</strong>
          </div>
        </div>

        <CandlestickChart
          candles={selectedChartCandles}
          hoveredIndex={hoveredChartIndex}
          onHoverIndexChange={setHoveredChartIndex}
        />

        <div className="backtest-footer">
          <div>
            <span>From</span>
            <strong>
              {selectedChartStart ? new Date(selectedChartStart.openTime).toLocaleString() : '-'}
            </strong>
          </div>
          <div>
            <span>To</span>
            <strong>
              {selectedChartEnd ? new Date(selectedChartEnd.openTime).toLocaleString() : '-'}
            </strong>
          </div>
          <div>
            <span>Selected candle</span>
            <strong>
              {activeChartCandle ? new Date(activeChartCandle.openTime).toLocaleString() : '-'}
            </strong>
          </div>
        </div>
      </section>
      )}

      {activeTab === 'tasks' && (
      <section className="panel tasks-panel">
        <div className="tasks-head">
          <div>
            <h3>Tâches backfill</h3>
            <p>Jobs en cours, terminés, en erreur avec durées et détails d&apos;exécution.</p>
          </div>
          <div className="tasks-head-actions">
            <span>{jobsLoading ? 'Actualisation...' : `${backfillJobs.length} tâches`}</span>
            <button
              type="button"
              className="backfill-all-button"
              onClick={handleBackfillAllSymbols}
              disabled={isBackfillingAll}
            >
              {isBackfillingAll ? 'Lancement...' : '⟳ Backfill tous les symbols (7j)'}
            </button>
          </div>
        </div>

        {tasksMessage && (
          <div className={`backtest-message ${tasksMessage.startsWith('✓') ? 'success' : 'error'}`}>
            {tasksMessage}
          </div>
        )}

        <div className="tasks-kpis">
          <div>
            <span>En cours</span>
            <strong>{runningJobs.length}</strong>
          </div>
          <div>
            <span>Terminées</span>
            <strong>{completedJobs.length}</strong>
          </div>
          <div>
            <span>En erreur</span>
            <strong>{failedJobs.length}</strong>
          </div>
        </div>

        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Job</th>
                <th>État</th>
                <th>Symboles</th>
                <th>Créée</th>
                <th>Début</th>
                <th>Fin</th>
                <th>Durée</th>
                <th>Détails</th>
              </tr>
            </thead>
            <tbody>
              {backfillJobs.map((job) => {
                const startedMs = job.startedAt ? Date.parse(job.startedAt) : 0
                const endedMs = job.endedAt ? Date.parse(job.endedAt) : 0
                const durationMs =
                  job.report?.durationMs ??
                  (startedMs > 0 ? (endedMs > 0 ? endedMs - startedMs : Date.now() - startedMs) : 0)

                return (
                  <tr key={job.id}>
                    <td>{job.id}</td>
                    <td>
                      <span className={`job-state ${job.state}`}>{job.state}</span>
                    </td>
                    <td>{job.symbols.join(', ') || '-'}</td>
                    <td>{job.createdAt ? new Date(job.createdAt).toLocaleString() : '-'}</td>
                    <td>{job.startedAt ? new Date(job.startedAt).toLocaleString() : '-'}</td>
                    <td>{job.endedAt ? new Date(job.endedAt).toLocaleString() : '-'}</td>
                    <td>{formatDurationMs(durationMs)}</td>
                    <td className="task-details-cell">
                      {job.error
                        ? `Erreur: ${job.error}`
                        : job.report
                          ? `5m persistées: ${job.report.persisted5m} | 1m fetch: ${job.report.fetched1mCandles} | chunks: ${job.report.chunks}`
                          : job.reason || '-'}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </section>
      )}

      {activeTab === 'wallet' && (
      <section className="panel wallet-panel">
        <div className="wallet-head">
          <div>
            <h3>Wallet</h3>
            <p>Portfolio et soldes des positions.</p>
          </div>
          <div className="wallet-head-actions">
            <button
              type="button"
              className="wallet-toggle-button"
              onClick={() => setIncludeStakingInPnl((prev) => !prev)}
            >
              {includeStakingInPnl
                ? 'PnL: staking inclus'
                : 'PnL: staking ignoré'}
            </button>
            <span>{walletLoading ? 'Actualisation...' : 'À jour'}</span>
          </div>
        </div>
        {visibleWalletAssets.length === 0 ? (
          <div className="wallet-empty">
            <p>Pas de données de wallet pour le moment.</p>
          </div>
        ) : (
          <div className="wallet-content">
            <div className="wallet-summary">
              <div className="wallet-card">
                <span>Valeur totale</span>
                <h2>{formatNumber(wallet.totalValue, 2)} EUR</h2>
              </div>
              <div className="wallet-card">
                <span>Valeur en espèces</span>
                <h2>{formatNumber(wallet.cashValue, 2)} EUR</h2>
              </div>
              <div className="wallet-card">
                <span>Valeur des actifs</span>
                <h2>{formatNumber(wallet.assetValue, 2)} EUR</h2>
              </div>
              <div className="wallet-card">
                <span>Dépôts nets</span>
                <h2>{formatNumber(wallet.netDepositsValue, 2)} EUR</h2>
              </div>
              <div className="wallet-card">
                <span>PnL</span>
                <h2 className={walletPnlMetrics.globalPnlValue >= 0 ? 'wallet-positive' : 'wallet-negative'}>
                  {walletPnlMetrics.globalPnlValue >= 0 ? '+' : ''}
                  {formatNumber(walletPnlMetrics.globalPnlValue, 2)} EUR ({walletPnlMetrics.globalPnlPercent >= 0 ? '+' : ''}
                  {formatNumber(walletPnlMetrics.globalPnlPercent, 2)}%)
                </h2>
              </div>
              <div className="wallet-card">
                <span>PnL 24h</span>
                <h2 className={walletPnl24h.value >= 0 ? 'wallet-positive' : 'wallet-negative'}>
                  {walletPnl24h.value >= 0 ? '+' : ''}
                  {formatNumber(walletPnl24h.value, 2)} EUR ({walletPnl24h.percent >= 0 ? '+' : ''}
                  {formatNumber(walletPnl24h.percent, 2)}%)
                </h2>
              </div>
            </div>
            <div className="wallet-assets">
              <h4>Actifs</h4>
              <table>
                <thead>
                  <tr>
                    <th>Symbol</th>
                    <th>Montant</th>
                    <th>En ordre</th>
                    <th>Staking</th>
                    <th>Tendance 24h</th>
                    <th>Coût (EUR)</th>
                    <th>PnL</th>
                    <th>Valeur (EUR)</th>
                  </tr>
                </thead>
                <tbody>
                  {visibleWalletAssets.map((asset) => {
                    const trend24h = market24hBySymbol.get(asset.symbol.toUpperCase())
                    const pnlMetrics = walletPnlMetrics.bySymbol.get(asset.symbol.toUpperCase())
                    const assetPnlValue = pnlMetrics?.pnlValue ?? asset.pnlValue
                    const assetPnlPercent = pnlMetrics?.pnlPercent ?? asset.pnlPercent
                    return (
                      <tr key={asset.symbol}>
                        <td>{asset.symbol}</td>
                        <td>{formatNumber(asset.amount, 8)}</td>
                        <td>{formatNumber(asset.inOrder, 8)}</td>
                        <td>{formatNumber(asset.stakingAmount, 8)}</td>
                        <td className={trend24h === undefined ? '' : trend24h >= 0 ? 'wallet-positive' : 'wallet-negative'}>
                          {trend24h === undefined ? '-' : `${trend24h >= 0 ? '+' : ''}${formatNumber(trend24h, 2)}%`}
                        </td>
                        <td>{formatNumber(asset.costBasisValue, 2)}</td>
                        <td className={assetPnlValue >= 0 ? 'wallet-positive' : 'wallet-negative'}>
                          {assetPnlValue >= 0 ? '+' : ''}
                          {formatNumber(assetPnlValue, 2)} EUR ({assetPnlPercent >= 0 ? '+' : ''}
                          {formatNumber(assetPnlPercent, 2)}%)
                        </td>
                        <td>{formatNumber(asset.value, 2)}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </section>
      )}

      {activeTab === 'heatmap' && (
      <section className="panel heatmap-panel">
        <div className="heatmap-head">
          <div>
            <h3>Activité quotidienne des symbols</h3>
            <p>
              Chaque carré représente un jour. La couleur dépend du nombre de symbols avec des
              données 5m ce jour-là.
            </p>
          </div>
          <span>{activityLoading ? 'Actualisation...' : `${dailyActivity.length} jours`}</span>
        </div>

        <div className="heatmap-wrap" role="img" aria-label="Heatmap activité quotidienne">
          {dailyRows.map((week, weekIndex) => (
            <div key={`week-${weekIndex}`} className="heatmap-week">
              {week.map((day) => {
                const ratio = maxDailySymbolCount > 0 ? day.symbolCount / maxDailySymbolCount : 0
                const level = day.symbolCount === 0 ? 0 : ratio < 0.25 ? 1 : ratio < 0.5 ? 2 : ratio < 0.75 ? 3 : 4
                return (
                  <div
                    key={day.day}
                    className={`heatmap-cell level-${level}`}
                    title={`${new Date(day.day).toLocaleDateString()} — ${day.symbolCount} symbols, ${day.candleCount} bougies`}
                  />
                )
              })}
            </div>
          ))}
        </div>

        <div className="heatmap-legend">
          <span>Moins</span>
          <div className="heatmap-cell level-0" />
          <div className="heatmap-cell level-1" />
          <div className="heatmap-cell level-2" />
          <div className="heatmap-cell level-3" />
          <div className="heatmap-cell level-4" />
          <span>Plus</span>
        </div>
      </section>
      )}
    </main>
  )
}

export default App
