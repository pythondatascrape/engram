package optimizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvisor_RecordAndRecommend(t *testing.T) {
	dir := t.TempDir()
	adv, err := NewAdvisor(dir)
	require.NoError(t, err)

	adv.RecordSession(SessionStats{
		Turns:               10,
		IdentityTokensSaved: 2500,

		TotalTokensSent:     5000,
	})

	recs := adv.Recommendations()
	assert.NotEmpty(t, recs)
}

func TestAdvisor_PersistsState(t *testing.T) {
	dir := t.TempDir()
	adv, err := NewAdvisor(dir)
	require.NoError(t, err)

	adv.RecordSession(SessionStats{
		Turns:               5,
		IdentityTokensSaved: 1000,

		TotalTokensSent:     2000,
	})

	require.NoError(t, adv.Save())

	adv2, err := NewAdvisor(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, adv2.State.TotalSessions)
	assert.Equal(t, 1000, adv2.State.TotalIdentitySaved)
}

func TestAdvisor_DetectsUncompressedOpportunity(t *testing.T) {
	dir := t.TempDir()
	adv, err := NewAdvisor(dir)
	require.NoError(t, err)

	adv.RecordSession(SessionStats{
		Turns:               20,
		IdentityTokensSaved: 100,

		TotalTokensSent:     50000,
	})

	recs := adv.Recommendations()
	found := false
	for _, r := range recs {
		if r.Type == RecTypeUncompressedContent {
			found = true
		}
	}
	assert.True(t, found, "should detect uncompressed content opportunity")
}

func TestAdvisor_NoSessions(t *testing.T) {
	dir := t.TempDir()
	adv, err := NewAdvisor(dir)
	require.NoError(t, err)

	recs := adv.Recommendations()
	require.Len(t, recs, 1)
	assert.Equal(t, RecTypeOptimal, recs[0].Type)
	assert.Contains(t, recs[0].Description, "No session data")
}
