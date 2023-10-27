package util

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewDebouncer(t *testing.T) {
	var flushed []string

	ctx, cancelFunc := context.WithCancel(context.Background())
	debouncer := NewDebouncer(50, func(s []string, reason string) {
		flushed = append(flushed, s...)
	})
	debouncer.Start(ctx)
	debouncer.Add(ctx, []string{"test1"})

	assert.Equal(t, len(flushed), 0)
	time.Sleep(time.Millisecond * 60)
	assert.Equal(t, len(flushed), 1)

	debouncer.Add(ctx, []string{"test2"})
	time.Sleep(time.Millisecond * 60)
	assert.Equal(t, len(flushed), 2)

	debouncer.Add(ctx, []string{"test3"})
	cancelFunc()
	time.Sleep(time.Millisecond * 60)
	assert.Equal(t, len(flushed), 2)
}
