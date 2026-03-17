package keyApp

type (
	GenerateKeyCommand struct {
		Key string
	}
	SetKeyCommand struct{}
)

type (
	RetrieveKeyCommand struct{}
	ActivateKeyCommand struct {
		ID string
	}
)

type DeactivateKeyCommand struct {
	ID string
}

type VerifyKeyCommand struct {
	Key string
}
type GetKeyCommand struct{}
