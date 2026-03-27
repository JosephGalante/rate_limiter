package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/joe/distributed-rate-limiter/internal/auth"
)

type ProtectedHandler struct{}

func NewProtectedHandler() *ProtectedHandler {
	return &ProtectedHandler{}
}

func (h *ProtectedHandler) Route(routeID string, cost int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey, _ := auth.APIKeyFromContext(r.Context())

		payload := map[string]any{
			"route_id":     routeID,
			"request_cost": cost,
			"api_key_id":   apiKey.ID.String(),
			"served_at":    time.Now().UTC().Format(time.RFC3339),
		}

		status := http.StatusOK
		switch routeID {
		case "ping":
			payload["message"] = "pong"
		case "orders":
			status = http.StatusCreated
			payload["message"] = "order accepted"
			payload["order_id"] = uuid.NewString()
		case "report":
			payload["message"] = "report generated"
			payload["report_id"] = uuid.NewString()
		default:
			payload["message"] = "protected endpoint served"
		}

		writeJSON(w, status, payload)
	}
}
