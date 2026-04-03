---
name: config
description: Manage engram plugin settings — redundancy thresholds, notification preferences, pricing, and auto-report configuration
---

# Engram Config Manager

Manage engram plugin settings stored in `.engram/config.yaml`.

## Default Configuration

```yaml
redundancy_threshold: 10000
notify_on_redundancy: true
auto_report_on_exit: false
pricing:
  model: "claude-sonnet-4-20250514"
  input_per_1k: 0.003
```

---

## Steps

### 1. Show current config

Check if `.engram/config.yaml` exists in the project root.

- If it exists, read the file and display all current values in a clear table or list.
- If it does not exist, inform the user: "No config file found. Defaults will be used." Then display the default values listed above.

### 2. Ask what to do

Present these options:

1. **Edit settings** — interactively change one or more settings
2. **Reset to defaults** — overwrite with the default config
3. **Cancel** — do nothing

---

## Edit Settings

For each setting, show the current value and ask if they want to change it. Accept Enter to keep the current value.

- **`redundancy_threshold`** *(integer, default: 10000)*
  Number of redundant tokens detected before a notification is surfaced. Higher values reduce noise; lower values increase sensitivity.

- **`notify_on_redundancy`** *(true/false, default: true)*
  Whether to surface a notification when the redundancy threshold is crossed during a session.

- **`auto_report_on_exit`** *(true/false, default: false)*
  Whether to automatically generate a savings report when the session ends.

- **`pricing.model`** *(string, default: "claude-sonnet-4-20250514")*
  The model name used for cost estimation in reports. Must match an Anthropic model identifier.

- **`pricing.input_per_1k`** *(number, default: 0.003)*
  Cost per 1,000 input tokens in USD, used to calculate estimated savings.

After collecting all changes, display a summary of what will be written and ask for confirmation before saving.

---

## Reset to Defaults

Show the default config and ask: "Reset `.engram/config.yaml` to defaults? (yes/no)"

If confirmed, write the default config to `.engram/config.yaml`, creating `.engram/` if it does not exist.

---

## Save

When writing the config file:

1. Ensure the `.engram/` directory exists (create it if not).
2. Write the updated YAML to `.engram/config.yaml`.
3. Confirm: "Config saved to `.engram/config.yaml`."
