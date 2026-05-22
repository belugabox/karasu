package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karasu/internal/ingestion"

	"github.com/gin-gonic/gin"
)

func RegisterBackfill(r *gin.Engine, ingestionService *ingestion.IngestionService) {
	r.POST("/api/backfill-5m", func(c *gin.Context) {
		rawSymbols := strings.TrimSpace(c.Query("symbols"))
		symbols := make([]string, 0)
		if rawSymbols != "" {
			for _, symbol := range strings.Split(rawSymbols, ",") {
				symbol = strings.ToUpper(strings.TrimSpace(symbol))
				if symbol == "" {
					continue
				}
				symbols = append(symbols, symbol)
			}
		}

		from, err := parseTimeQuery(c, "from")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		to, err := parseTimeQuery(c, "to")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		job, err := ingestionService.EnqueueBackfill(symbols, from, to, "api-backfill")
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"status": "queued",
			"job":    job,
		})
	})

	r.POST("/api/backtest-symbol", func(c *gin.Context) {
		symbol := strings.ToUpper(strings.TrimSpace(c.Query("symbol")))
		if symbol == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
			return
		}

		from, err := parseTimeQuery(c, "from")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		to, err := parseTimeQuery(c, "to")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		job, err := ingestionService.EnqueueBackfill([]string{symbol}, from, to, "api-backtest-symbol")
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"status": "queued",
			"symbol": symbol,
			"job":    job,
		})
	})

	r.GET("/api/backfill-status", func(c *gin.Context) {
		jobID := strings.TrimSpace(c.Query("jobId"))
		if jobID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "jobId is required"})
			return
		}

		job, ok := ingestionService.GetBackfillJob(jobID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"job":    job,
		})
	})

	r.GET("/api/backfill-jobs", func(c *gin.Context) {
		limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}

		jobs := ingestionService.ListBackfillJobs(limit)
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"count":  len(jobs),
			"jobs":   jobs,
		})
	})
}

func parseTimeQuery(c *gin.Context, key string) (time.Time, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}, fmt.Errorf("%s is required (RFC3339 or unix milliseconds)", key)
	}

	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.UnixMilli(ms).UTC(), nil
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s: use RFC3339 (example 2026-05-01T00:00:00Z) or unix milliseconds", key)
	}

	return parsed.UTC(), nil
}
