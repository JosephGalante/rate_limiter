package policies

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestResolverAppliesPrecedence(t *testing.T) {
	apiKeyID := uuid.MustParse("95e35040-c33a-4a39-b801-b6fdf6dc9fcc")
	globalPolicy := Policy{ID: uuid.MustParse("0daee1b5-7ff1-45ea-ba1e-e3f2da45ec29"), ScopeType: ScopeGlobal}
	routePolicy := Policy{ID: uuid.MustParse("7bb767b7-fef8-4839-8f15-2f0ca17955a6"), ScopeType: ScopeRoute}
	apiKeyPolicy := Policy{ID: uuid.MustParse("3d0077f3-353a-40aa-ba9f-b446fd7891a1"), ScopeType: ScopeAPIKey}
	apiKeyRoutePolicy := Policy{ID: uuid.MustParse("0c9fc453-c4e3-4749-85f1-abbb2289ad0a"), ScopeType: ScopeAPIKeyRoute}

	store := fakeProjectedPolicyReader{
		items: map[string]Policy{
			fakePolicyLookupKey(ScopeGlobal, nil, nil):                                globalPolicy,
			fakePolicyLookupKey(ScopeRoute, nil, stringPointer("report")):             routePolicy,
			fakePolicyLookupKey(ScopeAPIKey, &apiKeyID, nil):                          apiKeyPolicy,
			fakePolicyLookupKey(ScopeAPIKeyRoute, &apiKeyID, stringPointer("report")): apiKeyRoutePolicy,
		},
	}

	resolver := NewResolver(store)
	resolution, found, err := resolver.Resolve(context.Background(), ResolveInput{
		APIKeyID: &apiKeyID,
		RouteID:  "report",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !found {
		t.Fatalf("expected policy to be found")
	}
	if resolution.Policy.ID != apiKeyRoutePolicy.ID {
		t.Fatalf("expected api_key_route policy %s, got %s", apiKeyRoutePolicy.ID, resolution.Policy.ID)
	}
	if resolution.MatchedScopeType != ScopeAPIKeyRoute {
		t.Fatalf("expected matched scope %q, got %q", ScopeAPIKeyRoute, resolution.MatchedScopeType)
	}
}

func TestResolverFallsBackThroughScopes(t *testing.T) {
	apiKeyID := uuid.MustParse("06a3cf27-cba0-4c09-a0c4-c4492d3ed84f")

	t.Run("api_key over route", func(t *testing.T) {
		apiKeyPolicy := Policy{ID: uuid.MustParse("abeb50ec-bcb5-4732-84e1-cad8e6886f05"), ScopeType: ScopeAPIKey}
		routePolicy := Policy{ID: uuid.MustParse("345bb515-26e2-46ee-bc07-c6e662e4dc45"), ScopeType: ScopeRoute}
		store := fakeProjectedPolicyReader{
			items: map[string]Policy{
				fakePolicyLookupKey(ScopeAPIKey, &apiKeyID, nil):              apiKeyPolicy,
				fakePolicyLookupKey(ScopeRoute, nil, stringPointer("orders")): routePolicy,
			},
		}

		resolution, found, err := NewResolver(store).Resolve(context.Background(), ResolveInput{
			APIKeyID: &apiKeyID,
			RouteID:  "orders",
		})
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if !found {
			t.Fatalf("expected policy to be found")
		}
		if resolution.Policy.ID != apiKeyPolicy.ID {
			t.Fatalf("expected api_key policy %s, got %s", apiKeyPolicy.ID, resolution.Policy.ID)
		}
	})

	t.Run("route over global", func(t *testing.T) {
		globalPolicy := Policy{ID: uuid.MustParse("74ec3e43-a5c4-4269-8200-5b8eb2376f24"), ScopeType: ScopeGlobal}
		routePolicy := Policy{ID: uuid.MustParse("46c66838-2fdc-4d01-96f9-f122f0f7e149"), ScopeType: ScopeRoute}
		store := fakeProjectedPolicyReader{
			items: map[string]Policy{
				fakePolicyLookupKey(ScopeGlobal, nil, nil):                  globalPolicy,
				fakePolicyLookupKey(ScopeRoute, nil, stringPointer("ping")): routePolicy,
			},
		}

		resolution, found, err := NewResolver(store).Resolve(context.Background(), ResolveInput{
			RouteID: "ping",
		})
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if !found {
			t.Fatalf("expected policy to be found")
		}
		if resolution.Policy.ID != routePolicy.ID {
			t.Fatalf("expected route policy %s, got %s", routePolicy.ID, resolution.Policy.ID)
		}
	})

	t.Run("no policy", func(t *testing.T) {
		resolution, found, err := NewResolver(fakeProjectedPolicyReader{}).Resolve(context.Background(), ResolveInput{
			RouteID: "report",
		})
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if found {
			t.Fatalf("expected no policy, got %#v", resolution)
		}
	})
}

func TestResolverRejectsInvalidRouteID(t *testing.T) {
	_, _, err := NewResolver(fakeProjectedPolicyReader{}).Resolve(context.Background(), ResolveInput{
		RouteID: "unknown",
	})
	if !errors.Is(err, ErrInvalidRouteID) {
		t.Fatalf("expected ErrInvalidRouteID, got %v", err)
	}
}

type fakeProjectedPolicyReader struct {
	items map[string]Policy
	err   error
}

func (f fakeProjectedPolicyReader) GetProjectedPolicy(_ context.Context, scopeType string, scopeIdentifier *uuid.UUID, routePattern *string) (Policy, bool, error) {
	if f.err != nil {
		return Policy{}, false, f.err
	}

	item, ok := f.items[fakePolicyLookupKey(scopeType, scopeIdentifier, routePattern)]
	return item, ok, nil
}

func fakePolicyLookupKey(scopeType string, scopeIdentifier *uuid.UUID, routePattern *string) string {
	scopeKey := "default"
	if scopeIdentifier != nil {
		scopeKey = scopeIdentifier.String()
	}

	routeKey := "ALL"
	if routePattern != nil {
		routeKey = *routePattern
	}

	return scopeType + ":" + scopeKey + ":" + routeKey
}
