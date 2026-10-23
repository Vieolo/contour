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
    - [Combining profiles](#combining-profiles)
  - [Project-specific context](#project-specific-context)
    - [Overlay folders](#overlay-folders)
    - [Local wins on conflict](#local-wins-on-conflict)
    - [Project profiles](#project-profiles)
    - [Gradual migration](#gradual-migration)
  - [Presets](#presets)
    - [Single files (CLAUDE.md, AGENTS.md)](#single-files-claudemd-agentsmd)
    - [Switching presets](#switching-presets)
    - [Presets in a monorepo](#presets-in-a-monorepo)
    - [Keeping your own store out](#keeping-your-own-store-out)
    - [Where the config lives](#where-the-config-lives)
    - [Upgrading from the old config](#upgrading-from-the-old-config)
  - [CLI reference](#cli-reference)
  - [Using contour with Claude Code](#using-contour-with-claude-code)
    - [Wiring it up](#wiring-it-up)
    - [Choosing the preset](#choosing-the-preset)
    - [What the agent gets](#what-the-agent-gets)
    - [When the rules don't fit](#when-the-rules-dont-fit)
  - [Seeing what your profiles offer](#seeing-what-your-profiles-offer)
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

### Combining profiles

A project rarely sits under exactly one heading. A Python service might also
ship an accessory CLI — real, but not what the project is *about*, so making
`cli` the entry point would be wrong, and so would a `python-cli` profile that
exists only for this one combination.

So name more than one:

```bash
contour bootstrap python cli
```

or, in a preset of the project config (see [Presets](#presets)):

```yaml
presets:
  default:
    bootstrap: [python, cli]
```

They compose exactly the way tags inside a single profile already do — the
order you list them drives the order of the payload, and an item both profiles
select is loaded once, in the position of the first that selected it. Each
profile's preamble is emitted, in the same order.

This is what keeps `bootstrap/` from growing combinatorially. Profiles stay
single-purpose (`python`, `cli`, `web`), and each project assembles the entry
point it actually needs. A tag that matches nothing is still reported against
the profile that asked for it, so a typo names the file to go and fix:

```
warning: profile "cli" requests tag "flgas", which matches no item
```

---

## Project-specific context

Not everything belongs in the central store. Some rules and knowledge are true only for one project, and a project should carry its own context rather than scatter it into a shared store. So contour layers **project-local context on top of the central store**, and lets you migrate between the two at your own pace.

### Overlay folders

In a project, contour looks for an overlay folder — any of `.contour/`, `.agents/` or `.claude/` — with the same layout as the store:

```
my-project/
├── .agents/
│   ├── bootstrap/
│   │   └── frontend.md             ← a profile of the project's own
│   ├── rules/
│   │   ├── 010-logging.md
│   │   └── frontend/
│   │       └── 010-react.md        ← tagged "frontend"
│   └── skills/
│       └── deploy/
│           └── SKILL.md
└── ...
```

Three folder names, because teams already keep agent context in different places; contour reads whichever you use (and unions them if you have more than one). It only looks at the `bootstrap/`, `rules/`, `skills/` and `knowledge/` inside — anything else in the folder is left alone, so it sits happily beside another tool's config.

By default, project items are **always active**: local rules load eagerly, local skills and knowledge join the on-demand menu, no tags needed — being local is scope enough. A preset can instead take only the project items its profiles select (`local: selected`), which is how a large project gives each of its parts just its own context; see [Presets](#presets). Folder names are tags here, just as in the central store.

To start one, run this from the project root:

```bash
contour init --local
```

It creates `.agents/` with empty `bootstrap/`, `rules/`, `skills/` and `knowledge/` folders (each holding a `.gitkeep`, since git does not track empty folders) and a README documenting the formats. Commit it, and the whole project shares it.

There are **no sample files**, and that is deliberate. A sample in the central store is inert until a profile selects it; a local item is live the moment it exists. A sample rule would load into every session in the project, ahead of your central rules. So the examples live in the README, which contour never reads: it only looks inside those four folders.

`.agents/` is the default because it is vendor-neutral. If the project already has a local store in any overlay folder, `init --local` leaves it alone rather than splitting the project's context across two places. And since other tools use `.agents/` too, it scaffolds alongside their files without overwriting any. It never touches the central store either, so a project can run on local context alone.

### Local wins on conflict

When a local item and a central one clash, the local one is authoritative. contour states this to the agent rather than trying to detect clashes itself, so it covers semantic conflicts too:

```
# Rules                                     (from your central store)
...
# Project rules (local — authoritative on conflict)
...
```

An identical path in both places (say `rules/python/errors.md` locally and in the store) resolves to the local one. Everything is labelled by source in `contour list`, so you always know where a rule came from.

### Project profiles

A profile can live in the project too, in an overlay's `bootstrap/` folder, written exactly like one in your central store:

```markdown
---
description: The React app
rules: [frontend]
skills: [frontend, testing]
---
```

Profile names are looked up in the project first, then your central store, so a project profile takes the place of a central one with the same name — the same rule local items follow. That is what makes it portable: committed with the project, `frontend` means the same thing on every machine, whatever each developer's own store holds under that name. `contour bootstrap` marks project profiles, and the central ones they override, so an override is never invisible.

Profiles select project items by tag as well as central ones, so a profile selecting `frontend` picks up `.agents/rules/frontend/`.

### Gradual migration

This is the on-ramp, too. Start with everything local — a `CLAUDE.md` and a few overlay rules — and move the reusable pieces into the central store one at a time, as you notice them repeating across projects. During a move the local copy wins, so nothing changes until you delete it and the store version takes over. You can adopt contour with no central store at all and grow into it.

---

## Presets

A project's contour config is a set of **presets**: named starting points for an agent session, each saying which profiles and which project files load eagerly.

```yaml
# .contour.yaml   (at the project root)
presets:
  default:
    bootstrap: [python, cli]
    eager_files: [AGENTS.md]
  docs:
    description: Writing and reviewing the docs site
    bootstrap: [writing]
    eager_files: [AGENTS.md, docs/STYLE.md]
```

| Field | Purpose |
|---|---|
| `description` | Shown to the agent when it lists the presets it can load. |
| `bootstrap` | Profiles whose rules load eagerly — one name or a list, composed in order. Looked up in the project first, then your central store. |
| `eager_files` | Project files loaded eagerly as rules, paths relative to the project root. |
| `local` | `all` (the default) includes every project item. `selected` includes only those the profiles select; the rest stay reachable through `list`, `search` and `get`. Files in `eager_files` are delivered either way. |
| `central` | `true` (the default) uses your central store. `false` uses only the project's local store. |

A session starts with `default` — the one reserved name. Every preset stands on its own: nothing is inherited from `default`, so each lists everything it needs. `contour mcp-init` writes a config with a `default` preset, and documents `local` and `central` in it as comments.

### Single files (CLAUDE.md, AGENTS.md)

If you already keep a `CLAUDE.md` or `AGENTS.md`, you don't have to restructure it: list it in a preset's `eager_files` and it loads eagerly as a project rule. `contour mcp-init` detects a root `AGENTS.md`/`CLAUDE.md` automatically. Only listed files load — contour never slurps a file it merely finds.

### Switching presets

- **Mid-session.** The server's instructions list the project's other presets with their descriptions. Ask the agent to "load the docs preset" and it calls the `bootstrap` tool with that preset's name. Loading *adds* to the session — what was in effect stays — and leaves out the rules the session already has from its start, naming them instead of repeating them.
- **Per developer.** `contour mcp --preset docs` starts every session with that preset instead of `default`. It belongs in a developer's own `.mcp.json`, which is specific to their machine anyway (see [Wiring it up](#wiring-it-up)).

A preset name the config does not define falls back to `default`, and the agent is told. So does a profile contour cannot find: the session starts without it rather than failing, and the agent is asked to tell you.

`contour bootstrap --preset <name>` prints a preset's payload — exactly what an agent starting with it gets.

### Presets in a monorepo

`default` is always loaded, and loading only ever adds. So in a project with several unrelated parts, **keep `default` light** — what every session needs, plus the list of presets — and give each part a preset of its own, loaded on demand:

```yaml
presets:
  default:
    bootstrap: []
    eager_files: [AGENTS.md]
    local: selected
  frontend:
    description: The React app
    bootstrap: [frontend]
    eager_files: [AGENTS.md, frontend/AGENTS.md]
    local: selected
  backend:
    description: The Go API
    bootstrap: [backend]
    eager_files: [AGENTS.md, backend/AGENTS.md]
    local: selected
```

With the project's context in one local store — `.agents/rules/frontend/`, `.agents/rules/backend/` — and a [project profile](#project-profiles) per part, `local: selected` gives each preset only its own part. A session starts with the repo-wide `AGENTS.md` and the list of presets; "load the frontend preset" adds the frontend's rules and skills, and nothing of the backend's. Nothing is out of reach, either: presets decide what is *offered*, never what can be *found*. `list`, `search` and `get` reach every part of the project, other presets' eager files included.

Loading cannot take anything away, so moving from one part to another means starting a new session.

One thing to watch under `local: selected`: a project item no profile selects is only reachable by search. To keep a repo-wide rule eager, list it in `eager_files` or give it a tag a profile selects. `contour list --profiles` shows anything nothing offers.

### Keeping your own store out

`central: false` makes a preset use only the project's local store. None of the central store reaches the session — not its rules or profiles at the start, and not through `list`, `search` or `get` either. It is for teams that want an agent's context to come from the project alone, without each developer's personal store mixed in. Such a preset can only use the project's own profiles.

Two limits worth knowing. It governs what *contour* gives an agent: a developer's `~/.claude/CLAUDE.md` and their other MCP servers still apply. And presets are yours to combine: load a preset that uses the central store into a `central: false` session, and the session's tools can reach the central store from then on. Your own terminal is never scoped — `contour list` and `search` always show your whole store, which is how you curate it.

### Where the config lives

The config lives at the project **root**, not inside an overlay folder, so moving your local rules from `.contour/` to `.agents/` later never silently drops it — the same reason the machine-wide config sits outside the store.

It can be `.contour.yaml` or, in a Go project that already keeps a [`go.yaml`](https://github.com/vieolo/godotyaml), that file's `external.contour` section — where the go.yaml spec puts third-party tool configuration — sparing the project a second config file:

```yaml
name: myproject
version: 1.2.3

external:
  gomore:
    commands:
      build: "go build ./..."
  contour:
    presets:
      default:
        bootstrap: [python, cli]
        eager_files: [AGENTS.md]
```

The fields are identical; only their location differs. `contour mcp-init` detects an existing `go.yaml` and writes there instead, leaving the rest of the file's settings — its metadata and other tools' sections — untouched.

**`.contour.yaml` always wins.** If a project has one, that is the config, and any
contour section in a host manifest is ignored. Being contour's own file makes it
the unambiguous statement of intent — and the rule stays a single comparison as
more manifests are supported (`pyproject.toml` and `package.json` are the
obvious next ones). Were a manifest to win instead, every new one would have to
be ranked against all the others.

When a host section is shadowed this way, contour names it on stderr rather than
leaving you to wonder which file is in effect.

**A bare `go.yaml` changes nothing.** contour only reads from a manifest that
actually has a contour section. A project keeps its `go.yaml` for reasons of its
own, so adding one must never be mistaken for a contour config — nor disturb the
`.contour.yaml` beside it. And a `go.yaml` contour cannot parse is the project's
file, not contour's: it degrades to a warning rather than breaking every command.

So the precedence, in full:

| Project has | contour reads |
|---|---|
| `.contour.yaml` only | `.contour.yaml` |
| `go.yaml` with `external.contour` | that section |
| both | `.contour.yaml`, and says the other is ignored |
| `go.yaml` without a contour section | nothing — no config |

`mcp-init` only writes a config when the project has none, and puts it where it will be read — into `go.yaml` if the project keeps one, otherwise `.contour.yaml` — so it never files settings somewhere permanently shadowed, and never rewrites the ones already there.

### Upgrading from the old config

Before presets, the config held `bootstrap` and `eager_files` at the top level. The first contour command run in such a project migrates it: those settings become the `default` preset, the file is rewritten in the new shape — documented like a freshly generated one — and contour says so, so you can review the change and commit it. What loads does not change.

The migration runs on whichever command reads the config first, including the MCP server an agent starts. If the file cannot be written, the session runs on the migrated settings anyway, with a warning. A config holding both shapes at once is refused rather than guessed at. In a `go.yaml`, the rewrite also drops the file's blank lines, which the YAML library cannot preserve.

---

## CLI reference

| Command | Purpose |
|---|---|
| `contour list [kind]` | List items with their IDs, descriptions and tags. Optionally limit to `rules`, `skills` or `knowledge`. `--profiles` adds which bootstrap profiles offer each item. |
| `contour get <id>` | Print one item's content. Body only, so it pipes cleanly. |
| `contour search <query> [kind]` | Search IDs, descriptions, tags and content, case-insensitively. |
| `contour bootstrap [name...]` | Print the session payload for one or more profiles, composed in the order given, or with `--preset <name>` for one of the project's presets. Without either, list the project's presets and the available profiles. |
| `contour init` | Create the store explicitly. Optional, contour creates it on first use. Only ever creates: on an existing store it changes nothing, so samples you deleted stay deleted. `--local` creates a project-local store in `./.agents` instead. |
| `contour home` | Show where the store lives, how that was decided, and which config file records it. |
| `contour set-home <path\|here>` | Move the store to another directory, content and all, recording it in the config file. `here` uses a folder in the working directory. Creates a new store if you have none. |
| `contour mcp` | Run the MCP server over stdio. `--preset <name>` starts sessions with that preset instead of `default`. |
| `contour mcp-init` | Register contour in `.mcp.json` on this machine (absolute path, bare `mcp`), and give the project a config if it has none — a `default` preset with profiles from `--bootstrap` or picked from a list with `-i`, plus a detected root `AGENTS.md`/`CLAUDE.md`. Never modifies an existing project config. |
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

# ...for a Python project that also ships a CLI
contour bootstrap python cli

# What a session starting with this project's frontend preset gets
contour bootstrap --preset frontend

# Where is my store?
contour home

# Move it somewhere I can browse (content comes along)
contour set-home ~/my/new/path/contour

# ...or into the folder I'm already in
contour set-home here

# Which profiles offer each item — and what no profile offers?
contour list --profiles

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
`.contour.yaml` whose `default` preset records the profile.

**Commit the project config; keep `.mcp.json` out of git.** The two files are
different in kind. The project config is portable — a profile name means the
same thing on every machine — so committing it gives everyone on the project
the same context. `.mcp.json` is not: it holds the absolute path to *your*
contour binary (see below), which is `/opt/homebrew/bin/contour` on one machine
and something else on the next. Add it to `.gitignore`.

Everyone else then runs, once, on their own machine:

```bash
contour mcp-init
```

That registers contour in their own `.mcp.json` and leaves the committed config
exactly as it is. **`mcp-init` never modifies an existing project config** — it
is the team's shared setup, and this command runs on every machine, so it must
not be able to rewrite it as a side effect. To change the profiles, edit the
config file, where the change shows up in review. Passing `--bootstrap` against
an existing config is accepted only if it names the profiles the config already
selects, so re-running the original setup command is harmless; anything else
is refused before a file is touched.

If a profile the project's presets use cannot be found on that developer's
machine, `mcp-init` says so — their sessions would start without its rules.

If your `.mcp.json` also carries servers the whole team shares, ignoring it
costs you those too — in that case keep contour out of the shared file and
register it per developer instead (see *Writing the files by hand* below).

Repeat the flag to combine entry points:

```bash
contour mcp-init --bootstrap python --bootstrap cli
```

Or, if you remember what you want but not what the profile is called, pick
from a list:

```bash
contour mcp-init -i
```

It lists every profile in your store with its description — space selects,
enter confirms, `/` filters by name or description. The chosen profiles compose
in the order listed (alphabetical), so if order matters, adjust `bootstrap:`
in the config afterwards. `-i` needs a terminal, and like `--bootstrap` it only
applies to a project that has no config yet: on one that does, it says so
before showing the list.

The `.mcp.json` entry is a bare launch line — the presets live in the project
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

The profiles and eager files come from the project config's presets (see
[Presets](#presets)), so nothing project-specific is baked into the launch
arguments. And nothing else needs configuring even if
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

Then give `.contour.yaml` a `default` preset naming the profiles in its
`bootstrap` (see [Presets](#presets)).

</details>

### Choosing the preset

A session starts with the project's `default` preset. To start every session
of yours somewhere else, add `--preset` to the arguments in your own
`.mcp.json`:

```json
"args": ["mcp", "--preset", "frontend"]
```

Any other preset can be loaded part-way through a session by asking the agent
(see [Switching presets](#switching-presets)). `--bootstrap <name>` on
`contour mcp` replaces the starting preset's profiles while keeping the rest of
it.

Started with no profile at all, the server still serves the whole store through
its tools, and any project rules still load, so a project can rely on local
context alone.

### What the agent gets

**At session start**, in the server's instructions:

- the project's other presets, one line each, to load when the work calls for one
- each selected profile's preamble
- every rule those profiles select, and the preset's eager files, in full
- a menu of the available skills and knowledge, one line each

**On demand**, through four tools:

| Tool | Does |
|---|---|
| `bootstrap` | Load the complete set of rules in effect — or, given a preset's name, load that preset |
| `list` | List available items, optionally filtered by kind |
| `get` | Read one item's full content by ID |
| `search` | Find items by ID, description, tag or content |

### When the rules don't fit

The MCP `instructions` field is not a delivery guarantee. The specification
calls it a hint that a client *may* add to the system prompt, and clients cap
how much of it they keep — cutting from the end. A store whose eager rules grow
past that cap would lose its last rules with nothing to show for it.

So contour sends only what it knows will fit, and says what it withheld:

```
## INCOMPLETE — `bootstrap` must be your FIRST tool call

Shown: 915 of 1948 characters, 3 of 7 rules. The other 4 rules are in force and you have not been told what they say.

Call `bootstrap` now — literally your next call, before reading a file, searching, planning or replying.
"Before the first important action" is the wrong reading: you cannot know which actions the missing rules govern.
What follows is a partial sample, not your instructions.

...the rules that fit...

— end of sample: 4 of 7 rules withheld. Call `bootstrap` before doing anything else. —
```

The details matter:

- **The notice comes first.** A capping client cuts from the end, so a directive
  placed after the rules would be the first thing discarded.
- **The sizes are stated.** An agent told only that "more rules exist" cannot
  judge whether fetching them is worth a tool call. One told it is holding 915
  of 1,948 characters knows it is missing most of its instructions.
- **The wording names a specific event, and rejects the wrong reading.** An
  earlier version said "call `bootstrap` before your first action", and agents
  reliably read that as *before the first important action* and deferred it
  indefinitely — "action" is elastic, and an excerpt that looks like a usable
  rule set makes postponing feel safe. The directive now anchors to the first
  *tool call*, which cannot be reinterpreted, and says why deferring is not a
  judgement the agent is positioned to make: it cannot know which actions the
  unseen rules govern.
- **The sample is bracketed.** The notice is read before the rules; by the end of
  them the agent has accumulated plausible guidance and may have stopped treating
  the gap as real, so the closing line restates it.
- **Rules are included whole or not at all.** A body cut mid-sentence can invert
  its own meaning — "never do X unless…" truncated to "never do X".
- **Project-local rules get first claim.** They are authoritative on conflict, so
  dropping them in favour of the central rules they exist to override would make
  a session wrong, not merely short.
- **Menus are sacrificed before rules.** The `list` tool reproduces a menu on
  demand; a rule body has no other eager route into the session.

A store that fits the budget is delivered exactly as before, with no notice.

Edits to your store take effect on the next tool call, no need to restart the server after changing a file. The eagerly-loaded rules are fixed when the session starts, so changes to those apply from the next session.

---

## Seeing what your profiles offer

As a store grows, it gets hard to tell what your profiles actually put in front
of an agent. `contour list --profiles` cross-references the two:

```bash
contour list --profiles
```

```
contour store: /Users/you/contour
2 profiles: cli, python

rules (4)
  rules/general/010-comm
      comm
      tags: general, style
      profiles: cli, python
  rules/legacy/010-soap
      soap
      tags: legacy
      profiles: none — not offered at session start
  rules/python/010-errors
      errs
      tags: python
      profiles: python
  rules/010-logging  (local)
      local convention
      profiles: always active (local)

2 items are in no profile.
```

**"No profile" does not mean unreachable.** The `list`, `search` and `get` tools
serve the whole store regardless of profile, so an agent can always find such an
item. What it lacks is *disclosure*: nothing puts it in front of the agent when
a session starts, so it is only ever used if the agent goes looking for it.

That is the difference between an item being **offered** and merely being
**available** — and it is worth knowing deliberately. A rule you assumed was in
effect everywhere, sitting under a tag no profile selects, is a silent gap. Give
it a tag one of your profiles selects and it moves from available to offered.

Project-local items need no profile by default — being local to the project is
scope enough — so they show as always active, alongside any profiles that also
select them. If every preset in the project uses `local: selected`, they get in
only through profiles, and one that no profile selects is reported like any
other item nothing offers. Files listed in `eager_files` are always active
either way.

It is a flag on `list` rather than a command of its own, so the same kind
argument narrows it:

```bash
contour list skills --profiles
```

The closing tally counts only the kinds shown. `--profiles` is CLI-only: the
`list` tool an agent sees is unchanged, since how its context was assembled is
your concern, not the agent's.

This is decidable from the store alone — no usage data needed. It pairs with the
stats below, which tell you what agents actually reached for once the items *are*
offered.

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
