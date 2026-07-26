# contour

**Centralized context provider for AI agents.**

`contour` allows you to maintain a centralized library of rules, skills, and knowledge and reuse them across projects, sessions, and agents.

`contour` provides two interfaces:

- a **CLI**, for humans and short-lived agents
- an **MCP server**, for long-running agents like Claude Code

Write a rule once; every project and every session can reach it.

---

## Contents

- [contour](#contour)
  - [Contents](#contents)
  - [Install](#install)
    - [Homebrew](#homebrew)
    - [Using Go](#using-go)
  - [Quick start](#quick-start)
  - [Where the store lives](#where-the-store-lives)
    - [Moving the store somewhere else](#moving-the-store-somewhere-else)
  - [Store layout](#store-layout)
    - [Folder names are tags](#folder-names-are-tags)
    - [Ordering](#ordering)
    - [Skills are folders](#skills-are-folders)
    - [Item IDs](#item-ids)
  - [File headers](#file-headers)
    - [Rules, knowledge and skills](#rules-knowledge-and-skills)
    - [Bootstrap profiles](#bootstrap-profiles)
  - [Bootstrap profiles](#bootstrap-profiles-1)
  - [Project-specific context](#project-specific-context)
    - [Overlay folders](#overlay-folders)
    - [Local wins on conflict](#local-wins-on-conflict)
    - [Single files (CLAUDE.md, AGENTS.md)](#single-files-claudemd-agentsmd)
    - [Gradual migration](#gradual-migration)
  - [CLI reference](#cli-reference)
  - [Using contour with Claude Code](#using-contour-with-claude-code)
    - [Wiring it up](#wiring-it-up)
    - [Choosing the profile](#choosing-the-profile)
    - [What the agent gets](#what-the-agent-gets)
  - [Improving the store with usage stats](#improving-the-store-with-usage-stats)
    - [Privacy](#privacy)

---

## Install

### Homebrew

```bash
brew install vieolo/tap/contour
```

### Using Go

```bash
# requires Go 1.26 or newer
go install github.com/vieolo/contour@latest
```


Verify either install with:

```bash
contour version
```

---

## Quick start

```bash
contour list
```

There is no setup step. On first use contour creates the store at `~/contour`, fills it with sample files, prints the layout so you know how it is organised, and then runs your command.

For example:

```bash
contour bootstrap python
```


## Where the store lives

The store is one central directory. It is created once and shared by every project. It is not a per-project scaffolding.

| | |
|---|---|
| **Default location** | `~/contour` |
| **Config file** | `~/.contour/config.yaml` |
| **Created** | automatically, the first time a command needs it |

The store is a plain folder of markdown files that you are meant to open and edit, so it lives somewhere visible rather than in a dotfile directory.

To see where contour is currently reading from:

```bash
contour home
```

### Moving the store somewhere else

One command — your content comes with it:

```bash
contour set-home ~/my/new/path/contour
```

That moves the store to the new directory and records the location in
`~/.contour/config.yaml`. There is nothing to move by hand.

Or skip typing a path altogether — `here` puts the store in the directory you
are standing in:

```bash
contour set-home here
```

Run from `~/Documents`, that gives you `~/Documents/contour`. It creates a
folder for the store rather than filling the working directory itself, so your
other files stay where they are. (To target a directory genuinely named
`here`, write `./here`.)

What it does depends on what is there:

| Situation | Result |
|---|---|
| You have a store, destination is free | The store is **moved**, content and all |
| No store yet | A new one is **created** at the destination |
| Destination already has content | It is **adopted as-is** — nothing moved or overwritten, and contour tells you where your previous store was left |

The move is atomic where it can be, and falls back to copy-then-delete across
filesystems — so the original is only removed once the copy has succeeded.

contour reads no environment variables at all. The config file is the only way to
change where the store lives.

The generated config will hold:

```yaml
# store_path: the directory holding your rules, skills and knowledge.
#   Leave it empty to use the default location (~/contour).
#   Prefer `contour set-home <path>` over editing this by hand: it
#   moves the store's content to the new location, or creates one there
#   if you do not have a store yet. Editing this line only repoints
#   contour, leaving your content where it is.
store_path: ""

# usage_logging: record which items agents fetch and what they search for,
#   under ~/.contour/usage, so `contour stats` can show what earns its
#   place and what agents looked for but could not find. It never leaves this
#   machine. Search queries are recorded, so set it to false to turn logging
#   off entirely.
usage_logging: true
```


## Store layout

Four top-level folders.

```
~/contour/
├── README.md
├── bootstrap/                            entry points (see below)
│   └── python.md
├── rules/                                loaded EAGERLY
│   ├── general/
│   │   └── 010-communication.md
│   └── python/
│       └── 010-errors.md
├── skills/                               fetched ON DEMAND
│   ├── general/
│   │   └── write-commit-message/
│   │       └── SKILL.md
│   └── python/
│       └── release/
│           └── SKILL.md
└── knowledge/                            fetched ON DEMAND
    └── general/
        └── stack.md
```

| Folder | Holds | Loading |
|---|---|---|
| `rules/` | Imperative "how to behave" guidance | **Eager**, loaded up front, always in effect |
| `skills/` | Procedural "how to do X" | **Lazy**, listed by description, fetched when relevant |
| `knowledge/` | Reference facts | **Lazy**, listed by description, fetched when relevant |
| `bootstrap/` | Named entry points | Provides different entry points based on projects. You can have different bootstraps based on the stack of the project |

Rules are small and always apply, so they are delivered immediately. Skills and
knowledge can grow without limit, so agents only see a one-line description of
each until they actually need one. That is what keeps a session's starting
context small while leaving the whole store reachable.

### Folder names are tags

Every folder under `rules/`, `skills/` and `knowledge/` becomes an implicit tag
on the files beneath it. There is no metadata file to maintain:

```
rules/python/010-errors.md      →  tagged "python"
rules/python/web/020-http.md    →  tagged "python" and "web"
```

Nest as deeply as you like. Every segment is a tag. Add extra tags in the files' header when the folder alone is not enough.

### Ordering

Items in a folder are ordered by filename. When order matters, prefix with a
**three-digit** number:

```
010-communication.md
020-tone.md
030-escalation.md
```

The gaps leave room to insert entries later without renaming anything. Three
digits rather than two because past 99 files `100-` would sort before `20-`.

Prefixes are optional. Files without one simply sort alphabetically.

### Skills are folders

A skill is any directory containing a `SKILL.md`. The directory name is the
skill's name, and the folders above it are its tags:

```
skills/python/release/SKILL.md   →  skill "release", tagged "python"
```

You can keep supporting files beside `SKILL.md` in a folder, such as scripts, templates, examples. contour serves `SKILL.md` and ignores the rest, so nothing extra reaches the agent's context.

### Item IDs

Every item has an ID: its path from the store root, without the extension. IDs
are what you pass to `contour get`, and what agents pass to the `get` tool.

```
rules/python/010-errors
skills/python/release
knowledge/general/stack
```


## File headers

Files may start with a YAML frontmatter block delimited by `---`. It is optional for content files and required for bootstrap profiles.

### Rules, knowledge and skills

```markdown
---
description: How we handle errors in Python
tags: [errors, style]
---

- Raise a specific exception type; never raise or catch a bare Exception.
- Chain with "raise WrappedError(...) from err" so the original traceback survives.
```

| Field | Required | Purpose |
|---|---|---|
| `description` | recommended | One line saying what this is. Agents read it to decide whether to fetch the file, so it is worth writing well — for skills and knowledge it is often the *only* thing an agent sees until it fetches the body. |
| `tags` | optional | Extra tags, added to the implicit ones from the folder path. |

A file with no frontmatter is still valid, it simply has no
description and only its folder-derived tags.

### Bootstrap profiles

Profiles live in `bootstrap/` and use a different set of fields. Here the
frontmatter *is* the configuration, and the body is an optional preamble sent
ahead of the rules.

```markdown
---
description: Python backend projects
rules: [general, python]
skills: [general, python]
knowledge: [general, python]
---

Entry point for Python backend projects.
```

| Field | Purpose |
|---|---|
| `description` | Shown when listing profiles. |
| `rules` | Tags whose rules are loaded **eagerly**, in the order listed. |
| `skills` | Tags whose skills are offered for **on-demand** fetching. |
| `knowledge` | Tags whose knowledge is offered for **on-demand** fetching. |

---

## Bootstrap profiles

A profile is a named entry point: it decides which slice of your store a project gets. The filename is the profile name. `bootstrap/python.md` is the `python` profile.

Because selection is by tag, profiles compose naturally. A `python-web` profile is just a longer tag list:

```yaml
rules: [general, python, web]
```

They also stay current on their own: add `rules/python/040-typing.md` and every profile selecting `python` picks it up. There is no index to update.

The order of the files are important in how they are loaded. Tags are emitted in the order you list them, so `[general, python]` puts your general rules before your Python ones. An item matching two selected tags appears once, in the position of the first tag that matched. If a profile names a tag that matches nothing, contour warns on stderr rather
than silently sending less than you expected, which catches typos.

---

## Project-specific context

Not everything belongs in the central store. Some rules and knowledge are true only for one project, and a project should carry its own context rather than scatter it into a shared store. So contour layers **project-local context on top of the central store**, and lets you migrate between the two at your own pace.

### Overlay folders

In a project, contour looks for an overlay folder — any of `.contour/`, `.agents/` or `.claude/` — with the same `rules/`, `skills/` and `knowledge/` layout as the store:

```
my-project/
├── .contour/
│   ├── rules/
│   │   └── 010-logging.md          ← always eager for this project
│   └── skills/
│       └── deploy/
│           └── SKILL.md
└── ...
```

Three folder names, because teams already keep agent context in different places; contour reads whichever you use (and unions them if you have more than one). It only looks at the `rules/`, `skills/` and `knowledge/` inside — anything else in the folder is left alone, so it sits happily beside another tool's config.

Overlay items are **always active** for the project: local rules load eagerly, local skills and knowledge join the on-demand menu, no tags needed. Being local is scope enough.

### Local wins on conflict

When a local item and a central one clash, the local one is authoritative. contour states this to the agent rather than trying to detect clashes itself, so it covers semantic conflicts too:

```
# Rules                                     (from your central store)
...
# Project rules (local — authoritative on conflict)
...
```

An identical path in both places (say `rules/python/errors.md` locally and in the store) resolves to the local one. Everything is labelled by source in `contour list`, so you always know where a rule came from.

### Single files (CLAUDE.md, AGENTS.md)

If you already keep a `CLAUDE.md` or `AGENTS.md`, you don't have to restructure it. List it in the project config and contour loads it eagerly as a project rule:

```yaml
# .contour.yaml   (at the project root)
bootstrap: python              # the central profile for this project
eager_files:
  - AGENTS.md
  - docs/architecture.md
```

`contour mcp-init` writes this file for you and detects a root `AGENTS.md`/`CLAUDE.md` automatically. Only listed files load — contour never slurps a file it merely finds.

The config lives at the project **root** (`.contour.yaml`), not inside an overlay folder, so moving your local rules from `.contour/` to `.agents/` later never silently drops it — the same reason the machine-wide config sits outside the store.

### Gradual migration

This is the on-ramp, too. Start with everything local — a `CLAUDE.md` and a few overlay rules — and move the reusable pieces into the central store one at a time, as you notice them repeating across projects. During a move the local copy wins, so nothing changes until you delete it and the store version takes over. You can adopt contour with no central store at all and grow into it.

---

## CLI reference

| Command | Purpose |
|---|---|
| `contour list [kind]` | List items with their IDs, descriptions and tags. Optionally limit to `rules`, `skills` or `knowledge`. |
| `contour get <id>` | Print one item's content. Body only, so it pipes cleanly. |
| `contour search <query> [kind]` | Search IDs, descriptions, tags and content, case-insensitively. |
| `contour bootstrap [name]` | Print a profile's session payload. Without a name, list the available profiles. |
| `contour init` | Create the store explicitly. Optional, contour creates it on first use. Safe to re-run; never overwrites existing files. |
| `contour home` | Show where the store lives, how that was decided, and which config file records it. |
| `contour set-home <path\|here>` | Move the store to another directory, content and all, recording it in the config file. `here` uses a folder in the working directory. Creates a new store if you have none. |
| `contour mcp` | Run the MCP server over stdio. |
| `contour mcp-init` | Set the project up: register contour in `.mcp.json` (absolute path, bare `mcp`) and write `.contour.yaml` with the profile, detecting a root `AGENTS.md`/`CLAUDE.md`. |
| `contour stats` | Show how agents have used the store: gaps (searched, found nothing), never-fetched items, and most-fetched. Local only; `--project`, `--days`, `--clear`. |
| `contour version` | Print the version. |

Some examples:

```bash
# What is in my store?
contour list

# Just the skills
contour list skills

# Read one item
contour get rules/python/010-errors

# Find everything mentioning migrations
contour search migration

# The full session payload for a Python project
contour bootstrap python

# Where is my store?
contour home

# Move it somewhere I can browse (content comes along)
contour set-home ~/my/new/path/contour

# ...or into the folder I'm already in
contour set-home here

# What have agents searched for but never found?
contour stats
```

`get` and `bootstrap` write **only** their payload to stdout, notices and
warnings go to stderr, so they redirect and pipe safely:

```bash
contour bootstrap python > CONTEXT.md
```


## Using contour with Claude Code

The MCP server delivers your rules automatically at the start of every session,
and lets the agent pull skills and knowledge as it needs them.

### Wiring it up

From your project root:

```bash
contour mcp-init --bootstrap python
```

That does two things: registers contour in `.mcp.json` (or adds it to the one
you already have, leaving other servers untouched), and writes a
`.contour.yaml` recording the profile. Commit both and everyone on the
project gets the same context.

The `.mcp.json` entry is a bare launch line — the profile lives in the project
config, not the arguments:

```json
{
  "mcpServers": {
    "contour": {
      "command": "/opt/homebrew/bin/contour",
      "args": ["mcp"]
    }
  }
}
```

**Why the absolute path and not just `contour`?** Your agent launches its MCP
servers without a login shell, so it never sees the `PATH` from your
`.zshrc`. A Homebrew install lives in `/opt/homebrew/bin`, which is not in
macOS's default `PATH` at all — a bare `contour` would simply not be found.
`mcp-init` records the path contour was actually invoked from, so it is right
by construction.

It records the symlink (`/opt/homebrew/bin/contour`), never the versioned target
underneath it (`/opt/homebrew/Cellar/contour/0.2.0/...`), so the entry keeps
working after `brew upgrade`.

The profile and any eager files come from `.contour.yaml` (see
[Project-specific context](#project-specific-context)), so nothing project-specific
is baked into the launch arguments. And nothing else needs configuring even if
your store is somewhere custom: contour reads its location from
`~/.contour/config.yaml`, so wherever `contour set-home` put it is where the
server reads from.

<details>
<summary>Writing the files by hand</summary>

`mcp-init` is a convenience, not a requirement. If you write `.mcp.json`
yourself, use the absolute path to the binary — `which contour` will tell you —
for the reason above, and a bare `mcp` argument. `claude mcp add` works too:

```bash
claude mcp add contour -- $(which contour) mcp
```

Then set the profile in `.contour.yaml` with `bootstrap: <name>`.

</details>

### Choosing the profile

The profile lives in the project config, so it travels with the project:

```yaml
# .contour.yaml   (at the project root)
bootstrap: python
```

`contour mcp-init --bootstrap <name>` writes that for you. The `--bootstrap` flag on `contour mcp` overrides it if you ever need to. It belongs with the project rather than in the machine-wide config because the right profile depends on the project, not the machine.

Started without one, the server still serves the whole store through its tools, and its instructions explain how to select an entry point — a missing profile degrades gracefully rather than failing. Any project overlay rules still load, so a project can rely on local context alone.

### What the agent gets

**At session start**, in the server's instructions:

- your profile's preamble
- every rule the profile selects, in full
- a menu of the available skills and knowledge, one line each

**On demand**, through three tools:

| Tool | Does |
|---|---|
| `list` | List available items, optionally filtered by kind |
| `get` | Read one item's full content by ID |
| `search` | Find items by ID, description, tag or content |

Edits to your store take effect on the next tool call, no need to restart the server after changing a file. The eagerly-loaded rules are fixed when the session starts, so changes to those apply from the next session.

---

## Improving the store with usage stats

Centralising your rules is half the value; keeping them *good* is the other half. contour records how agents actually use the store during MCP sessions, so you can see what earns its place and what to fix.

```bash
contour stats
```

It shows three things, most actionable first:

- **Gaps** — searches that returned nothing. This is the strongest signal: it tells you exactly what to write next. If agents keep searching for `docker` and finding nothing, add a docker rule.
- **Never fetched** — items no agent has pulled. Review candidates, not a delete list: an item can be unused because it is weak, or because the situation it covers has not come up yet.
- **Most fetched** — what pulls its weight, and might deserve promoting from a lazy skill to an eager rule.

Example:

```
contour usage — 42 sessions across 3 projects  (all projects, all time)

gaps — agents searched, found nothing
  "docker"        12×   last 2d ago
  "graphql"        4×   last 9d ago

never fetched — review or prune
  skills/aws/lambda-deploy
  knowledge/legacy/soap-api

most fetched
  rules/python/010-errors        88×   last 1h ago
  skills/python/release          14×   last 3d ago
```

Scope it to one project with `--project <substring>`, to a recent window with `--days <n>`, or wipe the logs with `--clear`.

> **Note**
> This measures *engagement*, not effectiveness — what agents reached for, not whether it helped. A never-fetched item is a prompt to look, not a verdict to delete.

### Privacy

Usage stays on your machine; contour never sends it anywhere. It is on by default and disclosed in the config file, where you can turn it off:

```yaml
usage_logging: false
```

Search queries are recorded (they are what makes the gaps signal work), so if that matters to you, set the flag to `false` or run `contour stats --clear` to wipe the logs. Only MCP (agent) usage is recorded — never the commands you run yourself in the terminal.
