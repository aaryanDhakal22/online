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
	Key string `json:"key"`
}

type VerifyKeyResult struct {
	Match bool `json:"match"`
}
