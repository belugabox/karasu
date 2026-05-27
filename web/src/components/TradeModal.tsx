import { useEffect, useRef, useState } from 'react'
import { placeMarketOrder, type OrderResult } from '../services/orderService'
import { formatNumber } from '../utils/format'

type TradeModalProps = {
  symbol: string
  side: 'buy' | 'sell'
  suggestedAmountEur?: number
  onClose: () => void
}

export function TradeModal({ symbol, side, suggestedAmountEur, onClose }: TradeModalProps) {
  const [amountEur, setAmountEur] = useState(suggestedAmountEur ? String(suggestedAmountEur.toFixed(2)) : '')
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<OrderResult | null>(null)
  const [error, setError] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const isBuy = side === 'buy'
  const title = isBuy ? `Acheter ${symbol}` : `Vendre ${symbol}`

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    const amount = parseFloat(amountEur)
    if (!isFinite(amount) || amount <= 0) {
      setError('Montant invalide')
      return
    }

    setSubmitting(true)
    setError('')
    try {
      const order = await placeMarketOrder({ symbol, side, amountEur: amount })
      setResult(order)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erreur lors du passage de l'ordre")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="trade-modal-backdrop" onClick={onClose} role="dialog" aria-modal="true" aria-label={title}>
      <div className="trade-modal" onClick={(e) => e.stopPropagation()}>
        <div className="trade-modal-head">
          <h3>{title}</h3>
          <button type="button" className="trade-modal-close" onClick={onClose} aria-label="Fermer">✕</button>
        </div>

        {result ? (
          <div>
            <p className="success-text">
              Ordre placé avec succès — ID : {result.orderID || '—'} · Statut : {result.status}
            </p>
            <button type="button" className="action-button" onClick={onClose}>Fermer</button>
          </div>
        ) : (
          <form onSubmit={handleSubmit}>
            <p className="trade-modal-desc">
              {isBuy
                ? `Saisir le montant en EUR à dépenser pour acheter ${symbol} au prix du marché.`
                : `Saisir le montant en EUR de ${symbol} à vendre au prix du marché.`}
            </p>
            <label className="trade-modal-label" htmlFor="trade-amount">Montant (EUR)</label>
            <input
              ref={inputRef}
              id="trade-amount"
              className="trade-modal-input"
              type="number"
              min="1"
              step="0.01"
              value={amountEur}
              onChange={(e) => setAmountEur(e.target.value)}
              placeholder="ex: 100.00"
              disabled={submitting}
            />
            {suggestedAmountEur !== undefined && suggestedAmountEur > 0 && (
              <p className="trade-modal-hint">
                Valeur actuelle de la position : {formatNumber(suggestedAmountEur, 2)} EUR
              </p>
            )}
            {error && <p className="error-text" style={{ marginTop: '8px' }}>{error}</p>}
            <div className="trade-modal-actions">
              <button type="button" className="action-button" onClick={onClose} disabled={submitting}>
                Annuler
              </button>
              <button
                type="submit"
                className={`action-button trade-modal-submit ${isBuy ? 'trade-buy' : 'trade-sell'}`}
                disabled={submitting}
              >
                {submitting ? 'En cours...' : isBuy ? `Acheter ${symbol}` : `Vendre ${symbol}`}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
