package keyApp

type (
	GenerateKeyCommand struct{}
	SetKeyCommand      struct{}
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
