package registry_test

import (
	"context"
	"errors"
	"testing"

	"github.com/pythondatascrape/engram/internal/plugin"
	"github.com/pythondatascrape/engram/internal/plugin/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPlugin is a test double implementing plugin.Plugin.
type mockPlugin struct {
	name      string
	ptype     plugin.Type
	builtIn   bool
	startErr  error
	stopErr   error
	healthErr error
	startCalled bool
	stopCalled  bool
}

func (m *mockPlugin) Name() string                      { return m.name }
func (m *mockPlugin) Type() plugin.Type                 { return m.ptype }
func (m *mockPlugin) BuiltIn() bool                     { return m.builtIn }
func (m *mockPlugin) Start(_ context.Context) error     { m.startCalled = true; return m.startErr }
func (m *mockPlugin) Stop(_ context.Context) error      { m.stopCalled = true; return m.stopErr }
func (m *mockPlugin) Health(_ context.Context) error    { return m.healthErr }

func newMock(name string, t plugin.Type) *mockPlugin {
	return &mockPlugin{name: name, ptype: t, builtIn: true}
}

// --- Tests ---

func TestRegisterAndGet(t *testing.T) {
	r := registry.New()
	p := newMock("alpha", plugin.TypeProvider)

	require.NoError(t, r.Register(p))

	got, err := r.Get("alpha")
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestRegisterDuplicateReturnsError(t *testing.T) {
	r := registry.New()
	p := newMock("alpha", plugin.TypeProvider)

	require.NoError(t, r.Register(p))
	err := r.Register(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alpha")
}

func TestGetNonExistentReturnsError(t *testing.T) {
	r := registry.New()
	_, err := r.Get("ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

func TestListByTypeFiltersCorrectly(t *testing.T) {
	r := registry.New()
	p1 := newMock("p1", plugin.TypeProvider)
	p2 := newMock("p2", plugin.TypeSerializer)
	p3 := newMock("p3", plugin.TypeProvider)

	require.NoError(t, r.Register(p1))
	require.NoError(t, r.Register(p2))
	require.NoError(t, r.Register(p3))

	providers := r.ListByType(plugin.TypeProvider)
	assert.Len(t, providers, 2)

	serializers := r.ListByType(plugin.TypeSerializer)
	assert.Len(t, serializers, 1)

	hooks := r.ListByType(plugin.TypeHook)
	assert.Len(t, hooks, 0)
}

func TestAll(t *testing.T) {
	r := registry.New()
	p1 := newMock("p1", plugin.TypeProvider)
	p2 := newMock("p2", plugin.TypeSerializer)

	require.NoError(t, r.Register(p1))
	require.NoError(t, r.Register(p2))

	all := r.All()
	assert.Len(t, all, 2)
}

func TestStartAllCallsStartOnAll(t *testing.T) {
	r := registry.New()
	p1 := newMock("p1", plugin.TypeProvider)
	p2 := newMock("p2", plugin.TypeSerializer)

	require.NoError(t, r.Register(p1))
	require.NoError(t, r.Register(p2))

	err := r.StartAll(context.Background())
	require.NoError(t, err)
	assert.True(t, p1.startCalled)
	assert.True(t, p2.startCalled)
}

func TestStartAllStopsOnFirstError(t *testing.T) {
	r := registry.New()
	boom := errors.New("boom")
	p1 := &mockPlugin{name: "p1", ptype: plugin.TypeProvider, startErr: boom}
	p2 := newMock("p2", plugin.TypeProvider)

	// Register p1 first so it starts first.
	require.NoError(t, r.Register(p1))
	require.NoError(t, r.Register(p2))

	err := r.StartAll(context.Background())
	require.Error(t, err)
	assert.Equal(t, boom, err)
}

func TestStopAllCallsStopOnAll(t *testing.T) {
	r := registry.New()
	p1 := newMock("p1", plugin.TypeProvider)
	p2 := newMock("p2", plugin.TypeSerializer)

	require.NoError(t, r.Register(p1))
	require.NoError(t, r.Register(p2))

	err := r.StopAll(context.Background())
	require.NoError(t, err)
	assert.True(t, p1.stopCalled)
	assert.True(t, p2.stopCalled)
}

func TestStopAllContinuesOnErrorAndReturnsFirst(t *testing.T) {
	r := registry.New()
	boom := errors.New("stop-boom")
	p1 := &mockPlugin{name: "p1", ptype: plugin.TypeProvider, stopErr: boom}
	p2 := newMock("p2", plugin.TypeProvider)

	require.NoError(t, r.Register(p1))
	require.NoError(t, r.Register(p2))

	err := r.StopAll(context.Background())
	require.Error(t, err)
	assert.Equal(t, boom, err)
	// p2 must still be stopped even though p1 errored.
	assert.True(t, p2.stopCalled)
}

func TestDeregister(t *testing.T) {
	r := registry.New()
	p := newMock("alpha", plugin.TypeProvider)

	require.NoError(t, r.Register(p))
	require.NoError(t, r.Deregister("alpha"))

	_, err := r.Get("alpha")
	require.Error(t, err)
}

func TestDeregisterNonExistentReturnsError(t *testing.T) {
	r := registry.New()
	err := r.Deregister("ghost")
	require.Error(t, err)
}
