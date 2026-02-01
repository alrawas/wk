# wk - Week Planner CLI

Use this skill when the user asks to plan their week, schedule time blocks, track what they actually did, or review their schedule. Trigger phrases: "plan my week", "schedule", "time block", "what's on my calendar", "add to my schedule", "track my time".

## Prerequisites

Before using wk commands, verify it's installed:

```bash
which wk || echo "wk not found"
```

If not installed:
```bash
go install github.com/alrawas/wk@latest
```

Ensure `$GOPATH/bin` (usually `~/go/bin`) is in PATH. If still not found:
```bash
export PATH="$PATH:$HOME/go/bin"
```

## Commands

```bash
# Add planned time blocks
wk add <time-range> "<description>"              # today
wk add <day> <time-range> "<description>"        # specific day
wk add +<day> <time-range> "<description>"       # next week
wk add <YYYY-MM-DD> <time-range> "<description>" # explicit date

# Add notes
wk note "<text>"                                 # today
wk note <day> "<text>"                           # specific day

# Tags (use -t flag or #hashtag in description)
wk add 09:00-11:00 "deep work" -t project
wk add 09:00-11:00 "deep work #project"

# Record what actually happened
wk actual <id> <time-range>                      # actual time for planned block
wk actual --unplanned <time-range> "<desc>"      # unplanned today
wk actual --unplanned <day> <time-range> "<desc>" # unplanned specific day

# Manage blocks
wk done <id>                                     # mark complete
wk undone <id>                                   # unmark
wk rm <id>                                       # delete

# View schedule
wk ls                                            # current week
wk ls <day>                                      # specific day
wk ls --last                                     # last week
wk ls --next                                     # next week
wk ls --week 2025-W06                            # specific week
```

## Time & Day Formats

- **Time range**: `HH:MM-HH:MM` (e.g., `09:00-11:00`, `14:30-16:00`)
- **Days**: `monday`, `tuesday`, `wednesday`, `thursday`, `friday`, `saturday`, `sunday`, `today`
- **Next week**: prefix with `+` (e.g., `+monday`)
- **Explicit date**: `YYYY-MM-DD` (e.g., `2025-02-10`)

## Output Format

```
Week 2025-W06 (Feb 3 - Feb 9)
──────────────────────────────────────────────────

MONDAY (Feb 3)
  [a3f2c1] ✓ 14:00-16:00 → 14:30-17:00  backend work #project
  [x7q9b2]   17:00-18:00                 team sync
  [u2p4m8] ⚡ 19:00-20:00                 emergency call #client
  • energy was decent today
```

**Symbols**: ✓ done, ⚡ unplanned, → actual differed from planned, • note

## Examples

User: "Schedule deep work tomorrow morning 9-11"
```bash
# Use the day name (e.g., if today is Monday, use tuesday)
wk add tuesday 09:00-11:00 "deep work"
```

User: "I have a client call Friday 2-3pm, tag it acme"
```bash
wk add friday 14:00-15:00 "client call #acme"
```

User: "What's on my schedule this week?"
```bash
wk ls
```

User: "Mark that meeting done" (after seeing id in output)
```bash
wk done <id>
```

User: "I just had an unplanned 30 min call"
```bash
wk actual --unplanned 14:00-14:30 "unplanned call"
```

User: "Add a note that I felt productive today"
```bash
wk note "felt productive today"
```
