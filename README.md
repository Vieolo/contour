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
  - [CLI reference](#cli-reference)
  - [Using contour with Claude Code](#using-contour-with-claude-code)
    - [Wiring it up](#wiring-it-up)
    - [Choosing the profile](#choosing-the-profile)
    - [What the agent gets](#what-the-agent-gets)

---

## Install

### Homebrew

```bash
brew install vieolo/tap/contour
```

### Using Go

```bash
# requires Go 1.26
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

There is no setup step. On first use contour creates the store at `~/.contour`,
fills it with sample files, prints the layout so you know how it is organised,
and then runs your command.

For example:

```bash
contour bootstrap python
```


## Where the store lives

The store is one central directory. It is created once and shared by every project. It is not a per-project scaffolding.

| | |
|---|---|
| **Default location** | `~/.contour` |
| **Override** | `CONTOUR_HOME` environment variable |
| **Created** | automatically, the first time a command needs it |

### Moving the store somewhere else

Move the directory, then point `CONTOUR_HOME` at its new home:

```bash
mv ~/.contour /path/to/contour
```

Then add the variable to your shell profile:

```bash
export CONTOUR_HOME="/path/to/contour"
```


> **Note**
> When `CONTOUR_HOME` is set but points at a directory that does not exist, contour reports an error instead of creating a store there. A typo in the variable should not silently produce an empty store in the wrong place. Run `contour init` if you genuinely want a new store at that path.


## Store layout

Four top-level folders.

```
~/.contour/
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

## CLI reference

| Command | Purpose |
|---|---|
| `contour list [kind]` | List items with their IDs, descriptions and tags. Optionally limit to `rules`, `skills` or `knowledge`. |
| `contour get <id>` | Print one item's content. Body only, so it pipes cleanly. |
| `contour search <query> [kind]` | Search IDs, descriptions, tags and content, case-insensitively. |
| `contour bootstrap [name]` | Print a profile's session payload. Without a name, list the available profiles. |
| `contour init` | Create the store explicitly. Optional, contour creates it on first use. Safe to re-run; never overwrites existing files. |
| `contour mcp` | Run the MCP server over stdio. |
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

Create a `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "contour": {
      "command": "contour",
      "args": ["mcp", "--bootstrap", "python"]
    }
  }
}
```

Commit that file and everyone on the project gets the same context.

If your store is not at the default location, pass it through:

```json
{
  "mcpServers": {
    "contour": {
      "command": "contour",
      "args": ["mcp", "--bootstrap", "python"],
      "env": {
        "CONTOUR_HOME": "/Users/you/Dropbox/contour"
      }
    }
  }
}
```

You can also add it from the terminal instead of writing the file by hand:

```bash
claude mcp add contour -- contour mcp --bootstrap python
```

### Choosing the profile

Pick the profile per project, either with `--bootstrap <name>` as above or with
the `CONTOUR_BOOTSTRAP` environment variable. The flag wins when both are set.

Started without either, the server still serves the whole store through its
tools, and its instructions explain how to select an entry point — a missing
profile degrades gracefully rather than failing.

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
