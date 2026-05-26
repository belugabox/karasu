import { emptyWallet, normalizeWallet, type Wallet } from '../models/wallet'
import { fetchJson } from './http'

export async function getWallet(): Promise<Wallet> {
  const data = await fetchJson<unknown>('/api/wallet')
  return normalizeWallet(data ?? emptyWallet)
}
