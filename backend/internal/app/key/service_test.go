package keyApp

import (
	"context"
	"errors"
	"testing"
	"time"

	keys "quicc/online/internal/domain/key"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

type fakeRepo struct {
	createErr            error
	getActiveErr         error
	getPrimeErr          error
	getByIDErr           error
	activateErr          error
	deactivateAllKeysErr error
	unprimeAllErr        error
	deleteErr            error

	createCall            int
	getActiveCall         int
	getPrimeCall          int
	getByIDCall           int
	activateCall          int
	deactivateAllKeysCall int
	unprimeCall           int
	deleteCall            int

	data map[string]keys.APIKey
}

func (f *fakeRepo) Create(ctx context.Context, key *keys.APIKey) error {
	f.createCall++
	return f.createErr
}

func (f *fakeRepo) GetActive(ctx context.Context) (*keys.APIKey, error) {
	f.getActiveCall++
	return nil, f.getActiveErr
}

func (f *fakeRepo) GetPrimed(ctx context.Context) (*keys.APIKey, error) {
	f.getPrimeCall++
	return nil, f.getPrimeErr
}

func (f *fakeRepo) GetByID(ctx context.Context, id string) (*keys.APIKey, error) {
	f.getByIDCall++
	return nil, f.getByIDErr
}

func (f *fakeRepo) Activate(ctx context.Context, id string) error {
	f.activateCall++
	return f.activateErr
}

func (f *fakeRepo) DeactivateAllKeys(ctx context.Context) error {
	f.deactivateAllKeysCall++
	return f.deactivateAllKeysErr
}

func (f *fakeRepo) UnprimeAll(ctx context.Context) error {
	f.unprimeCall++
	return f.unprimeAllErr
}

func (f *fakeRepo) Delete(ctx context.Context, id string) error {
	f.deleteCall++
	return f.deleteErr
}

type fakeRedis struct {
	data    map[string]any
	setErr  error
	setCall int
}

func (f *fakeRedis) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	if f.data == nil {
		f.data = map[string]any{}
	}
	f.data[key] = value
	f.setCall++

	cmd := redis.NewStatusCmd(ctx)
	if f.setErr != nil {
		cmd.SetErr(f.setErr)
	} else {
		cmd.SetVal("OK")
	}
	return cmd
}

// Happy Path
func TestGenerate_AllSuccess(t *testing.T) {
	repo := &fakeRepo{}
	rd := &fakeRedis{}
	svc := NewKeyService(repo, rd, zerolog.Nop())
	svc.Generate(context.TODO(), GenerateKeyCommand{})
	assert.Equal(t, 1, repo.unprimeCall)
	assert.Equal(t, 1, repo.createCall)
}

func TestGenerate_UnprimeFails(t *testing.T) {
	repo := &fakeRepo{unprimeAllErr: errors.New("unable to unprimeall")}
	rd := &fakeRedis{}
	svc := NewKeyService(repo, rd, zerolog.Nop())
	svc.Generate(context.TODO(), GenerateKeyCommand{})
	assert.Equal(t, 1, repo.unprimeCall)
	assert.Equal(t, 0, repo.createCall)
}

func TestGenerate_CreateFails(t *testing.T) {
	repo := &fakeRepo{createErr: errors.New("unable to create")}
	rd := &fakeRedis{}
	svc := NewKeyService(repo, rd, zerolog.Nop())
	svc.Generate(context.TODO(), GenerateKeyCommand{})

	// Potentially, add a transaction here
	// assert.Equal(t, 0, repo.unprimeCall)

	assert.Equal(t, 1, repo.unprimeCall)
	assert.Equal(t, 1, repo.createCall)
}
