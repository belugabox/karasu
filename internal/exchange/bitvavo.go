package exchange

import (
	"fmt"
	"karasu/internal/exchange/bitvavo"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type walletHistoryAnalysis struct {
	NetDepositsEUR     float64
	AvgCostEURBySymbol map[string]float64
}

type symbolLedger struct {
	Qty     float64
	CostEUR float64
}

type BitvavoClient struct {
	api *bitvavo.Bitvavo
}

func NewBitvavoClient() (BitvavoClient, error) {

	bitvavoApiKey := os.Getenv("BITVAVO_API_KEY")
	bitvavoApiSecret := os.Getenv("BITVAVO_API_SECRET")
	if bitvavoApiKey == "" || bitvavoApiSecret == "" {
		return BitvavoClient{}, fmt.Errorf("missing Bitvavo credentials: set BITVAVO_API_KEY and BITVAVO_API_SECRET")
	}

	bitvavo := bitvavo.Bitvavo{
		ApiKey:       bitvavoApiKey,
		ApiSecret:    bitvavoApiSecret,
		RestUrl:      "https://api.bitvavo.com/v2",
		WsUrl:        "wss://ws.bitvavo.com/v2/",
		AccessWindow: 10000,
		Debugging:    false,
	}

	// test API call to verify credentials and connectivity
	_, err := bitvavo.Balance(map[string]string{})
	if err != nil {
		return BitvavoClient{}, fmt.Errorf("bitvavo API connectivity test failed: %w", err)
	}

	return BitvavoClient{
		api: &bitvavo}, nil
}

func symbolToMarket(symbol string) string {
	return symbol + "-EUR"
}

func marketToSymbol(market string) string {
	return market[:len(market)-4]
}

// Symbols retourne une map de symboles à labels, e.g., "BTC" à "Bitcoin"
func (c BitvavoClient) Symbols() (map[string]string, error) {
	response, err := c.api.Assets(map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("bitvavo GetAssets API call failed: %w", err)
	}
	symbols := make(map[string]string)

	for _, raw := range response {
		if raw.Symbol == "" || raw.Name == "" {
			continue
		}
		symbols[raw.Symbol] = raw.Name
	}
	return symbols, nil
}

// Prices retourne une map de symboles à prix en EUR, e.g., "BTC" à 30000.0
func (c BitvavoClient) Prices() (map[string]float64, error) {
	response, err := c.api.TickerPrice(map[string]string{})

	if err != nil {
		return nil, fmt.Errorf("bitvavo TickerPrice API call failed: %w", err)
	}

	prices := make(map[string]float64)
	for _, raw := range response {
		if raw.Market == "" || raw.Price == "" {
			continue
		}
		symbol := marketToSymbol(raw.Market)
		price, err := strconv.ParseFloat(raw.Price, 64)
		if err != nil {
			continue
		}
		prices[symbol] = price
	}
	return prices, nil
}

// Wallet retourne le wallet complet avec la valeur totale en EUR, la valeur du cash en EUR, la valeur des actifs en EUR, et la liste des actifs avec leur symbole, montant et valeur en EUR
func (c BitvavoClient) Wallet() (Wallet, error) {
	// On récupère les soldes de tous les actifs du wallet
	resBalance, err := c.api.Balance(map[string]string{})
	if err != nil {
		return Wallet{}, fmt.Errorf("bitvavo Balance API call failed: %w", err)
	}

	// On récupère les soldes de tous les actifs du wallet en staking pour les inclure dans le calcul de la valeur totale du wallet
	resStakingBalance, err := c.api.StakingBalance(map[string]string{})
	if err != nil {
		return Wallet{}, fmt.Errorf("bitvavo StakingBalance API call failed: %w", err)
	}

	// On récupère les prix actuels de tous les marchés pour calculer la valeur de chaque actif en EUR
	prices, err := c.Prices()
	if err != nil {
		return Wallet{}, err
	}
	prices["EUR"] = 1

	historyItems, err := c.fetchTransactionHistory(3, 100)
	if err != nil {
		slog.Warn("failed to fetch transaction history for wallet analytics", "err", err)
		historyItems = []bitvavo.AccountTransaction{}
	}
	historyAnalysis := analyzeWalletHistory(historyItems)
	netDepositsValue := historyAnalysis.NetDepositsEUR

	// On calcule la valeur de chaque actif en EUR, ainsi que la valeur totale du wallet et la valeur du cash
	mapAssets := make(map[string]WalletAsset, 0)
	var totalValue float64
	var cashValue float64
	for _, raw := range resBalance {
		if raw.Symbol == "" || raw.Available == "" || raw.InOrder == "" {
			continue
		}
		amount, err := strconv.ParseFloat(raw.Available, 64)
		if err != nil {
			continue
		}
		amountInOrder, err := strconv.ParseFloat(raw.InOrder, 64)
		if err != nil {
			continue
		}
		price, ok := prices[raw.Symbol]
		if !ok {
			continue
		}
		value := (amount + amountInOrder) * price
		mapAssets[raw.Symbol] = WalletAsset{
			Symbol:        raw.Symbol,
			Amount:        amount,
			InOrder:       amountInOrder,
			StakingAmount: 0,
			Value:         value,
		}
		totalValue += value
		if raw.Symbol == "EUR" {
			cashValue += value
		}
	}
	for _, raw := range resStakingBalance {
		if raw.Symbol == "" || raw.Amount == "" {
			continue
		}
		stakedAmount, err := strconv.ParseFloat(raw.Amount, 64)
		if err != nil {
			continue
		}
		price, ok := prices[raw.Symbol]
		if !ok {
			continue
		}
		value := stakedAmount * price
		asset, ok := mapAssets[raw.Symbol]
		if !ok {
			asset = WalletAsset{
				Symbol: raw.Symbol,
			}
		}
		asset.StakingAmount = stakedAmount
		asset.Value += value
		mapAssets[raw.Symbol] = asset
		totalValue += value
	}

	for symbol, asset := range mapAssets {
		upperSymbol := strings.ToUpper(strings.TrimSpace(symbol))
		if upperSymbol == "" {
			continue
		}

		if upperSymbol == "EUR" {
			asset.CostBasisValue = asset.Value
			asset.PnLValue = 0
			asset.PnLPercent = 0
			mapAssets[symbol] = asset
			continue
		}

		positionAmount := asset.Amount + asset.InOrder + asset.StakingAmount
		avgCost := historyAnalysis.AvgCostEURBySymbol[upperSymbol]
		costBasis := avgCost * positionAmount
		asset.CostBasisValue = costBasis
		asset.PnLValue = asset.Value - costBasis
		asset.PnLPercent = 0
		if costBasis > 0 {
			asset.PnLPercent = (asset.PnLValue / costBasis) * 100
		}
		mapAssets[symbol] = asset
	}

	assets := make([]WalletAsset, 0, len(mapAssets))
	for _, asset := range mapAssets {
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Value > assets[j].Value
	})

	pnlValue := totalValue - netDepositsValue
	pnlPercent := 0.0
	if netDepositsValue > 0 {
		pnlPercent = (pnlValue / netDepositsValue) * 100
	}

	return Wallet{
		TotalValue:       totalValue,
		CashValue:        cashValue,
		AssetValue:       totalValue - cashValue,
		NetDepositsValue: netDepositsValue,
		PnLValue:         pnlValue,
		PnLPercent:       pnlPercent,
		Assets:           assets,
	}, nil
}

