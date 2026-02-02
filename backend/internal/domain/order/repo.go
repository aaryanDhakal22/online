package order

type Repository interface {
	Create(order *Order) error
	GetByID(id int64) (*Order, error)
	GetLatest() (*Order, error)
	GetAllToday() ([]*Order, error)
	Delete(id int64) error
}
