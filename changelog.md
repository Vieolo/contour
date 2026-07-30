# Change Log

## v0.2.4 (2026-07-30)
- A Go project that keeps a `go.yaml` can hold contour's per-project settings in its `external.contour` section instead of a separate `.contour.yaml`. The fields are unchanged. `contour mcp-init` detects an existing `go.yaml` and writes there, preserving the rest of the file. `.contour.yaml` takes precedence over any such host manifest

## v0.2.3 (2026-07-27)
- Added the `bootstrap` MCP tool, returning the complete session payload. It will return every rule in effect plus the full skills and knowledge menu.
- Usage logging records a `bootstrap` event, so `contour stats` can show whether agents act on an incomplete-instructions notice
- Fixed the situation where the eager rules were silently truncated in MCP sessions. Long rules used to be silently truncated on startup and now, the startup introduction will tell the agent the exact size of the total eager rules and instruct it to run the bootstrap tool to get all the rules

## v0.2.2 (2026-07-26)
- Added support for the per-project config to `.contour.yaml` at the project root
- Added support for loading the per-project context files from `.contour`, `.agents` and `.claude` folders, layered over the central store. Local items are always active, and are authoritative where they conflict with a central item
- Added support for the eager loading of per-project `AGENTS.md`/`CLAUDE.md` via `eager_files` in the project config
- Added `contour list --profiles`, cross-referencing every item against the bootstrap profiles: which profiles offer it at session start, and which items no profile offers
- `contour` now accepts a list of bootstrap profiles to be loaded together. `contour bootstrap python cli`, `bootstrap: [python, cli]` in the project config, or a repeated `--bootstrap` flag, compose several entry points in the order given, so a Python project that also ships a CLI need not have a `python-cli` profile written for it. A bare `bootstrap: python` keeps working unchanged

## v0.2.1 (2026-07-25)
- Added `mcp-init` command to CLI to generate a `.mcp.json` in a project for the AI agent to use
- Improved the structure of the results of the `list` and `search` tools of the MCP
- Added usage logging, with a dedicated field in the config to switch it off
- Added `stats` command to show the stats of the usage, allowing you to understands the gaps and usage patterns

## v0.2.0 (2026-07-24)
- Introducing the config file
- Added the `set-home` command to change the location of the store and move the store to the new location. `contour set-home here` will set the location of the store to the current directory
- Added the `home` command to show the current location of the store

#### Breaking Changes
- contour no longer relies on the env vars and relies on the config file to locate the store
- The default store location is moved to `~/contour` and `~/.contour` is reserved to hold the config file

## v0.1.0 (2026-07-23)
- Initial release
