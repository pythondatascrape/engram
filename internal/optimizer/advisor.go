package optimizer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RecType classifies a recommendation.
type RecType string

const (
	RecTypeUncompressedContent RecType = "uncompressed_content"
	RecTypeHighHistoryUsage    RecType = "high_history_usage"
	RecTypeOptimal             RecType = "optimal"
)

// Recommendation is an optimization suggestion from the advisor.
type Recommendation struct {
	Type        RecType
	Description string
	Impact      Priority
}

// SessionStats captures per-session metrics for the advisor.
type SessionStats struct {
	Turns               int
	IdentityTokensSaved int
	TotalTokensSent     int
}

// AdvisorState is the persisted advisor state.
type AdvisorState struct {
	TotalSessions      int       `json:"total_sessions"`
	TotalIdentitySaved int       `json:"total_identity_saved"`
	TotalContextSaved  int       `json:"total_context_saved"`
	TotalTokensSent    int       `json:"total_tokens_sent"`
	TotalTurns         int       `json:"total_turns"`
	LastUpdated        time.Time `json:"last_updated"`
}

// Advisor tracks actual token usage and generates optimization recommendations.
type Advisor struct {
	State    AdvisorState
	stateDir string
}

// NewAdvisor creates or loads an advisor from the given state directory.
func NewAdvisor(stateDir string) (*Advisor, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("create advisor state dir: %w", err)
	}

	adv := &Advisor{stateDir: stateDir}

	path := filepath.Join(stateDir, "advisor.json")
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &adv.State); err != nil {
			return nil, fmt.Errorf("parse advisor state: %w", err)
		}
	}

	return adv, nil
}

// RecordSession adds session metrics to cumulative state.
func (a *Advisor) RecordSession(stats SessionStats) {
	a.State.TotalSessions++
	a.State.TotalIdentitySaved += stats.IdentityTokensSaved
	a.State.TotalTokensSent += stats.TotalTokensSent
	a.State.TotalTurns += stats.Turns
	a.State.LastUpdated = time.Now()
}

// Recommendations generates optimization suggestions based on accumulated data.
func (a *Advisor) Recommendations() []Recommendation {
	if a.State.TotalSessions == 0 {
		return []Recommendation{{
			Type:        RecTypeOptimal,
			Description: "No session data yet. Run some sessions to get recommendations.",
			Impact:      PriorityLow,
		}}
	}

	var recs []Recommendation

	totalSaved := a.State.TotalIdentitySaved + a.State.TotalContextSaved
	totalProcessed := a.State.TotalTokensSent + totalSaved
	if totalProcessed > 0 {
		savingsRate := float64(totalSaved) / float64(totalProcessed)
		if savingsRate < 0.10 {
			recs = append(recs, Recommendation{
				Type:        RecTypeUncompressedContent,
				Description: fmt.Sprintf("Only %.0f%% savings detected. Consider adding more identity content to CLAUDE.md or enabling history compression.", savingsRate*100),
				Impact:      PriorityHigh,
			})
		}
	}

	if a.State.TotalTurns > 10 && a.State.TotalContextSaved == 0 {
		recs = append(recs, Recommendation{
			Type:        RecTypeHighHistoryUsage,
			Description: "History compression not active. Enable context schema to compress conversation history.",
			Impact:      PriorityHigh,
		})
	}

	if len(recs) == 0 {
		avgSavedPerTurn := totalSaved / a.State.TotalTurns
		recs = append(recs, Recommendation{
			Type:        RecTypeOptimal,
			Description: fmt.Sprintf("Running well: ~%d tokens saved per turn across %d sessions.", avgSavedPerTurn, a.State.TotalSessions),
			Impact:      PriorityLow,
		})
	}

	return recs
}

// Save persists advisor state to disk.
func (a *Advisor) Save() error {
	data, err := json.MarshalIndent(a.State, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal advisor state: %w", err)
	}
	path := filepath.Join(a.stateDir, "advisor.json")
	return os.WriteFile(path, data, 0644)
}
