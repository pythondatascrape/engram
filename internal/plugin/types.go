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
	Name() string
	Type() Type
	BuiltIn() bool
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) error
}
