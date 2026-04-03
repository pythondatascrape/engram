package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeCommand_Help(t *testing.T) {
	cmd := newServeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "daemon")
	assert.Contains(t, buf.String(), "--socket")
}

func TestServeCommand_DefaultSocketPath(t *testing.T) {
	cmd := newServeCmd()
	f := cmd.Flag("socket")
	assert.NotNil(t, f)
}
