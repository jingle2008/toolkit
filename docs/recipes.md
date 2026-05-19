# Recipes

End-to-end flows that compose toolkit's primitives. Each recipe is real and copy-pasteable — set the placeholder values once and the commands work as-is.

> **Prerequisites.** A working `toolkit` install (`brew install jingle2008/toolkit/toolkit`) and a populated config:
> ```bash
> toolkit init                    # scaffolds ~/.config/toolkit/config.yaml
> $EDITOR ~/.config/toolkit/config.yaml
> toolkit doctor                  # confirms repo_path, kubeconfig, etc. are wired
> ```

---

## 1. Wire `toolkit mcp` into Claude Desktop / Claude Code

Expose every category as a tool an AI agent can call directly. No shell-out, no scraping `--help`.

### Add the MCP server

**Claude Desktop** — `~/Library/Application Support/Claude/claude_desktop_config.json`:

```jsonc
{
  "mcpServers": {
    "toolkit": {
      "command": "toolkit",
      "args": ["mcp"]
    }
  }
}
```

**Claude Code** — `~/.claude.json` (or your project's `.claude/settings.json`):

```jsonc
{
  "mcpServers": {
    "toolkit": {
      "command": "toolkit",
      "args": ["mcp"]
    }
  }
}
```

**Codex CLI** — `~/.codex/config.toml`. Same pattern as Claude Code, but TOML, and the key is `mcp_servers` (snake_case) rather than `mcpServers`:

```toml
[mcp_servers.toolkit]
command = "toolkit"
args = ["mcp"]
```

If you need environment overrides (e.g., a different `TOOLKIT_ENV_REALM` for an agent session), add an `env` table inline:

```toml
[mcp_servers.toolkit]
command = "toolkit"
args = ["mcp"]
env = { TOOLKIT_ENV_REALM = "oc1", TOOLKIT_ENV_REGION = "us-phoenix-1" }
```

(Claude Desktop / Claude Code support the same idea via `"env": { ... }` inside the server block.)

Restart the client; you should see `toolkit` in the MCP servers list. The agent can now call `list_tenants`, `list_gpu_pools`, `list_dacs`, etc.

### First prompts to try

> Show me every GPU pool with more than 8 nodes and group by capacity type.

> Which tenants are missing from the production realm but exist in dev?

> Audit limit overrides for tenant `acme-corp` and tell me which ones differ from the regional default.

The agent will fan out to the right tools and combine the results. Filter narrowing happens via the `filter` argument (fuzzy substring) the agent passes on each call.

### Multi-environment in one server

A single `toolkit mcp` server boots in one env (`env_type`/`env_region`/`env_realm` from your config) but every **read** tool accepts per-call env overrides. So a single server can answer cross-env questions:

> Compare the base-model lineup in `oc1`/`us-phoenix-1` versus `oc1`/`us-ashburn-1`.

For **mutation** tools, env overrides are off by default — the operator's startup env is the maximum blast radius. Opt in (carefully) with `--mutation_env_override_allowed` at server start if your agent needs multi-realm authority.

---

## 2. GPU node maintenance window

The TUI exposes cordon/drain/reboot via keyboard shortcuts; the CLI exposes them as scriptable subcommands so you can wrap them in change-management runbooks.

### Preview, then run

```bash
NODE=gpu-node-42

# 1. Preview each step before touching anything.
toolkit cordon  $NODE --dry-run
toolkit drain   $NODE --dry-run
toolkit reboot  $NODE --dry-run

# 2. Execute. -y skips the interactive prompt for runbook automation.
toolkit cordon  $NODE -y
toolkit drain   $NODE -y
toolkit reboot  $NODE -y

# 3. Wait for the node to come back. `get gpunode` reflects live status —
#    isReady is the bool we want; the human-readable status string (from
#    GetStatus()) is computed in the renderer and not in the JSON envelope.
until toolkit get gpunode -f "$NODE" -o json | jq -e '.[] | .isReady == true' >/dev/null; do
  sleep 30
done

# 4. Re-enable scheduling.
toolkit uncordon $NODE -y
```

### Inspect the audit trail

Every mutation writes a structured line to the log (configured via `--log_file` / `log_file:` — defaults to `toolkit.log`). Set `log_format: json` to make it `jq`-friendly:

```bash
jq 'select(.msg=="mutation") | {ts, action, target, phase, dry_run, error}' toolkit.log
```

Typical output (pretty-printed):

```jsonc
{ "ts": "...", "action": "cordon",  "target": "gpu-node-42", "phase": "begin", "dry_run": false }
{ "ts": "...", "action": "cordon",  "target": "gpu-node-42", "phase": "done",  "dry_run": false }
{ "ts": "...", "action": "drain",   "target": "gpu-node-42", "phase": "begin", "dry_run": false }
{ "ts": "...", "action": "drain",   "target": "gpu-node-42", "phase": "done",  "dry_run": false }
```

`phase` is `begin` / `done` / `failed`; `dry_run: true` lines have no
`phase` and are written by `--dry-run` previews. `error` is set only on
`phase: failed`.

### Or do the same flow via MCP

If you've wired MCP per Recipe 1, ask the agent:

> Drain gpu-node-42, reboot it, and re-enable scheduling once it's back. Don't do anything until I say "confirm".

The agent will call `cordon_node`, `drain_node`, etc. — but each call requires `confirm: true`. Without it, the call refuses, logs the refusal, and returns a notification. You can preview the plan before authorizing the destructive set.

---

## 3. Audit tenants → CSV → spreadsheet

`toolkit get` plus `csv` output produces RFC-4180-compliant CSV (commas, quotes, newlines inside fields are properly quoted via `encoding/csv`).

### Straight dump

```bash
toolkit get tenant -o csv > tenants.csv
open tenants.csv                 # macOS: opens in Numbers / Excel
```

### Cross-join: tenants with their DAC counts

`get dac` is grouped by tenant. Combine with `get tenant` via `jq` to produce a single sheet:

```bash
toolkit get tenant -o json > /tmp/tenants.json
toolkit get dac    -o json > /tmp/dacs.json

jq -nr '
  [
    "TENANT,IS_INTERNAL,DAC_COUNT,DAC_NAMES",
    (
      input as $tenants
      | input as $dacs
      | $tenants[]
      | . as $t
      | ($dacs | map(select(.tenant == $t.name)) | length) as $count
      | ($dacs | map(select(.tenant == $t.name).name) | join(";")) as $names
      | [$t.name, ($t.is_internal // false), $count, $names]
      | @csv
    )
  ]
  | join("\n")
' /tmp/tenants.json /tmp/dacs.json > tenant_dac_audit.csv
```

Or, if you just want "tenants with no DACs":

```bash
toolkit get tenant -o json | jq -r '.[] | select(.is_internal | not) | .name' > /tmp/external_tenants.txt
toolkit get dac    -o json | jq -r '.[].tenant' | sort -u > /tmp/tenants_with_dacs.txt
comm -23 <(sort /tmp/external_tenants.txt) /tmp/tenants_with_dacs.txt
```

### Compare two environments

```bash
toolkit --env_realm oc1 --env_region us-ashburn-1 get tenant -o tsv > /tmp/ash.tsv
toolkit --env_realm oc1 --env_region us-phoenix-1 get tenant -o tsv > /tmp/phx.tsv
diff <(cut -f1 /tmp/ash.tsv | sort) <(cut -f1 /tmp/phx.tsv | sort)
```

TSV (not CSV) for this: `cut -f` only understands a single character, and tabs never appear inside tenant names.

---

## 4. Daily GPU-pool digest to Slack

Wrap `toolkit get gpupool -o json` and post a [Slack blocks](https://api.slack.com/block-kit) message to a webhook on a schedule.

### The transformer

```bash
#!/usr/bin/env bash
# /usr/local/bin/toolkit-gpu-digest.sh
set -euo pipefail

WEBHOOK="${SLACK_WEBHOOK_URL:?missing SLACK_WEBHOOK_URL}"

payload=$(
  toolkit get gpupool -o json \
    | jq -c '
      def totals:
        (map(.size) | add) as $declared
        | (map(.actualSize // .size) | add) as $live
        | (length) as $pools
        | { declared: $declared, live: $live, pools: $pools };

      . as $pools
      | totals as $t
      | {
          blocks: [
            { type: "header",
              text: { type: "plain_text", text: "GPU pools — daily digest" } },
            { type: "section",
              text: {
                type: "mrkdwn",
                text: ("*\($t.pools)* pools — declared *\($t.declared)* nodes, live *\($t.live)*")
              } },
            { type: "divider" },
            { type: "section",
              text: {
                type: "mrkdwn",
                text: (
                  $pools
                  | map("• `\(.name)` — \(.shape) — \(.size) nodes (\(.capacityType // \"?\"))")
                  | join("\n")
                )
              } }
          ]
        }
    '
)

curl -sS -X POST -H "Content-type: application/json" --data "$payload" "$WEBHOOK"
```

### Schedule it

**Linux / cron** — `crontab -e`:

```cron
0 9 * * 1-5  SLACK_WEBHOOK_URL='https://hooks.slack.com/services/...' /usr/local/bin/toolkit-gpu-digest.sh
```

**macOS / launchd** — `~/Library/LaunchAgents/com.toolkit.gpu-digest.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>            <string>com.toolkit.gpu-digest</string>
  <key>ProgramArguments</key> <array>
    <string>/usr/local/bin/toolkit-gpu-digest.sh</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>SLACK_WEBHOOK_URL</key>
    <string>https://hooks.slack.com/services/...</string>
  </dict>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Hour</key>   <integer>9</integer>
    <key>Minute</key> <integer>0</integer>
  </dict>
</dict>
</plist>
```

Load it: `launchctl load ~/Library/LaunchAgents/com.toolkit.gpu-digest.plist`.

### What you'll see

The Slack message lands once a day with one section per pool. If you want partial-load awareness (e.g., one Terraform source can't be parsed), pipe through the existing toolkit stderr — the `get gpupool` command prints `warning: ...` lines for those and exits zero, so a wrapper script can branch on whether stderr was non-empty and post a follow-up alert.

---

## Patterns common to all recipes

- **`-o json` for tooling, `-o csv` / `-o tsv` for spreadsheets, `-o table` for humans.** Match the consumer.
- **Always preview mutations with `--dry-run` first.** Especially in scripts — a typo'd node name shouldn't drain prod.
- **`toolkit doctor` is your CI precondition.** Wrap any automation that ends in a mutation with `if ! toolkit doctor; then exit 1; fi` so a bad config fails fast.
- **The audit log is the truth.** When debugging "did that mutation actually fire?", check the structured log, not the CLI's stdout — stdout is for humans, the log carries the metadata (env, target, dry-run flag, status, error).
