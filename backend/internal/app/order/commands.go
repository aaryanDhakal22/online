package orderApp

import "encoding/json"

type CreateOrderCommand struct {
	OrderID     string
	Payload     json.RawMessage
	DateCreated string
	CreatedAt   string
}
