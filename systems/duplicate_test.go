package systems

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAll(t *testing.T) {
	cache, err := newMemoryCache()
	require.NoError(t, err)

	idExists := uuid.NewString()
	idNotExists := uuid.NewString()

	ctx := context.Background()

	require.NoError(t, cache.AddItem(ctx, idExists))

	{
		exists, err := cache.Exists(ctx, idExists)
		require.NoError(t, err)
		require.True(t, exists)
	}

	{
		exists, err := cache.Exists(ctx, idNotExists)
		require.NoError(t, err)
		require.False(t, exists)
	}
}

func TestEvict(t *testing.T) {
	cache, err := newMemoryCache()
	require.NoError(t, err)

	id := uuid.NewString()

	ctx := context.Background()

	require.NoError(t, cache.AddItem(ctx, id))
	cache.items[id] = time.Now().Add(-10 * time.Hour)
	cache.evict()

	exists, err := cache.Exists(ctx, id)
	require.NoError(t, err)
	require.False(t, exists)
}
