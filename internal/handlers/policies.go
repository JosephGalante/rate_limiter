package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/joe/distributed-rate-limiter/internal/policies"
)

type PoliciesHandler struct {
	service *policies.Service
}

type createPolicyRequest struct {
	ScopeType             string  `json:"scope_type"`
	ScopeIdentifier       *string `json:"scope_identifier"`
	RoutePattern          *string `json:"route_pattern"`
	Capacity              int32   `json:"capacity"`
	RefillTokens          int32   `json:"refill_tokens"`
	RefillIntervalSeconds int32   `json:"refill_interval_seconds"`
}

type policyResponse struct {
	ID                    string  `json:"id"`
	ScopeType             string  `json:"scope_type"`
	ScopeIdentifier       *string `json:"scope_identifier,omitempty"`
	RoutePattern          *string `json:"route_pattern,omitempty"`
	Capacity              int32   `json:"capacity"`
	RefillTokens          int32   `json:"refill_tokens"`
	RefillIntervalSeconds int32   `json:"refill_interval_seconds"`
	IsActive              bool    `json:"is_active"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
}

func NewPoliciesHandler(service *policies.Service) *PoliciesHandler {
	return &PoliciesHandler{service: service}
}

func (h *PoliciesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createPolicyRequest
	if err := decodeJSON(r, &request); err != nil {
		WriteBadRequest(w, "invalid_request", err.Error())
		return
	}

	var scopeIdentifier *uuid.UUID
	if request.ScopeIdentifier != nil && strings.TrimSpace(*request.ScopeIdentifier) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*request.ScopeIdentifier))
		if err != nil {
			WriteBadRequest(w, "invalid_scope_identifier", "scope_identifier must be a valid UUID")
			return
		}
		scopeIdentifier = &parsed
	}

	created, err := h.service.Create(r.Context(), policies.CreatePolicyInput{
		ScopeType:             request.ScopeType,
		ScopeIdentifier:       scopeIdentifier,
		RoutePattern:          request.RoutePattern,
		Capacity:              request.Capacity,
		RefillTokens:          request.RefillTokens,
		RefillIntervalSeconds: request.RefillIntervalSeconds,
	})
	if err != nil {
		switch {
		case errors.Is(err, policies.ErrInvalidScopeType),
			errors.Is(err, policies.ErrInvalidScopeShape),
			errors.Is(err, policies.ErrInvalidRoutePattern),
			errors.Is(err, policies.ErrInvalidCapacity),
			errors.Is(err, policies.ErrInvalidRefillTokens),
			errors.Is(err, policies.ErrInvalidRefillInterval),
			errors.Is(err, policies.ErrScopedAPIKeyNotFound):
			WriteBadRequest(w, "invalid_policy", err.Error())
		case errors.Is(err, policies.ErrPolicyConflict):
			WriteConflict(w, "policy_conflict", "an active policy already exists for that scope")
		default:
			WriteInternalServerError(w)
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"policy": policyResponseFromDomain(created),
	})
}

func (h *PoliciesHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		WriteInternalServerError(w)
		return
	}

	response := make([]policyResponse, 0, len(items))
	for _, item := range items {
		response = append(response, policyResponseFromDomain(item))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"policies": response,
	})
}

func (h *PoliciesHandler) Update(w http.ResponseWriter, r *http.Request) {
	policyID, err := uuid.Parse(chi.URLParam(r, "policyID"))
	if err != nil {
		WriteBadRequest(w, "invalid_policy_id", "policyID must be a valid UUID")
		return
	}

	var request createPolicyRequest
	if err := decodeJSON(r, &request); err != nil {
		WriteBadRequest(w, "invalid_request", err.Error())
		return
	}

	var scopeIdentifier *uuid.UUID
	if request.ScopeIdentifier != nil && strings.TrimSpace(*request.ScopeIdentifier) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*request.ScopeIdentifier))
		if err != nil {
			WriteBadRequest(w, "invalid_scope_identifier", "scope_identifier must be a valid UUID")
			return
		}
		scopeIdentifier = &parsed
	}

	updated, err := h.service.Update(r.Context(), policyID, policies.UpdatePolicyInput{
		ScopeType:             request.ScopeType,
		ScopeIdentifier:       scopeIdentifier,
		RoutePattern:          request.RoutePattern,
		Capacity:              request.Capacity,
		RefillTokens:          request.RefillTokens,
		RefillIntervalSeconds: request.RefillIntervalSeconds,
	})
	if err != nil {
		switch {
		case errors.Is(err, policies.ErrPolicyNotFound):
			WriteNotFound(w)
		case errors.Is(err, policies.ErrInvalidScopeType),
			errors.Is(err, policies.ErrInvalidScopeShape),
			errors.Is(err, policies.ErrInvalidRoutePattern),
			errors.Is(err, policies.ErrInvalidCapacity),
			errors.Is(err, policies.ErrInvalidRefillTokens),
			errors.Is(err, policies.ErrInvalidRefillInterval),
			errors.Is(err, policies.ErrScopedAPIKeyNotFound):
			WriteBadRequest(w, "invalid_policy", err.Error())
		case errors.Is(err, policies.ErrPolicyConflict):
			WriteConflict(w, "policy_conflict", "an active policy already exists for that scope")
		default:
			WriteInternalServerError(w)
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"policy": policyResponseFromDomain(updated),
	})
}

func (h *PoliciesHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	policyID, err := uuid.Parse(chi.URLParam(r, "policyID"))
	if err != nil {
		WriteBadRequest(w, "invalid_policy_id", "policyID must be a valid UUID")
		return
	}

	policy, err := h.service.Deactivate(r.Context(), policyID)
	if err != nil {
		switch {
		case errors.Is(err, policies.ErrPolicyNotFound):
			WriteNotFound(w)
		default:
			WriteInternalServerError(w)
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"policy": policyResponseFromDomain(policy),
	})
}

func policyResponseFromDomain(policy policies.Policy) policyResponse {
	return policyResponse{
		ID:                    policy.ID.String(),
		ScopeType:             policy.ScopeType,
		ScopeIdentifier:       pointerStringFromUUID(policy.ScopeIdentifier),
		RoutePattern:          policy.RoutePattern,
		Capacity:              policy.Capacity,
		RefillTokens:          policy.RefillTokens,
		RefillIntervalSeconds: policy.RefillIntervalSeconds,
		IsActive:              policy.IsActive,
		CreatedAt:             policy.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             policy.UpdatedAt.Format(time.RFC3339),
	}
}
