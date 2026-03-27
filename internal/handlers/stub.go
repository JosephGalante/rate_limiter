package handlers

import "net/http"

type StubHandler struct{}

func NewStubHandler() *StubHandler {
	return &StubHandler{}
}

func (h *StubHandler) AdminPing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": "admin surface is wired; CRUD handlers will be added in the next chunks",
	})
}

func (h *StubHandler) NotImplemented(feature string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotImplemented, "not_implemented", feature+" is scaffolded but not implemented yet")
	}
}

func (h *StubHandler) ProtectedRoute(routeID string, cost int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "scaffold",
			"route_id": routeID,
			"cost":     cost,
			"message":  "protected endpoint scaffold is online; API key auth and rate limiting will be wired in later chunks",
		})
	}
}
