package plugin

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wox/util"
)

type fakeLifecyclePlugin struct {
	initErr   error
	panicInit bool
	initCalls atomic.Int32
	initStart chan struct{}
	initDone  chan struct{}
}

func (f *fakeLifecyclePlugin) Init(context.Context, InitParams) {}

func (f *fakeLifecyclePlugin) Query(context.Context, Query) QueryResponse {
	return QueryResponse{}
}

func (f *fakeLifecyclePlugin) InitWithError(context.Context, InitParams) error {
	f.initCalls.Add(1)
	if f.initStart != nil {
		select {
		case <-f.initStart:
		default:
			close(f.initStart)
		}
	}
	if f.initDone != nil {
		<-f.initDone
	}
	if f.panicInit {
		panic("init boom")
	}
	return f.initErr
}

func newInitTestManager() *Manager {
	logger = util.GetLogger()
	return &Manager{}
}

func newInitTestInstance(plugin Plugin) *Instance {
	instance := &Instance{
		Plugin:   plugin,
		Metadata: Metadata{Id: "plugin-1", Name: "Fake"},
	}
	instance.beginInitCycle()
	return instance
}

func TestInitPluginRecordsHostInitError(t *testing.T) {
	manager := newInitTestManager()
	instance := newInitTestInstance(&fakeLifecyclePlugin{initErr: errors.New("host init failed")})

	manager.initPlugin(context.Background(), instance)

	assert.False(t, instance.Initialized)
	assert.EqualError(t, instance.WaitInit(context.Background()), "host init failed")
}

func TestInitPluginRecordsPanic(t *testing.T) {
	manager := newInitTestManager()
	instance := newInitTestInstance(&fakeLifecyclePlugin{panicInit: true})

	manager.initPlugin(context.Background(), instance)

	assert.False(t, instance.Initialized)
	require.Error(t, instance.InitError)
	assert.Contains(t, instance.InitError.Error(), "plugin init panic")
}

func TestWaitInitTimesOutWhileInitIsPending(t *testing.T) {
	instance := newInitTestInstance(&fakeLifecyclePlugin{})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := instance.WaitInit(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, instance.initCycleFinished())
}

func TestInitPluginMarksInitializedOnSuccess(t *testing.T) {
	manager := newInitTestManager()
	instance := newInitTestInstance(&fakeLifecyclePlugin{})

	manager.initPlugin(context.Background(), instance)

	assert.True(t, instance.Initialized)
	require.NoError(t, instance.WaitInit(context.Background()))
}

func TestActivatePluginReturnsInitError(t *testing.T) {
	manager := newInitTestManager()
	instance := newInitTestInstance(&fakeLifecyclePlugin{initErr: errors.New("still broken")})
	instance.RuntimeLoaded = true

	err := manager.activatePlugin(context.Background(), instance)

	require.EqualError(t, err, "still broken")
	assert.False(t, instance.Initialized)
}

func TestActivatePluginDoesNotOverlapPendingInit(t *testing.T) {
	manager := newInitTestManager()
	plugin := &fakeLifecyclePlugin{initStart: make(chan struct{}), initDone: make(chan struct{})}
	instance := newInitTestInstance(plugin)

	initialDone := make(chan struct{})
	go func() {
		manager.initPlugin(context.Background(), instance)
		close(initialDone)
	}()
	<-plugin.initStart

	activateDone := make(chan error, 1)
	go func() {
		activateDone <- manager.activatePlugin(context.Background(), instance)
	}()
	assert.Never(t, func() bool { return len(activateDone) > 0 }, 20*time.Millisecond, time.Millisecond)

	close(plugin.initDone)
	<-initialDone
	require.NoError(t, <-activateDone)
	assert.Equal(t, int32(1), plugin.initCalls.Load())
}
