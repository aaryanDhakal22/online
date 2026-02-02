package keyApp

type (
	GenerateKeyCommand  struct{}
	GetActiveKeyCommand struct{}
	GetPrimedKeyCommand struct{}
	GetKeyByIDCommand   struct {
		ID string
	}
)

type ActivateKeyCommand struct {
	ID string
}
type DeactivateKeyCommand struct {
	ID string
}
