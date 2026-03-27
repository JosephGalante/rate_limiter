package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/joe/distributed-rate-limiter/internal/policies"
)

type EffectivePolicyResolver interface {
	Resolve(ctx context.Context, input policies.ResolveInput) (policies.Resolution, bool, error)
}

type InspectorHandler struct {
	resolver EffectivePolicyResolver
}

func NewInspectorHandler(resolver EffectivePolicyResolver) *InspectorHandler {
	return &InspectorHandler{resolver: resolver}
}

func (h *InspectorHandler) EffectivePolicy(w http.ResponseWriter, r *http.Request) {
	routeID := strings.TrimSpace(r.URL.Query().Get("route_id"))
	if routeID == "" {
		WriteBadRequest(w, "missing_route_id", "route_id is required")
		return
	}

	var apiKeyID *uuid.UUID
	rawAPIKeyID := strings.TrimSpace(r.URL.Query().Get("api_key_id"))
	if rawAPIKeyID != "" {
		parsed, err := uuid.Parse(rawAPIKeyID)
		if err != nil {
			WriteBadRequest(w, "invalid_api_key_id", "api_key_id must be a valid UUID")
			return
		}
		apiKeyID = &parsed
	}

	resolution, found, err := h.resolver.Resolve(r.Context(), policies.ResolveInput{
		APIKeyID: apiKeyID,
		RouteID:  routeID,
	})
	if err != nil {
		switch {
		case errors.Is(err, policies.ErrInvalidRouteID):
			WriteBadRequest(w, "invalid_route_id", err.Error())
		default:
			WriteInternalServerError(w)
		}
		return
	}

	response := map[string]any{
		"found":    found,
		"route_id": routeID,
	}
	if apiKeyID != nil {
		response["api_key_id"] = apiKeyID.String()
	}

	if found {
		response["matched_scope_type"] = resolution.MatchedScopeType
		response["matched_scope_identifier"] = pointerStringFromUUID(resolution.MatchedScopeIdentifier)
		response["matched_route_pattern"] = resolution.MatchedRoutePattern
		response["policy"] = policyResponseFromDomain(resolution.Policy)
	} else {
		response["policy"] = nil
	}

	writeJSON(w, http.StatusOK, response)
}
