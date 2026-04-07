---
name: engram-codebook
description: "Build an engram codebook for Claude prompts — compresses identity (CLAUDE.md) and prompt vocabulary into key=value pairs to reduce per-turn token cost"
---

# Engram Codebook Wizard

Build a two-part codebook that reduces Claude context overhead:
- **Identity codebook** — compresses who Claude is (CLAUDE.md → key=value, sent once per session)
- **Prompt codebook** — compresses what you ask Claude (base slots + project vocabulary)

## Step 1: Choose scope

Ask the user:

> "Where should this codebook apply?
> 1. **Session** — in-memory only, savings this session, nothing written to disk
> 2. **Project** — saved to `./.engram-codebook.yaml` in the current project root
> 3. **Global** — saved to `~/.engram-codebook.yaml`, applies to all sessions"

Store the answer as `[scope]`. Proceed to Step 2.

---

## Step 2: Read CLAUDE.md

Based on `[scope]`:
- Session or Project → read `CLAUDE.md` from the current project root
- Global → read `~/.claude/CLAUDE.md`

If the file does not exist, inform the user: "No CLAUDE.md found at `[path]`. You can continue and I'll build the prompt codebook only, or create a CLAUDE.md first."

Proceed to Step 3.

---

## Step 3: Derive identity dimensions

Call `mcp__plugin_engram_engram__derive_codebook` with:
- `content`: the full CLAUDE.md text read in Step 2

Display the derived dimensions to the user in a table:

| Key | Type | Derived value |
|-----|------|--------------|
| (from result) | | |

Then supplement with these Claude-behavioral dimensions that `derive_codebook` typically misses from prose-style CLAUDE.md files. The values below are **defaults based on common CLAUDE.md patterns** — review each and override if your file uses different conventions:

| Key | Type | Suggested value | Common pattern (verify against your CLAUDE.md) |
|-----|------|----------------|--------------|
| `plan_mode` | enum: always,non_trivial,never | `non_trivial` | "Enter plan mode for ANY non-trivial task" |
| `subagent_use` | enum: liberal,minimal,never | `liberal` | "Use subagents liberally" |
| `self_improve` | bool | `true` | "update tasks/lessons.md after corrections" |
| `verify_before_done` | bool | `true` | "Never mark a task complete without proving it works" |
| `elegance_check` | enum: balanced,always,skip | `balanced` | "Demand Elegance (Balanced)" |
| `autonomous_bugfix` | bool | `true` | "When given a bug report: just fix it" |
| `response_style` | enum: concise,verbose | `concise` | "Make every change as simple as possible" |
| `task_tracking` | enum: todo_md,inline,none | `todo_md` | "Write plan to tasks/todo.md" |
| `code_quality` | enum: senior,standard | `senior` | "Senior developer standards" |
| `impact_scope` | enum: minimal,broad | `minimal` | "Impact minimal code" |

After review, ask: "Any additional custom identity dimensions to add?" If yes, collect key, type, and value for each.

Proceed to Step 4.

---

## Step 4: Build prompt codebook

Tell the user: "Now building the prompt codebook — base slots that compress any task prompt."

Add these base template slots (no user review needed — these are universal):

```yaml
prompt_task:
  type: enum
  values: [implement, fix, refactor, explain, review, plan]
  description: "The primary action being requested"

prompt_scope:
  type: text
  description: "What file/component/system is affected"

prompt_style:
  type: enum
  values: [concise, thorough, tdd, elegant]
  description: "How the work should be approached"

prompt_constraint:
  type: text
  description: "What NOT to do or hard limits"

prompt_target:
  type: text
  description: "Success criteria for this prompt"
```

Then scan the project for domain vocabulary. Use Grep to find the most frequent meaningful terms in `CLAUDE.md` and file names under the project root. Encode the top terms (skip common English words):

```yaml
vocab_skill:
  type: text
  description: "A Claude Code skill file (SKILL.md)"

vocab_agent:
  type: text
  description: "A Claude Code subagent dispatched via Agent tool"

vocab_subagent:
  type: text
  description: "Subagent handling a focused sub-task"

vocab_plan:
  type: text
  description: "Implementation plan doc in docs/superpowers/plans/"

vocab_codebook:
  type: text
  description: "Engram codebook YAML defining compression dimensions"

vocab_hook:
  type: text
  description: "Claude Code hook script triggered by tool events"

vocab_session:
  type: text
  description: "The current Claude Code conversation session"
```

