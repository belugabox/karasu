import { toNumber, toStringValue } from '../utils/parse'

export type WalletAsset = {
  symbol: string
  amount: number
  inOrder: number
  stakingAmount: number
  costBasisValue: number
  pnlValue: number
  pnlPercent: number
  value: number
}

export type Wallet = {
  totalValue: number
  cashValue: number
  assetValue: number
  netDepositsValue: number
  pnlValue: number
  pnlPercent: number
  assets: WalletAsset[]
}

export const emptyWallet: Wallet = {
  totalValue: 0,
  cashValue: 0,
  assetValue: 0,
  netDepositsValue: 0,
  pnlValue: 0,
  pnlPercent: 0,
  assets: [],
}

export function normalizeWalletAsset(raw: unknown): WalletAsset {
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

export function normalizeWallet(raw: unknown): Wallet {
  const r = (raw as Record<string, unknown>) || {}
  const assetsRaw = r.assets ?? r.Assets

  const assets = Array.isArray(assetsRaw) ? assetsRaw.map(normalizeWalletAsset) : []

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
