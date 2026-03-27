package auth

import (
	"strings"
	"testing"
)

func TestAPIKeyCodecGenerateAndHash(t *testing.T) {
	codec := NewAPIKeyCodec("test-pepper")

	rawKey, keyPrefix, err := codec.Generate()
	if err != nil {
		t.Fatalf("generate api key: %v", err)
	}

	if !strings.HasPrefix(rawKey, rawKeyPrefix) {
		t.Fatalf("expected raw key prefix %q, got %q", rawKeyPrefix, rawKey)
	}

	if !strings.HasPrefix(keyPrefix, rawKeyPrefix) {
		t.Fatalf("expected key prefix %q to start with %q", keyPrefix, rawKeyPrefix)
	}

	expectedPrefix := rawKey[:len(rawKeyPrefix)+displayPrefixLength]
	if keyPrefix != expectedPrefix {
		t.Fatalf("expected display prefix %q, got %q", expectedPrefix, keyPrefix)
	}

	hashA := codec.Hash(rawKey)
	hashB := codec.Hash(rawKey)
	if hashA != hashB {
		t.Fatalf("expected deterministic hash output")
	}
}
