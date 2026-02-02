package keys

type KeyStatus string

const (
	Inactive KeyStatus = "inactive"
	Primed   KeyStatus = "primed"
	Active   KeyStatus = "active"
	Expired  KeyStatus = "expired"
)

func ParseKeyStatus(status string) KeyStatus {
	switch status {
	case "inactive":
		return Inactive
	case "primed":
		return Primed
	case "active":
		return Active
	case "expired":
		return Expired
	default:
		return Inactive
	}
}
