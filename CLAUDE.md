# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
# Build
go build -o wk .

# Run directly
go run .

# Install globally
go install github.com/alrawas/wk@latest

# Cross-compile (no CGO required - uses modernc.org/sqlite)
GOOS=linux GOARCH=amd64 go build -o wk .
```

## Architecture

This is a single-file Go CLI application (~780 lines in `main.go`) using:
- **spf13/cobra** for CLI command structure
- **modernc.org/sqlite** for pure-Go SQLite (no CGO dependency)
- **embed** for bundling HTML templates

### Data Model

Single `Block` struct handles both time blocks and notes:
- Time blocks have `PlannedStart`/`PlannedEnd`, optionally `ActualStart`/`ActualEnd`
- Notes have `IsNote=true` with no time fields
- Unplanned events have `IsUnplanned=true` with only actual times
- All entries belong to an ISO week (`2025-W06` format) and day

Database stored at `~/.wk/week.db`.

### Command Structure

All commands defined in `main()`:
- `add` - planned time blocks
- `note` - floating notes
- `actual` - record actual times (or create unplanned with `--unplanned`)
- `done`/`undone` - toggle completion
- `rm` - delete
- `ls` - list (supports `--last`, `--next`, `--week`)
- `serve` - web viewer

### Day Parsing

`parseDay()` handles multiple formats:
- `today` - current day
- `monday`..`sunday` - current week
- `+monday` - next week (prefix with `+`)
- `2025-02-10` - explicit ISO date

### Web Server

`cmdServe()` serves a read-only week grid from embedded `templates/index.html`. Uses Go's `html/template` with the `Block` struct directly.
