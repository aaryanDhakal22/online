package keys

type APIKey struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Status      string `json:"status"`
	GeneratedAt string `json:"generated_at"`
}

func (k *APIKey) Use() error {
	k.Status = "InUse"
	return nil
}
