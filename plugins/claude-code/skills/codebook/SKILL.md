---
name: codebook
description: "engram: Manage the identity codebook — show the active codebook, diff against CLAUDE.md, initialize new codebooks, or validate existing ones"
---

# Engram Codebook Manager

Manage the active identity codebook for the current project.

## Menu

Present the following options to the user and ask which they want:

1. **show** — Display the active codebook dimensions
2. **diff** — Compare active codebook against current CLAUDE.md
3. **init** — Create a `.engram-codebook.yaml` override file
4. **validate** — Check codebook coverage against CLAUDE.md

---

## Subcommand: show

1. Call `mcp__engram__get_stats` with no arguments.
2. Extract the `codebook` field from the result and display each dimension with its key, description, and any example values.
3. If no codebook is active (the field is empty or absent), inform the user: "No active codebook found." Then offer to derive one — if they agree, proceed with the **init** flow.

---

## Subcommand: diff

1. Read the current `CLAUDE.md` file from the project root.
2. Call `mcp__engram__derive_codebook` with the full CLAUDE.md content as the `content` argument.
3. Call `mcp__engram__get_stats` to get the currently active codebook dimensions.
4. Compare the two sets of dimensions:
   - **Added** — dimensions in the derived set that are not in the active codebook
   - **Removed** — dimensions in the active codebook that are not in the derived set
   - **Unchanged** — dimensions present in both
5. Display the diff in a clear, structured format. If there are no differences, report: "Active codebook is in sync with CLAUDE.md."

---

## Subcommand: init

Walk the user through creating a `.engram-codebook.yaml` override file.

1. Read the current `CLAUDE.md` file from the project root.
2. Call `mcp__engram__derive_codebook` with the CLAUDE.md content to get auto-derived dimensions. Display them.
3. For each dimension, ask the user if they want to:
   - **Keep as-is** (default)
   - **Override** — let them provide a custom key, description, or example values
   - **Suppress** — exclude this dimension from the codebook
4. Ask if there are any additional custom dimensions to add.
5. Build the YAML structure and write it to `.engram-codebook.yaml` in the project root.
6. Confirm: "Codebook saved to `.engram-codebook.yaml`. Engram will use this on the next session start."

---

## Subcommand: validate

Check that the active codebook covers all meaningful sections of CLAUDE.md.

1. Read the current `CLAUDE.md` file from the project root.
2. Call `mcp__engram__derive_codebook` with the CLAUDE.md content.
3. Call `mcp__engram__get_stats` to get the active codebook.
4. For each top-level section heading in CLAUDE.md, check whether a corresponding dimension exists in the active codebook.
5. Call `mcp__engram__check_redundancy` with a sample of the CLAUDE.md content to detect any patterns the codebook is not capturing.
6. Report:
   - Sections with matching codebook coverage (pass)
   - Sections with no coverage (warn)
   - Any undetected redundancy patterns found
7. If issues are found, suggest running the **init** subcommand to rebuild the codebook.
