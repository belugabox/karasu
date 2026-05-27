import { fetchJson } from './http'

export type OrderRequest = {
  symbol: string
  side: 'buy' | 'sell'
  amountEur: number
}

export type OrderResult = {
  orderID: string
  market: string
  side: string
  status: string
}

export async function placeMarketOrder(req: OrderRequest): Promise<OrderResult> {
  return fetchJson<OrderResult>('/api/orders', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
}
