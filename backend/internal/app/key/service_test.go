package keyApp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	keys "quicc/online/internal/domain/key"
	"quicc/online/internal/infra/database/models"
	"quicc/online/internal/infra/database/repositories"
	"quicc/online/internal/migrations"
	"quicc/online/internal/observability"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// fakeClock replaces keys.Now; every call advances it by a millisecond so
// timestamps are strictly ordered.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(time.Millisecond)
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type fixture struct {
	db    *sql.DB
	repo  *repositories.KeyRepository
	svc   *KeyService
	clock *fakeClock
}

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1) // one connection = one in-memory database
	t.Cleanup(func() { db.Close() })
	require.NoError(t, migrations.Apply(context.Background(), db))
	return db
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	prev := keys.Now
	keys.Now = clock.now
	t.Cleanup(func() { keys.Now = prev })

	db := newDB(t)
	repo := repositories.NewKeyRepository(models.New(db), zerolog.Nop())
	meters, err := observability.NewMeters()
	require.NoError(t, err)
	return &fixture{db: db, repo: repo, svc: NewKeyService(repo, zerolog.Nop(), meters), clock: clock}
}

func (f *fixture) verify(t *testing.T, plain string) bool {
	t.Helper()
	ok, err := f.svc.Verify(context.Background(), plain)
	require.NoError(t, err)
	return ok
}

func (f *fixture) activeCount(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE status='active'`).Scan(&n))
	return n
}

func (f *fixture) idOf(t *testing.T, plain string) string {
	t.Helper()
	var id string
	require.NoError(t, f.db.QueryRow(`SELECT id FROM api_keys WHERE key=?`, plain).Scan(&id))
	return id
}

// ── migrations + triggers ─────────────────────────────────────────────────

func TestMigrations_IdempotentAndUpgradeInPlace(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	defer db.Close()
	ctx := context.Background()

	// A pre-guard database: 001's table, one active key.
	_, err = db.Exec(`CREATE TABLE api_keys (id VARCHAR(255) NOT NULL, key VARCHAR(255) NOT NULL, status VARCHAR(255) NOT NULL, PRIMARY KEY (id));
		INSERT INTO api_keys (id, key, status) VALUES ('legacy', 'live-vendor-key', 'active');`)
	require.NoError(t, err)

	require.NoError(t, migrations.Apply(ctx, db))
	require.NoError(t, migrations.Apply(ctx, db)) // every boot

	var key string
	var created, used sql.NullString
	require.NoError(t, db.QueryRow(`SELECT key, created_at, last_used_at FROM api_keys WHERE id='legacy'`).Scan(&key, &created, &used))
	assert.Equal(t, "live-vendor-key", key)
	assert.False(t, created.Valid)
	assert.False(t, used.Valid)

	var triggers int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name LIKE 'api_keys_keep_last_active%'`).Scan(&triggers))
	assert.Equal(t, 3, triggers)

	_, err = db.Exec(`DELETE FROM api_keys WHERE id='legacy'`)
	assert.ErrorContains(t, err, "last_active_key")
}

