package orderApp

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"quicc/online/internal/observability"
)

type fakePublisher struct {
	publishErr  error
	publishCall int
	lastID      string
	lastBody    []byte
}

func (f *fakePublisher) Publish(ctx context.Context, orderID string, body []byte) error {
	f.publishCall++
	f.lastID = orderID
	f.lastBody = body
	return f.publishErr
}

func newSvc(t *testing.T, pub *fakePublisher) *OrderService {
	t.Helper()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.DebugLevel)
	meters, err := observability.NewMeters()
	if err != nil {
		t.Fatalf("NewMeters: %v", err)
	}
	return NewOrderService(pub, logger, meters)
}

func TestRelay_Success(t *testing.T) {
	pub := &fakePublisher{}
	svc := newSvc(t, pub)

	body := []byte(`{"order":1}`)
	res, err := svc.Relay(context.Background(), RelayOrderCommand{OrderID: "abc", Body: body})

	assert.NoError(t, err)
	assert.Equal(t, 1, pub.publishCall)
	assert.Equal(t, "abc", pub.lastID)
	assert.Equal(t, body, pub.lastBody)
	assert.Equal(t, "relayed", res.Status)
	assert.Equal(t, len(body), res.BodyBytes)
}

func TestRelay_PublishError(t *testing.T) {
	pub := &fakePublisher{publishErr: errors.New("sqs down")}
	svc := newSvc(t, pub)

	_, err := svc.Relay(context.Background(), RelayOrderCommand{OrderID: "abc", Body: []byte("x")})

	assert.Error(t, err)
	assert.Equal(t, 1, pub.publishCall)
}
