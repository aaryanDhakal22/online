package order

import "encoding/json"

type Order struct {
	ID          string
	Payload     json.RawMessage
	DateCreated string
	CreatedAt   string
}

func NewOrder(id string, payload json.RawMessage, dateCreated string, createdAt string) *Order {
	return &Order{
		ID:          id,
		Payload:     payload,
		DateCreated: dateCreated,
		CreatedAt:   createdAt,
	}
}
