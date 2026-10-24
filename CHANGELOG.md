# Changelog

## [0.6.0]

- Add built-in changelog viewer (`c`) with scrollable viewport and mouse support
- Add `g` / `G` to jump to the top / bottom of the changelog
- Add `m` to return to the main view from the changelog
- Add `[c]` and `[enter]` hints to the footer
- Add Homebrew upgrade instructions to README

## [0.5.1]

- Add `[enter]` hint in footer
- Lower Go version to 1.26.0
- Add logo and installation guide to README

## [0.5.0]

- Add basic functionality
- List all TCP ports in `LISTEN` state via `lsof`
- Add terminal UI with styled table headers and highlighted selection
- Add search mode to filter by PID, process name, or address
- Add process kill with `y`/`enter` confirmation
- Add `r` shortcut to reload the list
- Keep search mode and selection stable while filtering or after killing a process
- Add debug logger
- Refactor the CLI app into modules
