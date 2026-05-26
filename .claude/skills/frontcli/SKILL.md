---
name: frontcli
description: Interact with Front customer support platform via the frontcli CLI. Use when managing conversations, messages, contacts, tags, drafts, comments, templates, inboxes, teammates, or channels in Front. Triggered by mentions of Front, frontcli, support tickets, customer conversations, or helpdesk operations.
license: MIT
homepage: https://github.com/dedene/frontapp-cli
metadata:
  author: dedene
  version: "1.1.0"
  openclaw:
    primaryEnv: FRONT_ACCOUNT
    requires:
      env:
        - FRONT_ACCOUNT
        - FRONT_KEYRING_BACKEND
      bins:
        - frontcli
    install:
      - kind: brew
        tap: dedene/tap
        formula: frontcli
        bins: [frontcli]
      - kind: go
        package: github.com/dedene/frontapp-cli/cmd/frontcli
        bins: [frontcli]
---

# frontcli -- Front CLI

CLI for [Front](https://frontapp.com) customer support. Manages conversations, messages, contacts, tags, drafts, comments, templates, and more.

## Quick Start

Verify auth before any operation:

```bash
frontcli auth status
frontcli whoami --json
```

If not authenticated, the user must run `frontcli auth login` interactively (requires browser for OAuth). Do not attempt auth setup on behalf of the user.

## Core Rules

1. **Always use `--json`** when parsing output. Human-readable format is for display only.
2. **Use correct ID prefixes** -- see ID Reference below. Wrong prefix produces a clear error.
3. **Read before write** -- fetch current state before modifying (archive, assign, tag, reply).
4. **Pipe with jq** -- extract IDs/fields: `frontcli conv list --json | jq -r '._results[].id'`
5. **Multi-account** -- use `--account user@email.com` if the user has multiple Front accounts.
6. **Do not inspect keychain contents**. Use `frontcli auth status`, `frontcli auth list`, and `frontcli whoami`; if auth fails, ask the user to run `frontcli auth login`.

## ID Reference

| Prefix | Resource     | Example      |
|--------|-------------|--------------|
| `cnv_` | conversation | `cnv_abc123` |
| `msg_` | message      | `msg_abc123` |
| `cmt_` | comment      | `cmt_abc123` |
| `tea_` | teammate     | `tea_abc123` |
| `tag_` | tag          | `tag_abc123` |
| `inb_` | inbox        | `inb_abc123` |
| `cha_` | channel      | `cha_abc123` |
| `ctc_` | contact      | `ctc_abc123` |
| `msg_` | draft        | `msg_abc123` |
| `rsp_` | template     | `rsp_abc123` |
| `att_` | attachment   | `att_abc123` |
| `hdl_` | handle       | `hdl_abc123` |

## Output Formats

| Flag       | Format | Use case |
|------------|--------|----------|
| (default)  | Table  | User-facing display |
| `--json`   | JSON   | Agent parsing, scripting |
| `--plain`  | TSV    | Pipe to awk/cut |

JSON list responses wrap results in `._results[]`. Single-object responses return the object directly.

## Workflows

### Read a Conversation Thread

```bash
# Full message bodies + comments (timeline view)
frontcli conv get cnv_xxx --full --json

# Metadata only
frontcli conv get cnv_xxx --json

# Messages only
frontcli conv messages cnv_xxx --json

# Comments only
frontcli conv comments cnv_xxx --json
```

Use `--full` when you need message content; omit for metadata only.

### Search / List Conversations

```bash
# List recent open conversations
frontcli conv list --status open --limit 10 --json

# List by inbox or tag
frontcli conv list --inbox inb_xxx --json
frontcli conv list --tag tag_xxx --json

# Free-text search
frontcli conv search "customer issue" --json

# Filtered search
frontcli conv search --from client@co.com --status open --json
frontcli conv search --tag tag_xxx --assignee me --json
frontcli conv search --after "2025-01-01" --before "2025-02-01" --json
```

Search flags: `--from`, `--to`, `--recipient`, `--inbox`, `--tag` (repeatable), `--status`, `--assignee`, `--unassigned`, `--before`, `--after`, `--limit`.

### Reply to a Conversation

```bash
frontcli msg reply cnv_xxx --body "Thanks for reaching out."
frontcli msg reply cnv_xxx --body-file ./reply.txt
```

### Send a New Message

Requires a channel ID. Find channels first:

```bash
frontcli channels list --json
frontcli msg send --channel cha_xxx --to user@example.com --subject "Hello" --body "Message body"
```

### Add Internal Comment

Comments are internal notes visible only to teammates:

```bash
frontcli comments create cnv_xxx --body "Internal note: customer is VIP"
```

### Manage Tags

```bash
# List available tags
frontcli tags list --json

# Find tag ID by name
frontcli tags list --json | jq -r '._results[] | select(.name == "Urgent") | .id'

# Tag / untag a conversation
frontcli conv tag cnv_xxx tag_xxx
frontcli conv untag cnv_xxx tag_xxx
```

### Assign / Unassign

```bash
# Find teammate IDs
frontcli teammates list --json

# Assign / unassign
frontcli conv assign cnv_xxx --to tea_xxx
frontcli conv unassign cnv_xxx
```

### Manage Conversation Status

```bash
frontcli conv archive cnv_xxx
frontcli conv open cnv_xxx
frontcli conv trash cnv_xxx

# Snooze (timestamp or duration)
frontcli conv snooze cnv_xxx --until "2025-01-15T09:00:00Z"
frontcli conv snooze cnv_xxx --duration 2h
frontcli conv unsnooze cnv_xxx
```

### Look Up Contacts

```bash
frontcli contacts search "john" --json
frontcli contacts get ctc_xxx --json
frontcli contacts handles ctc_xxx --json
frontcli contacts convos ctc_xxx --json
```

### Templates

```bash
frontcli templates list --json
frontcli templates use rsp_xxx

# Use template body in a reply
BODY=$(frontcli templates use rsp_xxx)
frontcli msg reply cnv_xxx --body "$BODY"
```

### Drafts

Drafts are Front draft messages, so their IDs usually start with `msg_`.

```bash
frontcli drafts create cnv_xxx --channel cha_xxx --author tea_xxx --inbox inb_xxx --body "Draft reply"
frontcli drafts list cnv_xxx --json
frontcli drafts get msg_xxx --json
frontcli drafts update msg_xxx --body "Updated" --draft-version draft-ver-123
frontcli drafts delete msg_xxx --draft-version draft-ver-123
```

#### Reply Drafts

Front requires `channel_id` in the request body for `POST /conversations/{conversation_id}/drafts`.
That means `frontcli drafts create <cnv>` must include `--channel`, even though the conversation
already exists. If Front returns `body.channel_id: missing`, the CLI is too old or the command is
missing `--channel`.

Resolve the channel before creating a reply draft:

```bash
# Read the last message recipients and identify the address the customer mailed.
frontcli conv messages cnv_xxx --json

# Map that address to a Front channel and inbox.
frontcli channels list --json
frontcli inboxes channels inb_xxx --json
```

For a clean shared reply draft:

```bash
frontcli drafts list cnv_xxx --json
frontcli drafts create cnv_xxx \
  --channel cha_xxx \
  --author tea_xxx \
  --inbox inb_xxx \
  --mode shared \
  --body-file ./reply.html \
  --no-default-signature \
  --json
frontcli drafts list cnv_xxx --json
```

Use a small HTML file with paragraph tags for multi-paragraph email bodies. Plain text bodies may
render as one dense block in Front:

```html
<p>Dag Tobias,</p>
<p>Merci om dit door te sturen.</p>
<p>...</p>
```

Do not use `frontcli drafts create --channel ... --to ...` when the user asked for a reply draft;
that creates a new draft conversation instead of attaching the draft to the existing thread.

If using a local dev build and auth works in the user shell but fails for the agent, rerun the same
`frontcli` command outside the sandbox. macOS Keychain access may be scoped to the executable and
the sandbox context.

### Custom Fields

```bash
frontcli conv update cnv_xxx --field "Priority=High" --field "Category=Support"
```

## Bulk Operations

```bash
# Archive all conversations with a tag (xargs)
frontcli conv list --tag tag_xxx --json | jq -r '._results[].id' | xargs frontcli conv archive

# Archive from stdin
frontcli conv list --tag tag_xxx --json | jq -r '._results[].id' | frontcli conv archive --ids-from -

# Tag all open unassigned conversations
frontcli conv search --status open --unassigned --json | jq -r '._results[].id' | \
  while read id; do frontcli conv tag "$id" tag_xxx; done
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `FRONT_ACCOUNT` | Default account (avoids `--account`) |
| `FRONT_JSON` | Set `1` for JSON output by default |
| `FRONT_PLAIN` | Set `1` for TSV output by default |

## Command Reference

| Command | Subcommands |
|---------|-------------|
| `conv` | `list`, `get`, `search`, `messages`, `comments`, `archive`, `open`, `trash`, `assign`, `unassign`, `snooze`, `unsnooze`, `followers`, `follow`, `unfollow`, `tag`, `untag`, `update` |
| `msg` | `get`, `send`, `reply`, `attachments`, `attachment download` |
| `drafts` | `create`, `list`, `get`, `update`, `delete` |
| `tags` | `list`, `get`, `create`, `update`, `delete`, `children`, `convos` |
| `contacts` | `list`, `search`, `get`, `handles`, `handle add/delete`, `notes`, `note add`, `convos`, `create`, `update`, `delete`, `merge` |
| `inboxes` | `list`, `get`, `convos`, `channels` |
| `teammates` | `list`, `get`, `convos` |
| `channels` | `list`, `get` |
| `comments` | `list`, `get`, `create` |
| `templates` | `list`, `get`, `use` |
| `whoami` | (show authenticated user) |
| `auth` | `setup`, `login`, `logout`, `status`, `list` |

## Guidelines

- Never expose or log OAuth tokens, client secrets, or keyring contents.
- Confirm destructive operations (`trash`, `delete`, `archive`) with the user first.
- `contacts merge` is irreversible -- always confirm before executing.
- Rate limits are handled automatically with exponential backoff.
- When an error mentions wrong ID prefix, check the ID Reference table.


## Installation

```bash
brew install dedene/tap/frontcli
```
