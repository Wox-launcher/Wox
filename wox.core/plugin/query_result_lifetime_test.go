package plugin

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestQueryResultLifetime follows the same eviction paths used by typing, hide and destruction.
func TestQueryResultLifetime(t *testing.T) {
	store := newQueryResultStore()
	released := map[string]int{}
	registerQuery := func(session, id string, age int64) func(any, func()) {
		store.startQuery(Query{SessionId: session, Id: id})
		set, _ := store.getQuery(session, id)
		set.StartedAt = age
		register, managed := QueryResultLifetime(store.withQueryResultLifetime(context.Background(), session, id))
		require.True(t, managed)
		register("host", func() { released[id]++ })
		return register
	}
	registerQuery("other", "other", 1)
	late := registerQuery("main", "old", 1)
	for i := 0; i < maxCachedQueriesPerSession; i++ {
		registerQuery("main", fmt.Sprint(i), int64(i+2))
	}
	require.Equal(t, 1, released["old"])
	require.Zero(t, released["other"])
	late("late-host", func() { released["late"]++ })
	require.Equal(t, 1, released["late"])
	keep, count := store.trimHidden("main")
	require.Equal(t, "31", keep)
	require.Equal(t, 31, count)
	require.Zero(t, released[keep])
	store.clearSession("main")
	require.Equal(t, 1, released[keep])
	require.Zero(t, released["other"])
	store.clearSession("main")
	require.Equal(t, 1, released[keep])
}

// TestQueryResultLateRegistration covers a host RPC finishing while its session is destroyed.
func TestQueryResultLateRegistration(t *testing.T) {
	for i := 0; i < 100; i++ {
		store := newQueryResultStore()
		store.startQuery(Query{SessionId: "s", Id: "q"})
		register, _ := QueryResultLifetime(store.withQueryResultLifetime(context.Background(), "s", "q"))
		var calls atomic.Int32
		var work sync.WaitGroup
		work.Add(2)
		go func() {
			defer work.Done()
			register("host", func() { calls.Add(1) })
		}()
		go func() {
			defer work.Done()
			store.clearSession("s")
		}()
		work.Wait()
		require.EqualValues(t, 1, calls.Load())
	}
	store := newQueryResultStore()
	register, managed := QueryResultLifetime(store.withQueryResultLifetime(context.Background(), "gone", "gone"))
	require.True(t, managed)
	called := false
	register("host", func() { called = true })
	require.True(t, called)
	_, managed = QueryResultLifetime(context.Background())
	require.False(t, managed)
}
