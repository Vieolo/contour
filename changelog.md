# Change Log

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
