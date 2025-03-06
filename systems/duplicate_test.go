package systems

import (
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

	cache.AddItem(idExists)

	{
		exists, err := cache.Exists(idExists)
		require.NoError(t, err)
		require.True(t, exists)
	}

	{
		exists, err := cache.Exists(idNotExists)
		require.NoError(t, err)
		require.False(t, exists)
	}
}

func TestEvict(t *testing.T) {
	cache, err := newMemoryCache()
	require.NoError(t, err)

	id := uuid.NewString()

	cache.AddItem(id)
	cache.items[id] = time.Now().Add(-10 * time.Hour)
	cache.evict()

	exists, err := cache.Exists(id)
	require.NoError(t, err)
	require.False(t, exists)
}
