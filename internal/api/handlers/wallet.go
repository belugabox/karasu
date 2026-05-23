package handlers

import (
	"log/slog"
	"net/http"

	"karasu/internal/exchange"

	"github.com/gin-gonic/gin"
)

func RegisterWallet(r *gin.Engine, exchangeClient exchange.ExchangeClient) {
	r.GET("/api/wallet", func(c *gin.Context) {
		wallet, err := exchangeClient.Wallet()
		if err != nil {
			slog.Error("failed to fetch wallet", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		slog.Info("wallet fetched", "totalValue", wallet.TotalValue, "assets", len(wallet.Assets))
		c.JSON(http.StatusOK, wallet)
	})
}
