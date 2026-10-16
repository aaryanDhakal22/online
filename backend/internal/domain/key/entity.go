package keys

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID          string
	Key         string
	Status      KeyStatus
	GeneratedAt string
}

// NewAPIKey generates a fresh random API key marked Primed (the staging
// status). Callers are expected to call Repository.UnprimeAll first so only
// the most recent generation is in Primed at any time. RNG failures are
// propagated; callers must not assume the returned struct is usable on
// non-nil error.
func NewAPIKey() (*APIKey, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("rand.Read: %w", err)
	}
	return &APIKey{
		ID:          uuid.New().String(),
		Key:         base64.RawURLEncoding.EncodeToString(b),
		Status:      Primed,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// FromExisting builds an APIKey value from a known plaintext (e.g. seeding
// from DEFAULT_API_KEY on first boot, or break-glass setKey path). The
// status is fixed to Active.
func FromExisting(plain string) *APIKey {
	return &APIKey{
		ID:          uuid.New().String(),
		Key:         plain,
		Status:      Active,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}
