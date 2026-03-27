package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/joe/distributed-rate-limiter/internal/auth"
	"github.com/joe/distributed-rate-limiter/internal/config"
)

type DemoAPIKeyResolver interface {
	ResolveActiveByRawKey(ctx context.Context, rawKey string) (auth.APIKey, error)
}

type PublicConfigHandler struct {
	demoConfig config.DemoConfig
	resolver   DemoAPIKeyResolver
}

func NewPublicConfigHandler(demoConfig config.DemoConfig, resolver DemoAPIKeyResolver) *PublicConfigHandler {
	return &PublicConfigHandler{
		demoConfig: demoConfig,
		resolver:   resolver,
	}
}

func (h *PublicConfigHandler) Show(w http.ResponseWriter, r *http.Request) {
	response := map[string]any{
		"public_demo_mode":        h.demoConfig.PublicMode,
		"admin_mutations_enabled": !h.demoConfig.PublicMode,
		"demo_api_key":            nil,
	}

	if !h.demoConfig.PublicMode || h.resolver == nil {
		writeJSON(w, http.StatusOK, response)
		return
	}

	rawKey := strings.TrimSpace(h.demoConfig.RawAPIKey)
	if rawKey == "" {
		writeJSON(w, http.StatusOK, response)
		return
	}

	apiKey, err := h.resolver.ResolveActiveByRawKey(r.Context(), rawKey)
	switch {
	case err == nil:
		response["demo_api_key"] = map[string]any{
			"id":         apiKey.ID.String(),
			"name":       apiKey.Name,
			"key_prefix": apiKey.KeyPrefix,
			"is_active":  apiKey.IsActive,
			"created_at": apiKey.CreatedAt.UTC().Format(time.RFC3339),
			"raw_key":    rawKey,
		}
	case errors.Is(err, auth.ErrAPIKeyNotFound), errors.Is(err, auth.ErrInvalidRawAPIKey):
		// Keep the response usable even if the configured demo key is missing.
	default:
		WriteInternalServerError(w)
		return
	}

	writeJSON(w, http.StatusOK, response)
}
