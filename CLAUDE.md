# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
go build -o wk .              # Build binary
go run .                      # Run directly
go install github.com/alrawas/wk@latest  # Install globally

# Cross-compile (no CGO - uses modernc.org/sqlite)
GOOS=linux GOARCH=amd64 go build -o wk .
```

No tests exist yet. Database is at `~/.wk/week.db`.

## Architecture

Single-file Go CLI (`main.go`) using:
- **spf13/cobra** - CLI command structure
- **modernc.org/sqlite** - pure-Go SQLite (no CGO)
- **embed** - bundled HTML templates in `templates/`

### Data Model

Single `Block` struct handles three entry types:
- **Time blocks**: `PlannedStart`/`PlannedEnd`, optionally `ActualStart`/`ActualEnd`
- **Notes**: `IsNote=true`, no time fields
- **Unplanned**: `IsUnplanned=true`, only actual times

All entries belong to an ISO week (`2025-W06` format) and day name.

### Command Structure

All commands defined in `main()`:
- `add` - planned time blocks
- `note` - floating notes
- `actual` - record actual times (or create unplanned with `--unplanned`)
- `done`/`undone` - toggle completion
- `rm` - delete
- `ls` - list (supports `--last`, `--next`, `--week`)
- `serve` - web viewer

### Web Server

`cmdServe()` serves a read-only week grid from embedded `templates/index.html`. Uses Go's `html/template` with the `Block` struct directly.

### Key Functions

- `parseDay()` - handles `today`, `monday`..`sunday`, `+monday` (next week), `YYYY-MM-DD`
- `parseTimeRange()` - parses `HH:MM-HH:MM` format
- `extractTags()` - extracts `#hashtags` from description text
- `weekDateRange()` - converts `2025-W06` to `Feb 3 - Feb 9` display format
