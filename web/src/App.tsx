import { useEffect, useMemo, useState, type MouseEvent } from 'react'
import './App.css'

type Market = {
  symbol: string
  quoteVolume: number
  quoteVolumePosition: number
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

type AppTab = 'overview' | 'markets' | 'backtest'

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
      const targets = topSymbols.slice(0, 8)
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
          <h3>Top markets</h3>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Symbol</th>
                  <th>24h %</th>
                  <th>1h %</th>
                  <th>5m %</th>
                  <th>Trend</th>
                </tr>
              </thead>
              <tbody>
                {markets.slice(0, 20).map((market) => (
                  <tr key={market.symbol}>
                    <td>{market.symbol}</td>
                    <td>{formatNumber(market.change24h, 2)}</td>
                    <td>{formatNumber(market.change1h, 2)}</td>
                    <td>{formatNumber(market.change5m, 2)}</td>
                    <td className="market-trend-cell">
                      <Sparkline
                        values={marketMiniCharts[market.symbol] ?? []}
                        stroke="#0f766e"
                        fill="rgba(15, 118, 110, 0.12)"
                        height={36}
                        label={`${market.symbol} mini trend`}
                      />
                    </td>
                  </tr>
                ))}
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
    </main>
  )
}

export default App