Show the user the vocabulary list. Ask: "Any terms to add or remove?"

Proceed to Step 5.

---

## Step 5: Check for existing codebook and confirm overwrite

Determine the target path:
- Session → no file write, skip to Step 6
- Project → `./.engram-codebook.yaml`
- Global → `~/.engram-codebook.yaml`

For Project or Global scope: check if the file exists (use Read tool and handle the "file not found" case).

If a file exists, display a warning:

> "A codebook already exists at `[path]`. Overwriting will replace all existing dimensions. Continue?"

If the user declines, stop. Offer: "You can cancel and manually merge new dimensions into the existing file, or run the engram:codebook skill's **diff** subcommand to see what would change."

If the user confirms (or no file exists), proceed to Step 6.

---

## Step 6: Assemble and write the codebook YAML

Assemble the final YAML by merging:
1. Auto-derived dimensions from Step 3
2. Reviewed behavioral dimensions from Step 3
3. Prompt base slots from Step 4
4. Project vocabulary from Step 4

Use this format (all dimensions live at the top-level `dimensions` key — engram's schema):

```yaml
# Engram codebook — generated by engram-codebook skill
# Identity dimensions compress CLAUDE.md behavioral instructions.
# Prompt dimensions (prefix: prompt_, vocab_) compress task prompts.
name: [project-name or "global"]
version: 1
dimensions:
  plan_mode:
    type: enum
    value: non_trivial
  subagent_use:
    type: enum
    value: liberal
  self_improve:
    type: bool
    value: "true"
  verify_before_done:
    type: bool
    value: "true"
  elegance_check:
    type: enum
    value: balanced
  autonomous_bugfix:
    type: bool
    value: "true"
  response_style:
    type: enum
    value: concise
  task_tracking:
    type: enum
    value: todo_md
  code_quality:
    type: enum
    value: senior
  impact_scope:
    type: enum
    value: minimal
  prompt_task:
    type: enum
    value: null
  prompt_scope:
    type: text
    value: null
  prompt_style:
    type: enum
    value: null
  prompt_constraint:
    type: text
    value: null
  prompt_target:
    type: text
    value: null
  vocab_skill:
    type: text
    value: "skill_file"
  vocab_agent:
    type: text
    value: "subagent"
  vocab_subagent:
    type: text
    value: "subagent"
  vocab_plan:
    type: text
    value: "impl_plan"
  vocab_codebook:
    type: text
    value: "engram_cb"
  vocab_hook:
    type: text
    value: "cc_hook"
  vocab_session:
    type: text
    value: "cc_session"
```

For **Session scope**: do not write a file. Instead call `mcp__plugin_engram_engram__compress_identity` passing the assembled dimensions as a structured object (not a raw YAML string). Pass each dimension key-value pair from the assembled codebook as the `identity` parameter content. Report what was compressed.

For **Project or Global scope**: write the assembled YAML to the target path using the Write tool.

---

## Step 7: Show sample compression (testing phase)

> **Note: This step is for the testing phase only. Remove once encoding is validated.**

Show the user a before/after example:

**Before (plain prompt):**
> "Create a new skill that uses subagents to parallelize the plan execution and verify it works before marking done"

**After (codebook-encoded):**
> `prompt_task=implement prompt_scope=vocab_skill prompt_style=concise prompt_target=parallel_subagent_plan verify_before_done=true`

Token count before: ~22 tokens. After: ~12 tokens (illustrative — actual savings vary by prompt). **~40–50% prompt compression typical.**

---

## Step 8: Confirm and show savings estimate

Report success:

> "Codebook saved to `[path]` (or: held in session memory).
>
> **Estimated savings:**
> - Identity: ~400 tokens → ~20 tokens per session start (~95% reduction)
> - Per-turn identity re-send: eliminated after turn 1
> - Prompt compression: ~30–50% per encoded prompt
>
> On a 50-turn session this saves approximately **19,000+ tokens** (theoretical maximum) — equivalent to ~15 pages of usable context added back to your window.
>
> To check redundancy in future responses, call `mcp__plugin_engram_engram__check_redundancy` with the response content."
