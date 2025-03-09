package common

import (
	"log/slog"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackpressure(t *testing.T) {
	b := NewBackpressureMonitor()

	for i := 0; i < 10; i++ {
		b.Increase("test")
		b.Log(slog.LevelDebug)
	}

	require.Equal(t, 10, b.services["test"].count)

	for i := 0; i < 15; i++ {
		b.Decrease("test")
		b.Log(slog.LevelDebug)
	}

	require.Equal(t, 0, b.services["test"].count)
}

func TestAsyncBackpressure(t *testing.T) {
	b := NewBackpressureMonitor()

	wg := &sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		b.Increase("test")
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				b.Decrease("test")
			}()

			t := time.Duration(rand.Intn(300)+200) * time.Millisecond

			slog.Info("sleeping", "duration", t.String())

			time.Sleep(t)
		}()
	}
	wg.Wait()

	require.Equal(t, 0, b.services["test"].count)
}
