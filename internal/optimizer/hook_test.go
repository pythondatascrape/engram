package optimizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultStateDir(t *testing.T) {
	dir := DefaultStateDir("/Users/test/my-project")
	assert.Contains(t, dir, ".engram/projects/")
	assert.NotContains(t, dir, "my-project")
}

func TestAdvisor_RoundTrip(t *testing.T) {
	stateDir := t.TempDir()
	adv, err := NewAdvisor(stateDir)
	require.NoError(t, err)

	adv.RecordSession(SessionStats{
		Turns:               5,
		IdentityTokensSaved: 2000,

		TotalTokensSent:     3000,
	})
	require.NoError(t, adv.Save())

	adv2, err := NewAdvisor(stateDir)
	require.NoError(t, err)
	assert.Equal(t, 1, adv2.State.TotalSessions)
	assert.Equal(t, 2000, adv2.State.TotalIdentitySaved)
}
