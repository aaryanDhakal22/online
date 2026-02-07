package orderApp

import (
	"context"
	"errors"
	"testing"
	"time"

	"quicc/online/internal/domain/order"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

type fakeRepo struct {
	createErr  error
	getByIDErr error
	deleteErr  error

	createCall  int
	getByIDCall int
	deleteCall  int
}

func (f *fakeRepo) Create(ctx context.Context, order *order.Order) error {
	f.createCall++
	return f.createErr
}

func (f *fakeRepo) GetByID(ctx context.Context, id string) (*order.Order, error) {
	f.getByIDCall++
	return nil, f.getByIDErr
}

func (f *fakeRepo) Delete(ctx context.Context, id string) error {
	f.deleteCall++
	return f.deleteErr
}

func (f *fakeRepo) GetLatest(ctx context.Context) (*order.Order, error) {
	return nil, nil
}

type fakePublisher struct {
	publishErr  error
	publishCall int
}

func (f *fakePublisher) Publish(orderID string, order order.Order) error {
	f.publishCall++
	return f.publishErr
}

func TestCreate(t *testing.T) {
	repo := &fakeRepo{}
	mb := &fakePublisher{}
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.DebugLevel)
	service := NewOrderService(repo, mb, logger)

	repo.createErr = errors.New("error")
	_, err := service.Create(CreateOrderCommand{
		OrderID:     "1",
		Payload:     "{}",
		DateCreated: time.Now().String(),
		CreatedAt:   time.Now().String(),
	})
	assert.Error(t, err)
	assert.Equal(t, 1, repo.createCall)
	assert.Equal(t, 0, mb.publishCall)

	repo.createErr = nil
	_, err = service.Create(CreateOrderCommand{
		OrderID:     "1",
		Payload:     "{}",
		DateCreated: time.Now().String(),
		CreatedAt:   time.Now().String(),
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, repo.createCall)
	assert.Equal(t, 1, mb.publishCall)
}

func TestRelayOrder(t *testing.T) {
	repo := &fakeRepo{}
	mb := &fakePublisher{}
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.DebugLevel)
	service := NewOrderService(repo, mb, logger)

	err := service.RelayOrder(RelayOrderCommand{OrderID: "1", Order: order.Order{ID: "1"}})
	assert.Error(t, err)
	assert.Equal(t, 1, mb.publishCall)
}
