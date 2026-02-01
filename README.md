# wk

A minimal CLI week planner. Plan time blocks, track what actually happened, tag by project.

```
Week 2025-W06 (Feb 3 - Feb 9)
──────────────────────────────────────────────────

MONDAY
  [a3f2c1] ✓ 14:00-16:00 → 14:30-17:00  backend work #booq
  [x7q9b2]   17:00-18:00                 team sync
  [u2p4m8] ⚡ 19:00-20:00                 emergency client call #acme
  • energy was decent, got distracted mid-afternoon
```

## Install

```bash
go install github.com/alrawas/wk@latest
```

Make sure `$GOPATH/bin` (or `$HOME/go/bin`) is in your `PATH`.

## Usage

### Add Blocks

```bash
wk add 09:00-11:00 "deep work"                # today
wk add monday 14:00-16:00 "client call"       # specific day
wk add +tuesday 10:00-12:00 "planning"        # next week
wk add 2025-02-10 14:00-16:00 "deadline"      # explicit date
```

### Add Notes

```bash
wk note "energy was low"                      # today
wk note friday "productive day"               # specific day
```

### Tags

```bash
wk add 09:00-11:00 "backend work" -t booq     # flag
wk add 09:00-11:00 "backend work #booq"       # hashtag
wk add 09:00-11:00 "sync #booq #shop"         # multiple
```

### Record Reality

```bash
# Actual time for a planned block
wk actual <id> 19:00-20:30

# Unplanned events
wk actual --unplanned 14:00-15:00 "emergency call"
wk actual --unplanned monday 15:00-17:00 "client sync #acme"
```

### Status

```bash
wk done <id>                                  # mark complete
wk undone <id>                                # unmark
wk rm <id>                                    # delete
```

### View

```bash
wk ls                                         # current week
wk ls monday                                  # specific day
wk ls --last                                  # last week
wk ls --next                                  # next week
wk ls --week 2025-W06                         # specific week
```

### Web Viewer

```bash
wk serve                                      # localhost:8080
wk serve --port 3000                          # custom port
```

Read-only week grid. Dark theme. Access via SSH port forwarding:

```bash
ssh -L 8080:localhost:8080 your-server
# then open http://localhost:8080
```

## Legend

| Symbol | Meaning |
|--------|---------|
| ✓ | done |
| ⚡ | unplanned |
| → | actual time differed from planned |
| • | note |

## Data

Database: `~/.wk/week.db` (SQLite)

```bash
# Backup
cp ~/.wk/week.db ~/.wk/week.db.backup
```

## Cross-Compile

Uses pure-Go SQLite (`modernc.org/sqlite`), no CGO. Build anywhere:

```bash
GOOS=linux GOARCH=amd64 go build -o wk .
```

## AI Agent Integration

The CLI is agent-friendly. To add as a Claude Code slash command:

```bash
mkdir -p ~/.claude/commands
cp skills/wk-planner.md ~/.claude/commands/wk.md
```

Then use `/wk` in any Claude Code session to load the skill.

## License

MIT