func (c BitvavoClient) fetchTransactionHistory(maxPages int, maxItems int) ([]bitvavo.AccountTransaction, error) {
	if maxPages < 1 {
		maxPages = 1
	}
	if maxItems < 1 {
		maxItems = 100
	}

	items := make([]bitvavo.AccountTransaction, 0, maxPages*maxItems)
	for page := 1; page <= maxPages; page++ {
		resp, err := c.api.TransactionHistory(map[string]string{
			"page":     strconv.Itoa(page),
			"maxItems": strconv.Itoa(maxItems),
		})
		if err != nil {
			return nil, fmt.Errorf("bitvavo transaction history API call failed (page %d): %w", page, err)
		}

		items = append(items, resp.Items...)

		if len(resp.Items) == 0 {
			break
		}
		if resp.TotalPages > 0 && page >= resp.TotalPages {
			break
		}
	}

	return items, nil
}

func analyzeWalletHistory(items []bitvavo.AccountTransaction) walletHistoryAnalysis {
	sorted := make([]bitvavo.AccountTransaction, len(items))
	copy(sorted, items)

	sort.SliceStable(sorted, func(i, j int) bool {
		ti, errI := time.Parse(time.RFC3339, sorted[i].ExecutedAt)
		tj, errJ := time.Parse(time.RFC3339, sorted[j].ExecutedAt)
		if errI != nil || errJ != nil {
			return sorted[i].ExecutedAt < sorted[j].ExecutedAt
		}
		return ti.Before(tj)
	})

	ledgers := make(map[string]symbolLedger)
	netDeposits := 0.0

	for _, tx := range sorted {
		netDeposits += netEURFromTransaction(tx)

		txType := strings.ToLower(strings.TrimSpace(tx.Type))
		sentCurrency := strings.ToUpper(strings.TrimSpace(tx.SentCurrency))
		receivedCurrency := strings.ToUpper(strings.TrimSpace(tx.ReceivedCurrency))
		feesCurrency := strings.ToUpper(strings.TrimSpace(tx.FeesCurrency))

		sentAmount := parseFloatOrZero(tx.SentAmount)
		receivedAmount := parseFloatOrZero(tx.ReceivedAmount)
		feesAmount := parseFloatOrZero(tx.FeesAmount)

		switch txType {
		case "buy":
			if receivedCurrency == "" || receivedCurrency == "EUR" || receivedAmount <= 0 {
				continue
			}
			ledger := ledgers[receivedCurrency]
			eurCost := 0.0
			if sentCurrency == "EUR" {
				eurCost += sentAmount
			}
			if feesCurrency == "EUR" {
				eurCost += feesAmount
			}
			ledger.Qty += receivedAmount
			ledger.CostEUR += eurCost
			ledgers[receivedCurrency] = ledger

		case "sell":
			if sentCurrency == "" || sentCurrency == "EUR" || sentAmount <= 0 {
				continue
			}
			ledger := ledgers[sentCurrency]
			if ledger.Qty <= 0 {
				continue
			}
			soldQty := sentAmount
			if soldQty > ledger.Qty {
				soldQty = ledger.Qty
			}
			costRemoved := 0.0
			if ledger.Qty > 0 {
				costRemoved = ledger.CostEUR * (soldQty / ledger.Qty)
			}
			ledger.Qty -= soldQty
			ledger.CostEUR -= costRemoved
			if ledger.Qty < 1e-12 {
				ledger.Qty = 0
				ledger.CostEUR = 0
			}
			if ledger.CostEUR < 0 {
				ledger.CostEUR = 0
			}
			ledgers[sentCurrency] = ledger

		default:
			if receivedCurrency != "" && receivedCurrency != "EUR" && receivedAmount > 0 {
				ledger := ledgers[receivedCurrency]
				ledger.Qty += receivedAmount
				ledgers[receivedCurrency] = ledger
			}

			if sentCurrency != "" && sentCurrency != "EUR" && sentAmount > 0 {
				ledger := ledgers[sentCurrency]
				if ledger.Qty > 0 {
					movedQty := sentAmount
					if movedQty > ledger.Qty {
						movedQty = ledger.Qty
					}
					costRemoved := 0.0
					if ledger.Qty > 0 {
						costRemoved = ledger.CostEUR * (movedQty / ledger.Qty)
					}
					ledger.Qty -= movedQty
					ledger.CostEUR -= costRemoved
					if ledger.Qty < 1e-12 {
						ledger.Qty = 0
						ledger.CostEUR = 0
					}
					if ledger.CostEUR < 0 {
						ledger.CostEUR = 0
					}
					ledgers[sentCurrency] = ledger
				}
			}
		}
	}

	avgCostBySymbol := make(map[string]float64, len(ledgers))
	for symbol, ledger := range ledgers {
		if ledger.Qty <= 0 || ledger.CostEUR <= 0 {
			continue
		}
		avgCostBySymbol[symbol] = ledger.CostEUR / ledger.Qty
	}

	return walletHistoryAnalysis{
		NetDepositsEUR:     netDeposits,
		AvgCostEURBySymbol: avgCostBySymbol,
	}
}

