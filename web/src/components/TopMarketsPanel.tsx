import { useCallback, useState } from 'react'
import { usePolling } from '../hooks/usePolling'
import { type Market, type MarketAnalysis, type MarketSignalHistory, type MarketSortKey, type Opportunity } from '../models/market'
import { getMarketAnalysis, getMarketSignalHistory, getMarkets, getOpportunities } from '../services/marketService'
import { MarketAnalysisSection } from './MarketAnalysisSection'
import { MarketsTableSection } from './MarketsTableSection'
import { ScannerSection } from './ScannerSection'

export function TopMarketsPanel() {
  const [opportunities, setOpportunities] = useState<Opportunity[]>([])
  const [opportunitiesLoading, setOpportunitiesLoading] = useState(false)
  const [opportunitiesError, setOpportunitiesError] = useState('')
  const [markets, setMarkets] = useState<Market[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [sortKey, setSortKey] = useState<MarketSortKey>('priority')
  const [selectedSymbol, setSelectedSymbol] = useState('')
  const [analysis, setAnalysis] = useState<MarketAnalysis | null>(null)
  const [analysisLoading, setAnalysisLoading] = useState(false)
  const [analysisError, setAnalysisError] = useState('')
  const [signalHistory, setSignalHistory] = useState<MarketSignalHistory | null>(null)
  const [signalHistoryLoading, setSignalHistoryLoading] = useState(false)
  const [signalHistoryError, setSignalHistoryError] = useState('')
  const [strategyProfileFilter, setStrategyProfileFilter] = useState('all')
  const [strategyStateFilter, setStrategyStateFilter] = useState('all')
  const [freshOnly, setFreshOnly] = useState(false)
  const [consensusOnly, setConsensusOnly] = useState(false)
  const [priorityBandFilter, setPriorityBandFilter] = useState('all')

  const loadOpportunities = useCallback(async () => {
    setOpportunitiesLoading(true)
    try {
      const nextOpportunities = await getOpportunities(18)
      setOpportunities(nextOpportunities)
      setOpportunitiesError('')
    } catch (err) {
      setOpportunitiesError(err instanceof Error ? err.message : 'echec du chargement des opportunites')
    } finally {
      setOpportunitiesLoading(false)
    }
  }, [])

  usePolling(loadOpportunities, 60_000)

  const loadMarkets = useCallback(async () => {
    setLoading(true)
    try {
      const nextMarkets = await getMarkets()
      setMarkets(nextMarkets)
      setSelectedSymbol((current) => {
        if (current !== '' && nextMarkets.some((market) => market.symbol === current)) {
          return current
        }
        return nextMarkets[0]?.symbol ?? ''
      })
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'echec du chargement des marches')
    } finally {
      setLoading(false)
    }
  }, [])

  usePolling(loadMarkets, 60_000)

  const loadAnalysis = useCallback(async () => {
    if (selectedSymbol === '') {
      setAnalysis(null)
      return
    }

    setAnalysisLoading(true)
    try {
      const nextAnalysis = await getMarketAnalysis(selectedSymbol)
      setAnalysis(nextAnalysis)
      setAnalysisError('')
    } catch (err) {
      setAnalysisError(err instanceof Error ? err.message : 'echec du chargement de l analyse du marche')
    } finally {
      setAnalysisLoading(false)
    }
  }, [selectedSymbol])

  usePolling(loadAnalysis, 60_000, selectedSymbol !== '')

  const loadSignalHistory = useCallback(async () => {
    if (selectedSymbol === '') {
      setSignalHistory(null)
      return
    }

    setSignalHistoryLoading(true)
    try {
      const nextHistory = await getMarketSignalHistory(selectedSymbol, 24)
      setSignalHistory(nextHistory)
      setSignalHistoryError('')
    } catch (err) {
      setSignalHistoryError(err instanceof Error ? err.message : 'echec du chargement de l historique des signaux')
    } finally {
      setSignalHistoryLoading(false)
    }
  }, [selectedSymbol])

  usePolling(loadSignalHistory, 60_000, selectedSymbol !== '')

  return (
    <section className="panel">
      {error && opportunitiesError && (
        <div className="degraded-banner">
          Connexion au backend impossible — les données affichées peuvent être obsolètes ou indisponibles.
        </div>
      )}
      <ScannerSection
        opportunities={opportunities}
        loading={opportunitiesLoading}
        error={opportunitiesError}
        priorityBandFilter={priorityBandFilter}
        freshOnly={freshOnly}
        consensusOnly={consensusOnly}
        onPriorityBandFilterChange={setPriorityBandFilter}
        onFreshOnlyChange={setFreshOnly}
        onConsensusOnlyChange={setConsensusOnly}
        onSelectSymbol={setSelectedSymbol}
      />
      <MarketsTableSection
        markets={markets}
        opportunities={opportunities}
        loading={loading}
        error={error}
        sortKey={sortKey}
        strategyProfileFilter={strategyProfileFilter}
        strategyStateFilter={strategyStateFilter}
        selectedSymbol={selectedSymbol}
        onSortKeyChange={setSortKey}
        onStrategyProfileFilterChange={setStrategyProfileFilter}
        onStrategyStateFilterChange={setStrategyStateFilter}
        onSelectSymbol={setSelectedSymbol}
      />
      <MarketAnalysisSection
        selectedSymbol={selectedSymbol}
        analysis={analysis}
        analysisLoading={analysisLoading}
        analysisError={analysisError}
        signalHistory={signalHistory}
        signalHistoryLoading={signalHistoryLoading}
        signalHistoryError={signalHistoryError}
      />
    </section>
  )
}

