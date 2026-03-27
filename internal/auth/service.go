package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	dbsqlc "github.com/joe/distributed-rate-limiter/internal/db/sqlc"
)

var (
	ErrAPIKeyNotFound    = errors.New("api key not found")
	ErrInvalidAPIKeyName = errors.New("api key name is required")
	ErrInvalidRawAPIKey  = errors.New("raw api key is required")
	ErrUserNotFound      = errors.New("user not found")
)

type APIKey struct {
	ID         uuid.UUID
	UserID     *uuid.UUID
	Name       string
	KeyPrefix  string
	IsActive   bool
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

type CreatedAPIKey struct {
	APIKey APIKey
	RawKey string
}

type CreateAPIKeyInput struct {
	Name   string
	UserID *uuid.UUID
}

type APIKeyCache interface {
	GetByHash(ctx context.Context, keyHash string) (APIKey, bool, error)
	SetByHash(ctx context.Context, keyHash string, apiKey APIKey) error
	DeleteByHash(ctx context.Context, keyHash string) error
}

type APIKeyService struct {
	queries *dbsqlc.Queries
	codec   *APIKeyCodec
	cache   APIKeyCache
	logger  *slog.Logger
}

func NewAPIKeyService(queries *dbsqlc.Queries, codec *APIKeyCodec, cache APIKeyCache, logger *slog.Logger) *APIKeyService {
	if logger == nil {
		logger = slog.Default()
	}

	if cache == nil {
		cache = noopAPIKeyCache{}
	}

	return &APIKeyService{
		queries: queries,
		codec:   codec,
		cache:   cache,
		logger:  logger,
	}
}

func (s *APIKeyService) Create(ctx context.Context, input CreateAPIKeyInput) (CreatedAPIKey, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CreatedAPIKey{}, ErrInvalidAPIKeyName
	}

	rawKey, keyPrefix, err := s.codec.Generate()
	if err != nil {
		return CreatedAPIKey{}, fmt.Errorf("generate api key: %w", err)
	}

	keyHash := s.codec.Hash(rawKey)
	record, err := s.queries.CreateAPIKey(ctx, dbsqlc.CreateAPIKeyParams{
		UserID:    nullableUUID(input.UserID),
		Name:      name,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
	})
	if err != nil {
		return CreatedAPIKey{}, translateWriteError(err)
	}

	apiKey := apiKeyFromRecord(record)
	if err := s.cache.SetByHash(ctx, keyHash, apiKey); err != nil {
		s.logger.Warn("failed to warm api key auth cache", slog.String("error", err.Error()), slog.String("api_key_id", apiKey.ID.String()))
	}

	return CreatedAPIKey{
		APIKey: apiKey,
		RawKey: rawKey,
	}, nil
}

func (s *APIKeyService) List(ctx context.Context) ([]APIKey, error) {
	records, err := s.queries.ListAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}

	apiKeys := make([]APIKey, 0, len(records))
	for _, record := range records {
		apiKeys = append(apiKeys, apiKeyFromRecord(record))
	}

	return apiKeys, nil
}

func (s *APIKeyService) Deactivate(ctx context.Context, id uuid.UUID) (APIKey, error) {
	existing, err := s.queries.GetAPIKeyByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return APIKey{}, ErrAPIKeyNotFound
		}

		return APIKey{}, fmt.Errorf("get api key: %w", err)
	}

	record, err := s.queries.DeactivateAPIKey(ctx, id)
	if err != nil {
		return APIKey{}, fmt.Errorf("deactivate api key: %w", err)
	}

	apiKey := apiKeyFromRecord(record)
	if err := s.cache.DeleteByHash(ctx, existing.KeyHash); err != nil {
		s.logger.Warn("failed to invalidate api key auth cache", slog.String("error", err.Error()), slog.String("api_key_id", apiKey.ID.String()))
	}

	return apiKey, nil
}

func (s *APIKeyService) ResolveActiveByRawKey(ctx context.Context, rawKey string) (APIKey, error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return APIKey{}, ErrInvalidRawAPIKey
	}

	keyHash := s.codec.Hash(rawKey)
	if cached, found, err := s.cache.GetByHash(ctx, keyHash); err == nil && found {
		return cached, nil
	} else if err != nil {
		s.logger.Warn("failed to read api key auth cache", slog.String("error", err.Error()))
	}

	record, err := s.queries.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return APIKey{}, ErrAPIKeyNotFound
		}

		return APIKey{}, fmt.Errorf("resolve api key: %w", err)
	}

	apiKey := apiKeyFromRecord(record)
	if err := s.cache.SetByHash(ctx, keyHash, apiKey); err != nil {
		s.logger.Warn("failed to backfill api key auth cache", slog.String("error", err.Error()), slog.String("api_key_id", apiKey.ID.String()))
	}

	return apiKey, nil
}

type noopAPIKeyCache struct{}

func (noopAPIKeyCache) GetByHash(context.Context, string) (APIKey, bool, error) {
	return APIKey{}, false, nil
}

func (noopAPIKeyCache) SetByHash(context.Context, string, APIKey) error {
	return nil
}

func (noopAPIKeyCache) DeleteByHash(context.Context, string) error {
	return nil
}

func translateWriteError(err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23503" {
		return ErrUserNotFound
	}

	return err
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

func apiKeyFromRecord(record dbsqlc.ApiKey) APIKey {
	return APIKey{
		ID:         record.ID,
		UserID:     pointerUUID(record.UserID),
		Name:       record.Name,
		KeyPrefix:  record.KeyPrefix,
		IsActive:   record.IsActive,
		CreatedAt:  record.CreatedAt.Time.UTC(),
		LastUsedAt: pointerTime(record.LastUsedAt),
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
