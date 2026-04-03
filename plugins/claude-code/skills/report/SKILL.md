---
name: report
description: "engram: Generate a token savings report showing compression and redundancy statistics for the current session"
---

# Engram Report

Generate a token savings report for the current session.

## Steps

1. Call `mcp__engram__get_stats` with no arguments to retrieve the current session statistics.

2. Present the stats summary to the user, then ask:
   - **Report name** — suggest the default: `Session Report — {today's date}` (e.g. "Session Report — 2026-03-27")
   - **Description** (optional) — a brief note about this session (press Enter to skip)
   - **Save to savings log?** — whether to append a row to `.engram/savings-log.csv` (default: yes)

3. Check if `.engram/config.yaml` exists. If it does, read it and extract any `pricing` fields (`pricing.model`, `pricing.input_per_1k`). Pass those as arguments when calling the report tool.

4. Call `mcp__engram__generate_report` with:
   - `name`: the report name the user provided
   - `description`: the description the user provided (omit if blank)
   - `savingsLogPath`: if user said yes to saving, pass `.engram/savings-log.csv`; omit if they said no
   - `pricing`: if config has pricing info, pass as `{ "model": "...", "inputPer1k": ... }` (nested object)

5. Display the full markdown report returned by the tool.

6. Ask: "Would you like to save this report as a markdown file?" If yes, ask for a filename (suggest `engram-report-{date}.md`) and write the report to that path in the current working directory.
