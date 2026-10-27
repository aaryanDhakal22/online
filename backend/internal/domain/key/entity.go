package keys

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TimeLayout is how key timestamps are stored: UTC, fixed width, so the
// strings also sort correctly.
const TimeLayout = "2006-01-02T15:04:05.000000Z"

type APIKey struct {
	ID     string
	Key    string
	Status KeyStatus
	// CreatedAt is zero for rows from before the key guard (002).
	CreatedAt time.Time
	// LastUsedAt is the last successful vendor auth with this key; zero if
	// never used since the key guard shipped.
	LastUsedAt time.Time
}

// Now is the clock used for key timestamps (tests may replace it).
var Now = func() time.Time { return time.Now().UTC() }

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
		ID:        uuid.New().String(),
		Key:       base64.RawURLEncoding.EncodeToString(b),
		Status:    Primed,
		CreatedAt: Now(),
	}, nil
}

// FromExisting builds an APIKey value from a known plaintext (e.g. seeding
// from DEFAULT_API_KEY on first boot, or the setKey path). The status is
// fixed to Active.
func FromExisting(plain string) *APIKey {
	return &APIKey{
		ID:        uuid.New().String(),
		Key:       plain,
		Status:    Active,
		CreatedAt: Now(),
	}
}
