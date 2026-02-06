package order

import "encoding/json"

type Order struct {
	ID          string
	Payload     string
	DateCreated string
	CreatedAt   string
}

func NewOrder(id string, payload string, dateCreated string, createdAt string) *Order {
	return &Order{
		ID:          id,
		Payload:     payload,
		DateCreated: dateCreated,
		CreatedAt:   createdAt,
	}
}

func (o *Order) Flatten() (string, error) {
	orderJSON, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	return string(orderJSON), nil
}
