package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/joe/distributed-rate-limiter/internal/auth"
)

type APIKeysHandler struct {
	service *auth.APIKeyService
}

type createAPIKeyRequest struct {
	Name   string  `json:"name"`
	UserID *string `json:"user_id"`
}

type apiKeyResponse struct {
	ID         string  `json:"id"`
	UserID     *string `json:"user_id,omitempty"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"key_prefix"`
	IsActive   bool    `json:"is_active"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
}

func NewAPIKeysHandler(service *auth.APIKeyService) *APIKeysHandler {
	return &APIKeysHandler{service: service}
}

func (h *APIKeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createAPIKeyRequest
	if err := decodeJSON(r, &request); err != nil {
		WriteBadRequest(w, "invalid_request", err.Error())
		return
	}

	var userID *uuid.UUID
	if request.UserID != nil && strings.TrimSpace(*request.UserID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*request.UserID))
		if err != nil {
			WriteBadRequest(w, "invalid_user_id", "user_id must be a valid UUID")
			return
		}

		userID = &parsed
	}

	created, err := h.service.Create(r.Context(), auth.CreateAPIKeyInput{
		Name:   request.Name,
		UserID: userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidAPIKeyName):
			WriteBadRequest(w, "invalid_api_key_name", err.Error())
		case errors.Is(err, auth.ErrUserNotFound):
			WriteBadRequest(w, "user_not_found", "user_id does not reference an existing user")
		default:
			WriteInternalServerError(w)
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"api_key": apiKeyResponseFromDomain(created.APIKey),
		"raw_key": created.RawKey,
	})
}

func (h *APIKeysHandler) List(w http.ResponseWriter, r *http.Request) {
	apiKeys, err := h.service.List(r.Context())
	if err != nil {
		WriteInternalServerError(w)
		return
	}

	response := make([]apiKeyResponse, 0, len(apiKeys))
	for _, apiKey := range apiKeys {
		response = append(response, apiKeyResponseFromDomain(apiKey))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"api_keys": response,
	})
}

func (h *APIKeysHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	apiKeyID, err := uuid.Parse(chi.URLParam(r, "apiKeyID"))
	if err != nil {
		WriteBadRequest(w, "invalid_api_key_id", "apiKeyID must be a valid UUID")
		return
	}

	apiKey, err := h.service.Deactivate(r.Context(), apiKeyID)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAPIKeyNotFound):
			WriteNotFound(w)
		default:
			WriteInternalServerError(w)
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"api_key": apiKeyResponseFromDomain(apiKey),
	})
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func apiKeyResponseFromDomain(apiKey auth.APIKey) apiKeyResponse {
	return apiKeyResponse{
		ID:         apiKey.ID.String(),
		UserID:     pointerStringFromUUID(apiKey.UserID),
		Name:       apiKey.Name,
		KeyPrefix:  apiKey.KeyPrefix,
		IsActive:   apiKey.IsActive,
		CreatedAt:  apiKey.CreatedAt.Format(time.RFC3339),
		LastUsedAt: pointerStringFromTime(apiKey.LastUsedAt),
	}
}

func pointerStringFromUUID(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}

	stringValue := value.String()
	return &stringValue
}

func pointerStringFromTime(value *time.Time) *string {
	if value == nil {
		return nil
	}

	timestamp := value.UTC().Format(time.RFC3339)
	return &timestamp
}
