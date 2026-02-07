package keyApp

type (
	GenerateKeyCommand struct{}
	SetKeyCommand      struct{}
)

type ActivateKeyCommand struct {
	ID string
}
type DeactivateKeyCommand struct {
	ID string
}

type VerifyKeyCommand struct {
	Key string
}
