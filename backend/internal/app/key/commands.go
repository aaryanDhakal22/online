package keyApp

type GenerateKeyCommand struct {
	Key string `json:"key"` // optional plaintext; if empty, a random key is generated
}

type SetByKeyCommand struct {
	Key string `json:"key"`
}
