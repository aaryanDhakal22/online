package keyApp

type GenerateKeyResult struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type SetKeyResult struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}

type GetKeyResult struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key"`
}

type VerifyKeyResult struct {
	Match bool `json:"match"`
}

// KeyInfo is a key's metadata for the admin list — never the plaintext.
type KeyInfo struct {
	ID         string  `json:"id"`
	Status     string  `json:"status"`
	CreatedAt  *string `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}
