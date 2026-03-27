package policies

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var ErrInvalidRouteID = errors.New("route_id must be one of ping, orders, or report")

type ProjectedPolicyReader interface {
	GetProjectedPolicy(ctx context.Context, scopeType string, scopeIdentifier *uuid.UUID, routePattern *string) (Policy, bool, error)
}

type ResolveInput struct {
	APIKeyID *uuid.UUID
	RouteID  string
}

type Resolution struct {
	Policy                 Policy
	MatchedScopeType       string
	MatchedScopeIdentifier *uuid.UUID
	MatchedRoutePattern    *string
}

type Resolver struct {
	reader ProjectedPolicyReader
}

func NewResolver(reader ProjectedPolicyReader) *Resolver {
	return &Resolver{reader: reader}
}

func (r *Resolver) Resolve(ctx context.Context, input ResolveInput) (Resolution, bool, error) {
	if r == nil || r.reader == nil {
		return Resolution{}, false, nil
	}

	routeID := strings.TrimSpace(input.RouteID)
	if !isValidRouteID(routeID) {
		return Resolution{}, false, ErrInvalidRouteID
	}

	candidates := make([]resolutionCandidate, 0, 4)
	if input.APIKeyID != nil {
		candidates = append(candidates,
			resolutionCandidate{
				scopeType:       ScopeAPIKeyRoute,
				scopeIdentifier: input.APIKeyID,
				routePattern:    routeIDPointer(routeID),
			},
			resolutionCandidate{
				scopeType:       ScopeAPIKey,
				scopeIdentifier: input.APIKeyID,
			},
		)
	}

	candidates = append(candidates,
		resolutionCandidate{
			scopeType:    ScopeRoute,
			routePattern: routeIDPointer(routeID),
		},
		resolutionCandidate{
			scopeType: ScopeGlobal,
		},
	)

	for _, candidate := range candidates {
		policy, found, err := r.reader.GetProjectedPolicy(ctx, candidate.scopeType, candidate.scopeIdentifier, candidate.routePattern)
		if err != nil {
			return Resolution{}, false, fmt.Errorf("read projected policy for scope %s: %w", candidate.scopeType, err)
		}

		if found {
			return Resolution{
				Policy:                 policy,
				MatchedScopeType:       candidate.scopeType,
				MatchedScopeIdentifier: candidate.scopeIdentifier,
				MatchedRoutePattern:    candidate.routePattern,
			}, true, nil
		}
	}

	return Resolution{}, false, nil
}

type resolutionCandidate struct {
	scopeType       string
	scopeIdentifier *uuid.UUID
	routePattern    *string
}

func isValidRouteID(routeID string) bool {
	_, ok := validRoutePatterns[routeID]
	return ok
}

func routeIDPointer(value string) *string {
	return &value
}
