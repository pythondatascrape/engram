package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMCPCommand(t *testing.T) {
	cmd := newMCPCmd()
	assert.Equal(t, "mcp", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestMCPCommandRegistered(t *testing.T) {
	root := newRootCmd()
	root.AddCommand(newMCPCmd())
	found := false
	for _, sub := range root.Commands() {
		if sub.Use == "mcp" {
			found = true
			break
		}
	}
	assert.True(t, found, "mcp subcommand should be registered on root")
}
