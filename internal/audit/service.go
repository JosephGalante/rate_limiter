package audit

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	dbsqlc "github.com/joe/distributed-rate-limiter/internal/db/sqlc"
)

const OutcomeBlocked = "blocked"

type BlockedRequest struct {
	APIKeyID        uuid.UUID
	RouteID         string
	PolicyID        uuid.UUID
	RequestCost     int32
	TokensRemaining int32
}

type Service struct {
	queries *dbsqlc.Queries
}

func NewService(queries *dbsqlc.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) LogBlocked(ctx context.Context, request BlockedRequest) error {
	if _, err := s.queries.CreateRequestAuditLog(ctx, dbsqlc.CreateRequestAuditLogParams{
		ApiKeyID:        uuidToPGType(request.APIKeyID),
		RouteID:         request.RouteID,
		PolicyID:        uuidToPGType(request.PolicyID),
		Outcome:         OutcomeBlocked,
		RequestCost:     request.RequestCost,
		TokensRemaining: request.TokensRemaining,
	}); err != nil {
		return fmt.Errorf("create request audit log: %w", err)
	}

	return nil
}

func uuidToPGType(value uuid.UUID) pgtype.UUID {
	var bytes [16]byte
	copy(bytes[:], value[:])

	return pgtype.UUID{
		Bytes: bytes,
		Valid: true,
	}
}
