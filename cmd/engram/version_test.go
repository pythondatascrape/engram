package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCommand(t *testing.T) {
	cmd := newVersionCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "engram version")
}
