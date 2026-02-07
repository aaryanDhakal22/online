package keyApp

type GenerateKeyResult struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type SetKeyResult struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}

// Result
type GetActiveKeyResult struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}
type GetPrimedKeyResult struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}
type GetKeyByIDResult struct {
	ID  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}
type ActivateKeyResult struct {
	ID string `json:"id,omitempty"`
}
type DeactivateKeyResult struct {
	ID string `json:"id,omitempty"`
}
type VerifyKeyResult struct {
	Match bool `json:"match"`
}