func parseFloatOrZero(raw string) float64 {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return v
}

func netEURFromTransaction(tx bitvavo.AccountTransaction) float64 {
	txType := strings.ToLower(strings.TrimSpace(tx.Type))
	if txType == "buy" || txType == "sell" || txType == "staking" || txType == "fixed_staking" {
		return 0
	}

	sentCurrency := strings.ToUpper(strings.TrimSpace(tx.SentCurrency))
	receivedCurrency := strings.ToUpper(strings.TrimSpace(tx.ReceivedCurrency))
	feesCurrency := strings.ToUpper(strings.TrimSpace(tx.FeesCurrency))

	sentAmount, err := strconv.ParseFloat(tx.SentAmount, 64)
	if err != nil {
		sentAmount = 0
	}
	receivedAmount, err := strconv.ParseFloat(tx.ReceivedAmount, 64)
	if err != nil {
		receivedAmount = 0
	}
	feesAmount, err := strconv.ParseFloat(tx.FeesAmount, 64)
	if err != nil {
		feesAmount = 0
	}

	net := 0.0
	if receivedCurrency == "EUR" {
		net += receivedAmount
	}
	if sentCurrency == "EUR" {
		net -= sentAmount
	}
	if feesCurrency == "EUR" {
		net -= feesAmount
	}

	return net
}

