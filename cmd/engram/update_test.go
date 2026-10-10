package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateCmd_HasSourceFlag(t *testing.T) {
	assert.NotNil(t, newUpdateCmd().Flags().Lookup("source"))
}
