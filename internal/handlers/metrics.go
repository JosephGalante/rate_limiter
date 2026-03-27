package handlers

import (
	"context"
	"net/http"

	"github.com/joe/distributed-rate-limiter/internal/redisstate"
)

type SummaryMetricsReader interface {
	GetSummaryMetrics(ctx context.Context) (redisstate.SummaryMetrics, error)
}

type MetricsHandler struct {
	reader SummaryMetricsReader
}

func NewMetricsHandler(reader SummaryMetricsReader) *MetricsHandler {
	return &MetricsHandler{reader: reader}
}

func (h *MetricsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.reader.GetSummaryMetrics(r.Context())
	if err != nil {
		WriteInternalServerError(w)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"metrics": map[string]any{
			"allowed_requests": summary.AllowedRequests,
			"blocked_requests": summary.BlockedRequests,
		},
	})
}
