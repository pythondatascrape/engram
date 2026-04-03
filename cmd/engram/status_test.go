package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusCommand_Help(t *testing.T) {
	cmd := newStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "status")
	assert.Contains(t, buf.String(), "daemon")
}
