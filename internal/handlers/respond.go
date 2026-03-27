package handlers

import (
	"encoding/json"
	"net/http"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if payload == nil {
		return
	}

	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorResponse{
		Error:   code,
		Message: message,
	})
}

func WriteUnauthorized(w http.ResponseWriter, code string, message string) {
	writeError(w, http.StatusUnauthorized, code, message)
}

func WriteBadRequest(w http.ResponseWriter, code string, message string) {
	writeError(w, http.StatusBadRequest, code, message)
}

func WriteConflict(w http.ResponseWriter, code string, message string) {
	writeError(w, http.StatusConflict, code, message)
}

func WriteInternalServerError(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "internal_server_error", "an unexpected error occurred")
}

func WriteServiceUnavailable(w http.ResponseWriter, code string, message string) {
	writeError(w, http.StatusServiceUnavailable, code, message)
}

func WriteTooManyRequests(w http.ResponseWriter, code string, message string) {
	writeError(w, http.StatusTooManyRequests, code, message)
}

func WriteNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not_found", "resource not found")
}
