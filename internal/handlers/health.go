package handlers

import (
	"net/http"
	"time"
)

type HealthHandler struct {
	startedAt time.Time
	version   string
}

func NewHealthHandler(startedAt time.Time, version string) *HealthHandler {
	return &HealthHandler{
		startedAt: startedAt,
		version:   version,
	}
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"service":    "distributed-rate-limiter",
		"version":    h.version,
		"started_at": h.startedAt.Format(time.RFC3339),
	})
}
