package policies

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	dbsqlc "github.com/joe/distributed-rate-limiter/internal/db/sqlc"
)

const (
	ScopeGlobal      = "global"
	ScopeAPIKey      = "api_key"
	ScopeRoute       = "route"
	ScopeAPIKeyRoute = "api_key_route"
)

var (
	ErrInvalidScopeType      = errors.New("scope_type is invalid")
	ErrInvalidScopeShape     = errors.New("scope_type requires a different scope_identifier and route_pattern combination")
	ErrInvalidRoutePattern   = errors.New("route_pattern must be one of ping, orders, or report")
	ErrInvalidCapacity       = errors.New("capacity must be greater than zero")
	ErrInvalidRefillTokens   = errors.New("refill_tokens must be greater than zero")
	ErrInvalidRefillInterval = errors.New("refill_interval_seconds must be greater than zero")
	ErrPolicyConflict        = errors.New("an active policy already exists for that scope")
	ErrScopedAPIKeyNotFound  = errors.New("scope_identifier does not reference an existing api key")
)

var validRoutePatterns = map[string]struct{}{
	"ping":   {},
	"orders": {},
	"report": {},
}

type Policy struct {
	ID                    uuid.UUID
	ScopeType             string
	ScopeIdentifier       *uuid.UUID
	RoutePattern          *string
	Capacity              int32
	RefillTokens          int32
	RefillIntervalSeconds int32
	IsActive              bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type CreatePolicyInput struct {
	ScopeType             string
	ScopeIdentifier       *uuid.UUID
	RoutePattern          *string
	Capacity              int32
	RefillTokens          int32
	RefillIntervalSeconds int32
}

type Service struct {
	queries *dbsqlc.Queries
}

func NewService(queries *dbsqlc.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) Create(ctx context.Context, input CreatePolicyInput) (Policy, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return Policy{}, err
	}

	record, err := s.queries.CreateRateLimitPolicy(ctx, dbsqlc.CreateRateLimitPolicyParams{
		ScopeType:             normalized.ScopeType,
		ScopeIdentifier:       nullableUUID(normalized.ScopeIdentifier),
		RoutePattern:          normalized.RoutePattern,
		Capacity:              normalized.Capacity,
		RefillTokens:          normalized.RefillTokens,
		RefillIntervalSeconds: normalized.RefillIntervalSeconds,
	})
	if err != nil {
		return Policy{}, translateWriteError(err)
	}

	return policyFromRecord(record), nil
}

func (s *Service) List(ctx context.Context) ([]Policy, error) {
	records, err := s.queries.ListRateLimitPolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}

	policies := make([]Policy, 0, len(records))
	for _, record := range records {
		policies = append(policies, policyFromRecord(record))
	}

	return policies, nil
}

func validateCreateInput(input CreatePolicyInput) (CreatePolicyInput, error) {
	input.ScopeType = strings.TrimSpace(input.ScopeType)
	if input.ScopeType == "" {
		return CreatePolicyInput{}, ErrInvalidScopeType
	}

	if input.RoutePattern != nil {
		trimmed := strings.TrimSpace(*input.RoutePattern)
		input.RoutePattern = &trimmed
	}

	if input.Capacity <= 0 {
		return CreatePolicyInput{}, ErrInvalidCapacity
	}

	if input.RefillTokens <= 0 {
		return CreatePolicyInput{}, ErrInvalidRefillTokens
	}

	if input.RefillIntervalSeconds <= 0 {
		return CreatePolicyInput{}, ErrInvalidRefillInterval
	}

	switch input.ScopeType {
	case ScopeGlobal:
		if input.ScopeIdentifier != nil || input.RoutePattern != nil {
			return CreatePolicyInput{}, ErrInvalidScopeShape
		}
	case ScopeAPIKey:
		if input.ScopeIdentifier == nil || input.RoutePattern != nil {
			return CreatePolicyInput{}, ErrInvalidScopeShape
		}
	case ScopeRoute:
		if input.ScopeIdentifier != nil || input.RoutePattern == nil {
			return CreatePolicyInput{}, ErrInvalidScopeShape
		}
	case ScopeAPIKeyRoute:
		if input.ScopeIdentifier == nil || input.RoutePattern == nil {
			return CreatePolicyInput{}, ErrInvalidScopeShape
		}
	default:
		return CreatePolicyInput{}, ErrInvalidScopeType
	}

	if input.RoutePattern != nil {
		if _, ok := validRoutePatterns[*input.RoutePattern]; !ok {
			return CreatePolicyInput{}, ErrInvalidRoutePattern
		}
	}

	return input, nil
}

func translateWriteError(err error) error {
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) {
		return err
	}

	switch pgError.Code {
	case "23503":
		return ErrScopedAPIKeyNotFound
	case "23505":
		return ErrPolicyConflict
	default:
		return err
	}
}

func nullableUUID(value *uuid.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}

	var bytes [16]byte
	copy(bytes[:], value[:])

	return pgtype.UUID{
		Bytes: bytes,
		Valid: true,
	}
}

func pointerUUID(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}

	id := uuid.UUID(value.Bytes)
	return &id
}

func pointerTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	timestamp := value.Time.UTC()
	return &timestamp
}

func policyFromRecord(record dbsqlc.RateLimitPolicy) Policy {
	policy := Policy{
		ID:                    record.ID,
		ScopeType:             record.ScopeType,
		ScopeIdentifier:       pointerUUID(record.ScopeIdentifier),
		RoutePattern:          record.RoutePattern,
		Capacity:              record.Capacity,
		RefillTokens:          record.RefillTokens,
		RefillIntervalSeconds: record.RefillIntervalSeconds,
		IsActive:              record.IsActive,
	}

	if createdAt := pointerTime(record.CreatedAt); createdAt != nil {
		policy.CreatedAt = *createdAt
	}

	if updatedAt := pointerTime(record.UpdatedAt); updatedAt != nil {
		policy.UpdatedAt = *updatedAt
	}

	return policy
}
