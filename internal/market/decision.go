package market

import (
	"sort"
	"strings"

	"karasu/internal/exchange"
)

type WalletDecisionItem struct {
	Symbol string
	Value  float64
	Reason string
}

type WalletDecision struct {
	Reduce      []WalletDecisionItem
	Watch       []WalletDecisionItem
	Reinforce   []WalletDecisionItem
	SellNowCount int
	HasSellSignal bool
}

func BuildWalletDecision(wallet exchange.Wallet, opportunities []Opportunity) WalletDecision {
	heldOpportunityMap := make(map[string]Opportunity, len(opportunities))
	for _, opportunity := range opportunities {
		heldOpportunityMap[strings.ToUpper(opportunity.Symbol)] = opportunity
	}

	reduce := make([]WalletDecisionItem, 0)
	watch := make([]WalletDecisionItem, 0)
	reinforce := make([]WalletDecisionItem, 0)

	for _, asset := range wallet.Assets {
		if asset.Value < 0.01 {
			continue
		}

		symbol := strings.ToUpper(asset.Symbol)
		opportunity, ok := heldOpportunityMap[symbol]
		if !ok {
			continue
		}

		isDefensive := opportunity.PriorityBand == "defensive" ||
			opportunity.Leader.State == "exit" ||
			opportunity.Leader.State == "avoid" ||
			opportunity.Freshness.HasFreshExit

		isActionable := opportunity.PriorityBand == "actionable" &&
			(opportunity.Leader.State == "entry" || opportunity.Leader.State == "hold")

		if isDefensive {
			reduce = append(reduce, WalletDecisionItem{
				Symbol: symbol,
				Value:  asset.Value,
				Reason: "profil leader " + opportunity.Leader.State + ", priorite " + opportunity.PriorityBand,
			})
			continue
		}

		if isActionable && opportunity.Freshness.HasFreshEntry {
			reinforce = append(reinforce, WalletDecisionItem{
				Symbol: symbol,
				Value:  asset.Value,
				Reason: "entree fraiche et priorite actionnable",
			})
			continue
		}

		watch = append(watch, WalletDecisionItem{
			Symbol: symbol,
			Value:  asset.Value,
			Reason: "etat " + opportunity.Leader.State + ", priorite " + opportunity.PriorityBand,
		})
	}

	sortByValueDesc := func(left, right WalletDecisionItem) int {
		switch {
		case left.Value > right.Value:
			return -1
		case left.Value < right.Value:
			return 1
		default:
			return strings.Compare(left.Symbol, right.Symbol)
		}
	}

	sort.SliceStable(reduce, func(i, j int) bool { return sortByValueDesc(reduce[i], reduce[j]) < 0 })
	sort.SliceStable(watch, func(i, j int) bool { return sortByValueDesc(watch[i], watch[j]) < 0 })
	sort.SliceStable(reinforce, func(i, j int) bool { return sortByValueDesc(reinforce[i], reinforce[j]) < 0 })

	return WalletDecision{
		Reduce:        reduce,
		Watch:         watch,
		Reinforce:     reinforce,
		SellNowCount:  len(reduce),
		HasSellSignal: len(reduce) > 0,
	}
}