func TestTriggers_GuardTheOnlyActiveKey(t *testing.T) {
	db := newDB(t)
	_, err := db.Exec(`INSERT INTO api_keys (id, key, status) VALUES ('a', 'ka', 'active'), ('p', 'kp', 'primed'), ('i', 'ki', 'inactive')`)
	require.NoError(t, err)

	count := func() int {
		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE status='active'`).Scan(&n))
		return n
	}

	_, err = db.Exec(`DELETE FROM api_keys WHERE id='a'`)
	assert.ErrorContains(t, err, "last_active_key", "delete the only active key")
	_, err = db.Exec(`UPDATE api_keys SET status='inactive' WHERE id='a'`)
	assert.ErrorContains(t, err, "last_active_key", "deactivate the only active key")
	_, err = db.Exec(`UPDATE api_keys SET status='primed' WHERE id='a'`)
	assert.ErrorContains(t, err, "last_active_key", "demote the only active key")
	_, err = db.Exec(`UPDATE api_keys SET key='overwritten' WHERE id='a'`)
	assert.ErrorContains(t, err, "last_active_key", "overwrite the only active key")
	_, err = db.Exec(`DELETE FROM api_keys`)
	assert.ErrorContains(t, err, "last_active_key", "wipe the table")
	assert.Equal(t, 1, count())

	// Non-active rows are free to change.
	_, err = db.Exec(`DELETE FROM api_keys WHERE id IN ('p', 'i')`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE api_keys SET last_used_at='x' WHERE id='a'`)
	require.NoError(t, err)

	// Two active: the old "deactivate every active key" statement aborts as a whole.
	_, err = db.Exec(`INSERT INTO api_keys (id, key, status) VALUES ('b', 'kb', 'active')`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE api_keys SET status='inactive' WHERE status='active'`)
	assert.ErrorContains(t, err, "last_active_key")
	assert.Equal(t, 2, count())

	// ...but one of two can go, and then the survivor is protected.
	_, err = db.Exec(`UPDATE api_keys SET status='inactive' WHERE id='a'`)
	require.NoError(t, err)
	_, err = db.Exec(`DELETE FROM api_keys WHERE id='b'`)
	assert.ErrorContains(t, err, "last_active_key")
	assert.Equal(t, 1, count())
}

func TestRepo_RetireMapsGuardErrors(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	only := keys.FromExisting("only")
	require.NoError(t, f.repo.Create(ctx, only))

	assert.ErrorIs(t, f.repo.Retire(ctx, only.ID), keys.ErrLastActiveKey)
	assert.ErrorIs(t, f.repo.Retire(ctx, "nope"), keys.ErrNotActive)
	_, err := f.repo.GetByID(ctx, "nope")
	assert.ErrorIs(t, err, keys.ErrNotFound)
}

// ── bootstrap + verify ────────────────────────────────────────────────────

func TestBootstrap_SeedsDefaultWhenEmpty(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.svc.Bootstrap(context.Background(), "default-plain"))
	assert.Equal(t, 1, f.activeCount(t))
	assert.True(t, f.verify(t, "default-plain"))
}

func TestBootstrap_NoOpWhenActiveExists(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.repo.Create(context.Background(), keys.FromExisting("already-here")))
	require.NoError(t, f.svc.Bootstrap(context.Background(), "default-plain"))
	assert.Equal(t, 1, f.activeCount(t))
	assert.True(t, f.verify(t, "already-here"))
	assert.False(t, f.verify(t, "default-plain"))
}

func TestBootstrap_NoDefaultRejectsEverything(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.svc.Bootstrap(context.Background(), ""))
	assert.False(t, f.verify(t, ""))
	assert.False(t, f.verify(t, "anything"))
	_, err := f.svc.Get(context.Background())
	assert.ErrorIs(t, err, keys.ErrNotFound)
}

func TestVerify_RejectsWrongKey(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.svc.Bootstrap(context.Background(), "the-real-one"))
	assert.False(t, f.verify(t, "imposter"))
	assert.False(t, f.verify(t, "the-real-on"))
	assert.False(t, f.verify(t, "the-real-one-"))
}

func TestVerify_StampsLastUsedThrottled(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Bootstrap(ctx, "k"))
	id := f.idOf(t, "k")

	lastUsed := func() string {
		var s sql.NullString
		require.NoError(t, f.db.QueryRow(`SELECT last_used_at FROM api_keys WHERE id=?`, id).Scan(&s))
		return s.String
	}
	assert.Empty(t, lastUsed())
	assert.False(t, f.verify(t, "wrong"))
	assert.Empty(t, lastUsed(), "a failed auth stamps nothing")

	assert.True(t, f.verify(t, "k"))
	first := lastUsed()
	require.NotEmpty(t, first)
	assert.True(t, f.verify(t, "k"))
	assert.Equal(t, first, lastUsed(), "second use within the throttle window isn't written")

	f.clock.advance(stampEvery)
	assert.True(t, f.verify(t, "k"))
	assert.Greater(t, lastUsed(), first)
}

// ── two-step rotation ─────────────────────────────────────────────────────

func TestGenerateThenSet_AddsWithoutDeactivating(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Bootstrap(ctx, "old-key"))

	gen, err := f.svc.Generate(ctx, GenerateKeyCommand{Key: "new-primed"})
	require.NoError(t, err)
	assert.False(t, f.verify(t, "new-primed"), "primed keys don't authenticate")

	res, err := f.svc.Set(ctx)
	require.NoError(t, err)
	assert.Equal(t, gen.ID, res.ID)
	assert.Equal(t, "new-primed", res.Key)

	assert.True(t, f.verify(t, "new-primed"))
	assert.True(t, f.verify(t, "old-key"), "the old key keeps working until retired")
	assert.Equal(t, 2, f.activeCount(t))

	got, err := f.svc.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "new-primed", got.Key, "getKey returns the newest active key")
}

func TestSetByKey_AddsAndIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Bootstrap(ctx, "before"))

	res, err := f.svc.SetByKey(ctx, "pasted-from-cli")
	require.NoError(t, err)
	assert.Equal(t, "pasted-from-cli", res.Key)
	assert.True(t, f.verify(t, "pasted-from-cli"))
	assert.True(t, f.verify(t, "before"))

	again, err := f.svc.SetByKey(ctx, "pasted-from-cli")
	require.NoError(t, err)
	assert.Equal(t, res.ID, again.ID)
	assert.Equal(t, 2, f.activeCount(t))
}

func TestSetByKey_RejectsEmpty(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.SetByKey(context.Background(), "")
	assert.Error(t, err)
}

func TestRetire_OnlyActiveKeyRefused(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Bootstrap(ctx, "live"))
	assert.True(t, f.verify(t, "live"))

	err := f.svc.Retire(ctx, f.idOf(t, "live"))
	assert.ErrorIs(t, err, keys.ErrLastActiveKey)
	assert.True(t, f.verify(t, "live"))
}

func TestRetire_AddUseRetire(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Bootstrap(ctx, "old"))
	assert.True(t, f.verify(t, "old")) // the vendor is live on "old"
	oldID := f.idOf(t, "old")

	_, err := f.svc.Generate(ctx, GenerateKeyCommand{Key: "new"})
	require.NoError(t, err)
	_, err = f.svc.Set(ctx)
	require.NoError(t, err)

	// The new key hasn't been used yet: retiring the old one is refused.
	assert.ErrorIs(t, f.svc.Retire(ctx, oldID), keys.ErrNewKeyUnconfirmed)
	assert.True(t, f.verify(t, "old"))

	// The vendor switches; now the old key can go.
	assert.True(t, f.verify(t, "new"))
	require.NoError(t, f.svc.Retire(ctx, oldID))
	assert.False(t, f.verify(t, "old"))
	assert.True(t, f.verify(t, "new"))

	// ...and the new one is now the last: protected.
	assert.ErrorIs(t, f.svc.Retire(ctx, f.idOf(t, "new")), keys.ErrLastActiveKey)
	assert.ErrorIs(t, f.svc.Retire(ctx, oldID), keys.ErrNotActive)
	assert.ErrorIs(t, f.svc.Retire(ctx, "no-such-id"), keys.ErrNotFound)
}

func TestRetire_AbortRotationNeedsOldKeyUsedSinceNewAdded(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Bootstrap(ctx, "old"))
	assert.True(t, f.verify(t, "old")) // used BEFORE the new key exists

	res, err := f.svc.SetByKey(ctx, "new")
	require.NoError(t, err)

	// "old" was used, but not since "new" was added: not proof the vendor
	// is still on it.
	assert.ErrorIs(t, f.svc.Retire(ctx, res.ID), keys.ErrNewKeyUnconfirmed)

	assert.True(t, f.verify(t, "old")) // stamped again (the add reset the throttle)
	require.NoError(t, f.svc.Retire(ctx, res.ID))
	assert.False(t, f.verify(t, "new"))
	assert.True(t, f.verify(t, "old"))
}

func TestRetire_LegacyKeyWithoutTimestamps(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.db.Exec(`INSERT INTO api_keys (id, key, status) VALUES ('legacy', 'live', 'active')`)
	require.NoError(t, err)
	require.NoError(t, f.svc.Bootstrap(ctx, "ignored"))
	assert.True(t, f.verify(t, "live"))

	_, err = f.svc.SetByKey(ctx, "next")
	require.NoError(t, err)
	assert.ErrorIs(t, f.svc.Retire(ctx, "legacy"), keys.ErrNewKeyUnconfirmed)
	assert.True(t, f.verify(t, "next"))
	require.NoError(t, f.svc.Retire(ctx, "legacy"))
}

func TestList_NeverExposesPlaintext(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Bootstrap(ctx, "secret-one"))
	assert.True(t, f.verify(t, "secret-one"))
	_, err := f.svc.Generate(ctx, GenerateKeyCommand{Key: "secret-two"})
	require.NoError(t, err)

	list, err := f.svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "primed", list[0].Status) // newest first
	assert.Equal(t, "active", list[1].Status)
	require.NotNil(t, list[1].LastUsedAt)
	assert.Nil(t, list[0].LastUsedAt)

	b, err := json.Marshal(list)
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(b), "secret-"), string(b))
}

func TestVerifyConcurrentWithRotation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Bootstrap(ctx, "stable"))

	var wg sync.WaitGroup
	stop := make(chan struct{})
	var bad error
	var badMu sync.Mutex
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				ok, err := f.svc.Verify(ctx, "stable")
				if err != nil || !ok {
					badMu.Lock()
					bad = errors.Join(bad, err, errors.New("stable key rejected mid-rotation"))
					badMu.Unlock()
					return
				}
			}
		}()
	}
	for i := 0; i < 20; i++ {
		res, err := f.svc.SetByKey(ctx, "rot-"+string(rune('a'+i)))
		require.NoError(t, err)
		ok, err := f.svc.Verify(ctx, "stable") // proves the vendor is still on "stable"
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, f.svc.Retire(ctx, res.ID))
	}
	close(stop)
	wg.Wait()
	assert.NoError(t, bad)
}
