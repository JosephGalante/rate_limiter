package auth

import "context"

type contextKey string

const apiKeyContextKey contextKey = "authenticated_api_key"

func WithAPIKey(ctx context.Context, apiKey APIKey) context.Context {
	return context.WithValue(ctx, apiKeyContextKey, apiKey)
}

func APIKeyFromContext(ctx context.Context) (APIKey, bool) {
	apiKey, ok := ctx.Value(apiKeyContextKey).(APIKey)
	return apiKey, ok
}