func (c BitvavoClient) Candles1mByDate(symbol string, date time.Time) (CandleBundle, error) {
	start := date.Add(-24 * time.Hour)
	end := date
	return c.Candles(symbol, start, end, Interval1m)
}

func (c BitvavoClient) Candles5mByDate(symbol string, date time.Time) (CandleBundle, error) {
	start := date.Add(-24 * time.Hour)
	end := date
	return c.Candles(symbol, start, end, Interval5m)
}

func (c BitvavoClient) CandlesLast24h() ([]CandleBundle, error) {
	params := map[string]string{}
	response, err := c.api.Ticker24h(params)
	if err != nil {
		return nil, fmt.Errorf("bitvavo GetCandles API call failed: %w", err)
	}
	candles := make([]CandleBundle, 0)
	for _, raw := range response {
		// if rmarket finsihes with "-EUR", we assume it's a valid market and try to parse it, otherwise skip
		if len(raw.Market) < 4 || raw.Market[len(raw.Market)-4:] != "-EUR" {
			//slog.Debug("skipping non-EUR market in 24h candles", "market", raw.Market)
			continue
		}
		symbol := marketToSymbol(raw.Market)
		if symbol == "" || raw.Open == "" || raw.High == "" || raw.Low == "" || raw.Last == "" || raw.Volume == "" {
			//slog.Debug("skipping 24h candle with zero values", "raw", raw)
			continue
		}
		candle, err := rawToCandle(BitvavoRawCandle{raw.Timestamp, raw.Open, raw.High, raw.Low, raw.Last, raw.Volume}, 24*time.Hour)
		if err != nil {
			slog.Debug("bitvavo candle parsing failed", "raw", raw, "error", err)
			return nil, fmt.Errorf("bitvavo candle parsing failed for market %s: %w", raw.Market, err)
		}
		candles = append(candles, CandleBundle{
			Symbol:    symbol,
			Interval:  Interval1d,
			Candles:   []Candle{candle},
			StartTime: time.UnixMilli(int64(raw.OpenTimestamp)),
			EndTime:   time.UnixMilli(int64(raw.CloseTimestamp)),
		})
	}
	return candles, nil
}

func (c BitvavoClient) Candles(symbol string, start time.Time, end time.Time, interval Interval) (CandleBundle, error) {
	market := symbolToMarket(symbol)
	params := map[string]string{
		"start": fmt.Sprintf("%d", start.UnixMilli()),
		"end":   fmt.Sprintf("%d", end.UnixMilli()),
	}
	response, err := c.api.Candles(market, string(interval), params)
	if err != nil {
		return CandleBundle{}, fmt.Errorf("bitvavo GetCandles API call failed: %w", err)
	}

	candles := make([]Candle, len(response))
	for i, raw := range response {
		candle, err := rawToCandle(BitvavoRawCandle{raw.Timestamp, raw.Open, raw.High, raw.Low, raw.Close, raw.Volume}, intervalDuration(interval))
		if err != nil {
			slog.Debug("bitvavo candle parsing failed", "raw", raw, "error", err)
			return CandleBundle{}, fmt.Errorf("bitvavo candle parsing failed: %w", err)
		}
		candles[i] = candle
	}

	return CandleBundle{
		Symbol:    symbol,
		Interval:  interval,
		Candles:   candles,
		StartTime: start,
		EndTime:   end,
	}, nil
}

type BitvavoRawCandle struct {
	Timestamp int
	Open      string
	High      string
	Low       string
	Close     string
	Volume    string
}

func rawToCandle(raw BitvavoRawCandle, intervalDur time.Duration) (Candle, error) {
	open, err := strconv.ParseFloat(raw.Open, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("bitvavo candle parse error (open): %w", err)
	}

	high, err := strconv.ParseFloat(raw.High, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("bitvavo candle parse error (high): %w", err)
	}

	low, err := strconv.ParseFloat(raw.Low, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("bitvavo candle parse error (low): %w", err)
	}

	closePrice, err := strconv.ParseFloat(raw.Close, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("bitvavo candle parse error (close): %w", err)
	}

	volume, err := strconv.ParseFloat(raw.Volume, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("bitvavo candle parse error (volume): %w", err)
	}

	candle := Candle{
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closePrice,
		Volume:    volume,
		OpenTime:  time.UnixMilli(int64(raw.Timestamp)),
		CloseTime: time.UnixMilli(int64(raw.Timestamp)).Add(intervalDur),
	}

	return candle, nil
}
