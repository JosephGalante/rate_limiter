package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/joe/distributed-rate-limiter/internal/policies"
	"github.com/joe/distributed-rate-limiter/internal/redisstate"
)

type EffectivePolicyResolver interface {
	Resolve(ctx context.Context, input policies.ResolveInput) (policies.Resolution, bool, error)
}

type BucketSnapshotReader interface {
	GetBucketSnapshot(ctx context.Context, ref redisstate.BucketRef) (redisstate.BucketSnapshot, bool, error)
}

type InspectorHandler struct {
	resolver EffectivePolicyResolver
	buckets  BucketSnapshotReader
}

func NewInspectorHandler(resolver EffectivePolicyResolver, buckets BucketSnapshotReader) *InspectorHandler {
	return &InspectorHandler{
		resolver: resolver,
		buckets:  buckets,
	}
}

func (h *InspectorHandler) EffectivePolicy(w http.ResponseWriter, r *http.Request) {
	input, apiKeyID, ok := inspectorInputFromRequest(w, r)
	if !ok {
		return
	}

	resolution, found, err := h.resolve(r, input)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, effectivePolicyResponse(input.RouteID, apiKeyID, resolution, found))
}

func (h *InspectorHandler) Bucket(w http.ResponseWriter, r *http.Request) {
	input, apiKeyID, ok := inspectorInputFromRequest(w, r)
	if !ok {
		return
	}

	resolution, found, err := h.resolve(r, input)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}

	response := effectivePolicyResponse(input.RouteID, apiKeyID, resolution, found)
	if !found {
		response["bucket_found"] = false
		response["bucket"] = nil
		writeJSON(w, http.StatusOK, response)
		return
	}

	ref := redisstate.BucketRef{
		ScopeType:       resolution.MatchedScopeType,
		ScopeIdentifier: resolution.MatchedScopeIdentifier,
		RoutePattern:    resolution.MatchedRoutePattern,
	}

	snapshot, bucketFound, err := h.buckets.GetBucketSnapshot(r.Context(), ref)
	if err != nil {
		WriteInternalServerError(w)
		return
	}

	response["bucket_key"] = redisstate.BucketKey(ref)
	response["bucket_found"] = bucketFound
	if bucketFound {
		response["bucket"] = map[string]any{
			"key":                 snapshot.Key,
			"tokens_remaining":    snapshot.TokensRemaining,
			"last_refill_unix_ms": snapshot.LastRefillUnixMs,
			"last_refill_at":      time.UnixMilli(snapshot.LastRefillUnixMs).UTC().Format(time.RFC3339),
		}
	} else {
		response["bucket"] = nil
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *InspectorHandler) resolve(r *http.Request, input policies.ResolveInput) (policies.Resolution, bool, error) {
	return h.resolver.Resolve(r.Context(), input)
}

func (h *InspectorHandler) writeResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, policies.ErrInvalidRouteID):
		WriteBadRequest(w, "invalid_route_id", err.Error())
	default:
		WriteInternalServerError(w)
	}
}

func inspectorInputFromRequest(w http.ResponseWriter, r *http.Request) (policies.ResolveInput, *uuid.UUID, bool) {
	routeID := strings.TrimSpace(r.URL.Query().Get("route_id"))
	if routeID == "" {
		WriteBadRequest(w, "missing_route_id", "route_id is required")
		return policies.ResolveInput{}, nil, false
	}

	var apiKeyID *uuid.UUID
	rawAPIKeyID := strings.TrimSpace(r.URL.Query().Get("api_key_id"))
	if rawAPIKeyID != "" {
		parsed, err := uuid.Parse(rawAPIKeyID)
		if err != nil {
			WriteBadRequest(w, "invalid_api_key_id", "api_key_id must be a valid UUID")
			return policies.ResolveInput{}, nil, false
		}
		apiKeyID = &parsed
	}

	return policies.ResolveInput{
		APIKeyID: apiKeyID,
		RouteID:  routeID,
	}, apiKeyID, true
}

func effectivePolicyResponse(routeID string, apiKeyID *uuid.UUID, resolution policies.Resolution, found bool) map[string]any {
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

	return response
}
