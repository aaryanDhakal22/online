package keyApp

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	keys "quicc/online/internal/domain/key"
	"quicc/online/internal/observability"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func newTestSvc(t *testing.T, repo keys.Repository) *KeyService {
	t.Helper()
	meters, err := observability.NewMeters()
	if err != nil {
		t.Fatalf("NewMeters: %v", err)
	}
	return NewKeyService(repo, zerolog.Nop(), meters)
}

type fakeRepo struct {
	rows map[string]*keys.APIKey

	getActiveErr error
	getPrimedErr error

	createCount     int
	activateCount   int
	deactivateCount int
	unprimeCount    int
}

func (f *fakeRepo) Create(_ context.Context, key *keys.APIKey) error {
	f.createCount++
	if f.rows == nil {
		f.rows = map[string]*keys.APIKey{}
	}
	f.rows[key.ID] = key
	return nil
}

func (f *fakeRepo) GetActive(_ context.Context) (*keys.APIKey, error) {
	if f.getActiveErr != nil {
		return nil, f.getActiveErr
	}
	for _, k := range f.rows {
		if k.Status == keys.Active {
			return k, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (f *fakeRepo) GetPrimed(_ context.Context) (*keys.APIKey, error) {
	if f.getPrimedErr != nil {
		return nil, f.getPrimedErr
	}
	for _, k := range f.rows {
		if k.Status == keys.Primed {
			return k, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (*keys.APIKey, error) {
	k, ok := f.rows[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return k, nil
}

func (f *fakeRepo) Activate(_ context.Context, id string) error {
	f.activateCount++
	k, ok := f.rows[id]
	if !ok {
		return sql.ErrNoRows
	}
	k.Status = keys.Active
	return nil
}

func (f *fakeRepo) DeactivateAllKeys(_ context.Context) error {
	f.deactivateCount++
	for _, k := range f.rows {
		if k.Status == keys.Active {
			k.Status = keys.Inactive
		}
	}
	return nil
}

func (f *fakeRepo) UnprimeAll(_ context.Context) error {
	f.unprimeCount++
	for _, k := range f.rows {
		if k.Status == keys.Primed {
			k.Status = keys.Inactive
		}
	}
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	delete(f.rows, id)
	return nil
}

func TestBootstrap_SeedsDefaultWhenEmpty(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestSvc(t, repo)

	err := svc.Bootstrap(context.Background(), "default-plain")
	assert.NoError(t, err)
	assert.Equal(t, 1, repo.createCount)

	ok, err := svc.Verify(context.Background(), "default-plain")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestBootstrap_NoOpWhenActiveExists(t *testing.T) {
	existing := keys.FromExisting("already-here")
	repo := &fakeRepo{rows: map[string]*keys.APIKey{existing.ID: existing}}
	svc := newTestSvc(t, repo)

	err := svc.Bootstrap(context.Background(), "default-plain")
	assert.NoError(t, err)
	assert.Equal(t, 0, repo.createCount)

	ok, err := svc.Verify(context.Background(), "already-here")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestBootstrap_PropagatesUnexpectedError(t *testing.T) {
	repo := &fakeRepo{getActiveErr: errors.New("db on fire")}
	svc := newTestSvc(t, repo)

	err := svc.Bootstrap(context.Background(), "default-plain")
	assert.Error(t, err)
}

func TestVerify_RejectsWrongKey(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestSvc(t, repo)
	_ = svc.Bootstrap(context.Background(), "the-real-one")

	ok, err := svc.Verify(context.Background(), "imposter")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestGenerateThenSet_RotatesActive(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestSvc(t, repo)
	_ = svc.Bootstrap(context.Background(), "old-key")

	gen, err := svc.Generate(context.Background(), GenerateKeyCommand{Key: "new-primed"})
	assert.NoError(t, err)
	assert.NotEmpty(t, gen.ID)
	assert.Equal(t, 1, repo.unprimeCount)

	ok, _ := svc.Verify(context.Background(), "new-primed")
	assert.False(t, ok)

	res, err := svc.Set(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, gen.ID, res.ID)
	assert.Equal(t, "new-primed", res.Key)

	ok, _ = svc.Verify(context.Background(), "new-primed")
	assert.True(t, ok)
	ok, _ = svc.Verify(context.Background(), "old-key")
	assert.False(t, ok)
}

func TestSetByKey_BreakGlassActivatesPasted(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestSvc(t, repo)
	_ = svc.Bootstrap(context.Background(), "before")

	res, err := svc.SetByKey(context.Background(), "pasted-from-cli")
	assert.NoError(t, err)
	assert.Equal(t, "pasted-from-cli", res.Key)

	ok, _ := svc.Verify(context.Background(), "pasted-from-cli")
	assert.True(t, ok)
	ok, _ = svc.Verify(context.Background(), "before")
	assert.False(t, ok)
}

func TestSetByKey_RejectsEmpty(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestSvc(t, repo)

	_, err := svc.SetByKey(context.Background(), "")
	assert.Error(t, err)
}
