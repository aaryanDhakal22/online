package keys

type KeyStatus string

const (
	Inactive KeyStatus = "inactive"
	Primed   KeyStatus = "primed"
	Active   KeyStatus = "active"
)

func ParseKeyStatus(status string) KeyStatus {
	switch status {
	case "active":
		return Active
	case "primed":
		return Primed
	default:
		return Inactive
	}
}
