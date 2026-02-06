package orderApp

import "quicc/online/internal/domain/order"

type RelayOrderCommand struct {
	OrderID string
	Order   order.Order
}
type CreateOrderCommand struct {
	OrderID     string
	Payload     string
	DateCreated string
	CreatedAt   string
}
