package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newBackfillRouter() *gin.Engine {
	svc := newIngestionService(&fakeExchangeClient{}, &fakeCandleStore{})
	r := gin.New()
	RegisterBackfill(r, svc)
	return r
}

// TestBackfill5mEndpointReturnsBadRequestOnMissingFrom verifies POST /api/backfill-5m
// returns 400 when the 'from' query parameter is missing.
func TestBackfill5mEndpointReturnsBadRequestOnMissingFrom(t *testing.T) {
	t.Parallel()

	r := newBackfillRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/backfill-5m?to=2026-05-01T00:00:00Z", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBackfill5mEndpointReturnsBadRequestOnMissingTo verifies POST /api/backfill-5m
// returns 400 when the 'to' query parameter is missing.
func TestBackfill5mEndpointReturnsBadRequestOnMissingTo(t *testing.T) {
	t.Parallel()

	r := newBackfillRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/backfill-5m?from=2026-05-01T00:00:00Z", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBackfill5mEndpointReturnsBadRequestOnInvalidTimeFormat verifies POST /api/backfill-5m
// returns 400 when the 'from' parameter is not a valid time format.
func TestBackfill5mEndpointReturnsBadRequestOnInvalidTimeFormat(t *testing.T) {
	t.Parallel()

	r := newBackfillRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/backfill-5m?from=notadate&to=2026-05-02T00:00:00Z", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBackfill5mEndpointAcceptsValidJob verifies POST /api/backfill-5m
// returns 202 Accepted with a job payload when valid parameters are provided.
func TestBackfill5mEndpointAcceptsValidJob(t *testing.T) {
	t.Parallel()

	from := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)

	r := newBackfillRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/backfill-5m?symbols=BTC,ETH&from="+from+"&to="+to, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", body["status"])
	}
	if _, ok := body["job"]; !ok {
		t.Error("expected 'job' field in backfill response")
	}

	job, ok := body["job"].(map[string]interface{})
	if !ok {
		t.Fatal("expected job to be an object")
	}
	for _, field := range []string{"id", "symbols", "from", "to", "state", "createdAt"} {
		if _, ok := job[field]; !ok {
			t.Errorf("expected field %q in backfill job payload, not found", field)
		}
	}
	if job["state"] != "queued" {
		t.Errorf("expected job state 'queued', got %v", job["state"])
	}
}

// TestBackfill5mEndpointAcceptsUnixMilliseconds verifies POST /api/backfill-5m
// also accepts unix milliseconds for the from/to parameters.
func TestBackfill5mEndpointAcceptsUnixMilliseconds(t *testing.T) {
	t.Parallel()

	from := time.Now().UTC().Add(-48 * time.Hour).UnixMilli()
	to := time.Now().UTC().Add(-24 * time.Hour).UnixMilli()

	r := newBackfillRouter()
	w := httptest.NewRecorder()
	url := "/api/backfill-5m?from=" + formatInt64(from) + "&to=" + formatInt64(to)
	req := httptest.NewRequest(http.MethodPost, url, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBackfillStatusEndpointReturns404ForUnknownJob verifies GET /api/backfill-status
// returns 404 when the requested job ID does not exist.
func TestBackfillStatusEndpointReturns404ForUnknownJob(t *testing.T) {
	t.Parallel()

	r := newBackfillRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/backfill-status?jobId=unknown_job", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBackfillStatusEndpointReturnsBadRequestOnMissingJobId verifies GET /api/backfill-status
// returns 400 when the jobId parameter is missing.
func TestBackfillStatusEndpointReturnsBadRequestOnMissingJobId(t *testing.T) {
	t.Parallel()

	r := newBackfillRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/backfill-status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBackfillStatusEndpointReturnsJobAfterEnqueue verifies GET /api/backfill-status
// returns the queued job after it has been enqueued via POST /api/backfill-5m.
func TestBackfillStatusEndpointReturnsJobAfterEnqueue(t *testing.T) {
	t.Parallel()

	svc := newIngestionService(&fakeExchangeClient{}, &fakeCandleStore{})
	r := gin.New()
	RegisterBackfill(r, svc)

	from := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)

	// Enqueue the job
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/api/backfill-5m?symbols=BTC&from="+from+"&to="+to, nil)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w1.Code)
	}

	var enqueueBody map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &enqueueBody); err != nil {
		t.Fatalf("enqueue response is not JSON: %v", err)
	}
	job, _ := enqueueBody["job"].(map[string]interface{})
	jobID, _ := job["id"].(string)
	if jobID == "" {
		t.Fatal("expected non-empty job ID")
	}

	// Check job status
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/backfill-status?jobId="+jobID, nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
}

// TestBackfillJobsEndpointReturnsListPayloadShape verifies GET /api/backfill-jobs
// returns a response with count and jobs fields.
func TestBackfillJobsEndpointReturnsListPayloadShape(t *testing.T) {
	t.Parallel()

	r := newBackfillRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/backfill-jobs", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if _, ok := body["count"]; !ok {
		t.Error("expected 'count' field in backfill-jobs response")
	}
	if _, ok := body["jobs"]; !ok {
		t.Error("expected 'jobs' field in backfill-jobs response")
	}
}

// TestBackfillJobsEndpointReturnsBadRequestOnInvalidLimit verifies GET /api/backfill-jobs
// returns 400 when the limit parameter is not a valid integer.
func TestBackfillJobsEndpointReturnsBadRequestOnInvalidLimit(t *testing.T) {
	t.Parallel()

	r := newBackfillRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/backfill-jobs?limit=bad", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}
