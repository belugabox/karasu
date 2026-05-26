package market

import (
	"testing"

	"karasu/internal/exchange"
)

func TestBuildWalletDecisionClassifiesPositions(t *testing.T) {
	t.Parallel()

	wallet := exchange.Wallet{
		Assets: []exchange.WalletAsset{
			{Symbol: "BTC", Value: 1200},
			{Symbol: "ETH", Value: 800},
			{Symbol: "SOL", Value: 500},
		},
	}

	opportunities := []Opportunity{
		{
			Symbol:       "BTC",
			PriorityBand: "defensive",
			Leader:       StrategyEvaluation{State: "exit"},
			Freshness:    OpportunityFreshness{HasFreshExit: true},
		},
		{
			Symbol:       "ETH",
			PriorityBand: "actionable",
			Leader:       StrategyEvaluation{State: "entry"},
			Freshness:    OpportunityFreshness{HasFreshEntry: true},
		},
		{
			Symbol:       "SOL",
			PriorityBand: "watchlist",
			Leader:       StrategyEvaluation{State: "watch"},
		},
	}

	decision := BuildWalletDecision(wallet, opportunities)

	if !decision.HasSellSignal || decision.SellNowCount != 1 {
		t.Fatalf("expected one sell signal, got %#v", decision)
	}
	if len(decision.Reduce) != 1 || decision.Reduce[0].Symbol != "BTC" {
		t.Fatalf("expected BTC in reduce list, got %#v", decision.Reduce)
	}
	if len(decision.Reinforce) != 1 || decision.Reinforce[0].Symbol != "ETH" {
		t.Fatalf("expected ETH in reinforce list, got %#v", decision.Reinforce)
	}
	if len(decision.Watch) != 1 || decision.Watch[0].Symbol != "SOL" {
		t.Fatalf("expected SOL in watch list, got %#v", decision.Watch)
	}
}
