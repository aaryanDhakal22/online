package keys

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

type APIKey struct {
	ID          string
	Key         string
	Status      KeyStatus
	GeneratedAt string
}

func NewAPIKey() (*APIKey, error) {
	newKey := new(APIKey)
	newKey.ID = fmt.Sprintf("%x", time.Now().UnixNano())
	newKey.Status = Primed
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		fmt.Printf("Error: %v\n", err)
		return &APIKey{}, err
	}
	newKey.Key = base64.RawURLEncoding.EncodeToString(b)
	newKey.GeneratedAt = time.Now().Format(time.RFC3339)
	return newKey, nil
}
