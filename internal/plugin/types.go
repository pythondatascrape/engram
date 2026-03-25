package plugin

import "context"

// Type identifies the category of a plugin.
type Type string

const (
	TypeProvider      Type = "provider"
	TypeSerializer    Type = "serializer"
	TypeCodebook      Type = "codebook"
	TypeHook          Type = "hook"
	TypeObservability Type = "observability"
)

// Plugin is the interface that all Engram plugins must implement.
type Plugin interface {
	// Name returns the unique identifier for this plugin.
	Name() string
	// Type returns the category of this plugin.
	Type() Type
	// BuiltIn reports whether this plugin runs in-process.
	BuiltIn() bool
	// Start initializes and starts the plugin.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the plugin.
	Stop(ctx context.Context) error
	// Health returns nil if the plugin is healthy.
	Health(ctx context.Context) error
}
