package systems

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAll(t *testing.T) {
	cache := newCache()

	idExists := uuid.NewString()
	idNotExists := uuid.NewString()

	cache.AddItem(idExists)

	require.True(t, cache.Exists(idExists))
	require.False(t, cache.Exists(idNotExists))
}

func TestEvict(t *testing.T) {
	cache := newCache()

	id := uuid.NewString()

	cache.AddItem(id)
	cache.items[id] = time.Now().Add(-10 * time.Hour)
	cache.evict()

	require.False(t, cache.Exists(id))
}
