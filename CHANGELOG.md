# Changelog

## [0.6.0]

- Kill the supervising dev process instead of only the port holder
- Add `[y]` graceful (`SIGTERM`) and `[f]` force (`SIGKILL`) kill options
- Verify the process is gone before refreshing the list

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
