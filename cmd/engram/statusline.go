package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pythondatascrape/engram/internal/optimizer"
	"github.com/pythondatascrape/engram/internal/updater"
	"github.com/spf13/cobra"
)

func newStatuslineCmd() *cobra.Command {
	var sessionID string
	var ctxOrig, ctxComp int

	cmd := &cobra.Command{
		Use:   "statusline [directory]",
		Short: "Print a compact token-savings chart for editor status lines",
		Long: `Scans a project directory for identity files and prints a three-row
bar chart (original / compressed / saved) suitable for embedding in
editor status lines such as Claude Code's statusLine setting.

When the Engram daemon is running, all three rows reflect actual runtime
token accounting from live sessions. Otherwise, static estimates are shown.

When invoked as a Claude Code statusLine command, session_id is read from
stdin automatically so each terminal session shows its own stats.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			// If stdin is piped (Claude Code statusLine command), try to extract
			// session_id from the JSON payload so we can show per-session stats.
			if sessionID == "" {
				if info, err := os.Stdin.Stat(); err == nil && (info.Mode()&os.ModeCharDevice) == 0 {
					if raw, err := io.ReadAll(os.Stdin); err == nil {
						var payload struct {
							SessionID string `json:"session_id"`
						}
						if json.Unmarshal(raw, &payload) == nil {
							sessionID = payload.SessionID
						}
					}
				}
			}

			// If we have a session_id, check for a per-session stats file first.
			// This ensures multiple concurrent sessions never cross-contaminate.
			if sessionID != "" {
				home, _ := os.UserHomeDir()
				sessionsDir := filepath.Join(home, ".engram", "sessions")
				sessFile := filepath.Join(sessionsDir, sessionID+".json")

				// Load proxy ctx stats from the companion .ctx.json file (written by the
				// proxy to avoid cross-process races with the stop hook).
				if ctxOrig == 0 || ctxComp == 0 {
					ctxFile := filepath.Join(sessionsDir, sessionID+".ctx.json")
					if raw, err := os.ReadFile(ctxFile); err == nil {
						var c struct {
							CtxOrig int `json:"ctx_orig"`
							CtxComp int `json:"ctx_comp"`
						}
						if json.Unmarshal(raw, &c) == nil {
							if ctxOrig == 0 {
								ctxOrig = c.CtxOrig
							}
							if ctxComp == 0 {
								ctxComp = c.CtxComp
							}
						}
					}
				}

				if raw, err := os.ReadFile(sessFile); err == nil {
					var s struct {
						TotalOrig  int `json:"total_orig"`
						TotalComp  int `json:"total_comp"`
						TotalSaved int `json:"total_saved"`
					}
					if json.Unmarshal(raw, &s) == nil && s.TotalOrig > 0 {
						data := optimizer.StatuslineData{
							Orig:  s.TotalOrig,
							Comp:  s.TotalComp,
							Saved: s.TotalSaved,
							Live:  true,
						}
						if ctxOrig > 0 && ctxComp > 0 {
							optimizer.FormatStatuslineSideBySide(os.Stdout, data, optimizer.ContextData{Orig: ctxOrig, Comp: ctxComp})
						} else {
							optimizer.FormatStatusline(os.Stdout, data)
						}
						if v := updater.ReadAvailableUpdate(); v != "" {
							fmt.Fprintf(os.Stdout, "update %s available — run: engram update\n", v)
						}
						return nil
					}
				}
			}

			profile, err := optimizer.ScanProject(absDir)
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			cfg := optimizer.DefaultEstimatorConfig()
			report := optimizer.EstimateSavings(profile, cfg)

			// Start with static estimates.
			data := optimizer.StatuslineEstimate(report)

			// Override with live stats from ~/.engram/stats.json if available.
			// Note: this is the global file shared across sessions; it is only used
			// as a fallback when no per-session file is found above.
			home, _ := os.UserHomeDir()
			statsPath := filepath.Join(home, ".engram", "stats.json")
			if raw, err := os.ReadFile(statsPath); err == nil {
				var s struct {
					TotalOriginalTokens   int `json:"totalOriginalTokens"`
					TotalCompressedTokens int `json:"totalCompressedTokens"`
					TotalSaved            int `json:"totalSaved"`
				}
				if json.Unmarshal(raw, &s) == nil && s.TotalOriginalTokens > 0 {
					data = optimizer.StatuslineData{
						Orig:  s.TotalOriginalTokens,
						Comp:  s.TotalCompressedTokens,
						Saved: s.TotalSaved,
						Live:  true,
					}
				}
			}

			if ctxOrig > 0 && ctxComp > 0 {
				optimizer.FormatStatuslineSideBySide(os.Stdout, data, optimizer.ContextData{Orig: ctxOrig, Comp: ctxComp})
			} else {
				optimizer.FormatStatusline(os.Stdout, data)
			}

			if v := updater.ReadAvailableUpdate(); v != "" {
				fmt.Fprintf(os.Stdout, "update %s available — run: engram update\n", v)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "Scope stats to a specific session (auto-detected from stdin when used as a statusLine command)")
	cmd.Flags().IntVar(&ctxOrig, "ctx-orig", 0, "Context tokens without Engram (enables side-by-side view)")
	cmd.Flags().IntVar(&ctxComp, "ctx-comp", 0, "Context tokens with Engram")

	return cmd
}
