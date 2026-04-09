package smc

// PreferenceSignal captures when a user corrects or overrides a prior response.
// Stored as metadata on the MatrixRow where the correction occurs.
type PreferenceSignal struct {
	Type     string `json:"type"`      // "correction", "rejection", "alternative_chosen"
	FromTurn int    `json:"from_turn"` // which prior turn this corrects (-1 if new)
	Category string `json:"category"`  // which category was corrected (empty if general)
	Detail   string `json:"detail"`    // what changed
}
