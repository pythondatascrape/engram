package plugin

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeConstants(t *testing.T) {
	tests := []struct {
		typ  Type
		want string
	}{
		{TypeProvider, "provider"},
		{TypeSerializer, "serializer"},
		{TypeCodebook, "codebook"},
		{TypeHook, "hook"},
		{TypeObservability, "observability"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, string(tt.typ), "Type constant %q", tt.want)
	}
}

// stubPlugin verifies the Plugin interface is implementable.
type stubPlugin struct {
	name    string
	typ     Type
	builtIn bool
	startErr error
	stopErr  error
	healthErr error
}

func (s *stubPlugin) Name() string                        { return s.name }
func (s *stubPlugin) Type() Type                          { return s.typ }
func (s *stubPlugin) BuiltIn() bool                       { return s.builtIn }
func (s *stubPlugin) Start(_ context.Context) error       { return s.startErr }
func (s *stubPlugin) Stop(_ context.Context) error        { return s.stopErr }
func (s *stubPlugin) Health(_ context.Context) error      { return s.healthErr }

var _ Plugin = (*stubPlugin)(nil)

func TestPluginInterface(t *testing.T) {
	p := &stubPlugin{
		name:    "test-plugin",
		typ:     TypeProvider,
		builtIn: true,
	}
	assert.Equal(t, "test-plugin", p.Name())
	assert.Equal(t, TypeProvider, p.Type())
	assert.True(t, p.BuiltIn())
	assert.NoError(t, p.Start(context.Background()))
	assert.NoError(t, p.Stop(context.Background()))
	assert.NoError(t, p.Health(context.Background()))
}

func TestPluginInterface_Errors(t *testing.T) {
	errStart := errors.New("start failed")
	errStop := errors.New("stop failed")
	errHealth := errors.New("unhealthy")

	p := &stubPlugin{
		name:      "broken",
		typ:       TypeHook,
		builtIn:   false,
		startErr:  errStart,
		stopErr:   errStop,
		healthErr: errHealth,
	}
	assert.Equal(t, errStart, p.Start(context.Background()))
	assert.Equal(t, errStop, p.Stop(context.Background()))
	assert.Equal(t, errHealth, p.Health(context.Background()))
}
