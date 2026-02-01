package main

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"github.com/spf13/cobra"
)

//go:embed templates/*
var templateFS embed.FS

// Block represents a time block or note
type Block struct {
	ID           string
	Week         string
	Day          string
	Description  string
	PlannedStart sql.NullString
	PlannedEnd   sql.NullString
	ActualStart  sql.NullString
	ActualEnd    sql.NullString
	IsNote       bool
	IsUnplanned  bool
	IsDone       bool
	Tags         sql.NullString
	CreatedAt    time.Time
}

var db *sql.DB

func main() {
	initDB()
	defer db.Close()

	rootCmd := &cobra.Command{
		Use:   "wk",
		Short: "Week planner - plan your time blocks and track reality",
	}

	// wk add [day] <start>-<end> "<desc>"
	addCmd := &cobra.Command{
		Use:   "add [day] <start>-<end> <description>",
		Short: "Add a planned time block (day defaults to today)",
		Args:  cobra.MinimumNArgs(2),
		Run:   cmdAdd,
	}
	addCmd.Flags().StringP("tag", "t", "", "Tag for the block (or use #hashtag in description)")

	// wk note [day] "<text>"
	noteCmd := &cobra.Command{
		Use:   "note [day] <text>",
		Short: "Add a floating note (day defaults to today)",
		Args:  cobra.MinimumNArgs(1),
		Run:   cmdNote,
	}
	noteCmd.Flags().StringP("tag", "t", "", "Tag for the note (or use #hashtag in text)")

	// wk actual <id> <start>-<end>
	actualCmd := &cobra.Command{
		Use:   "actual <id> <start>-<end>",
		Short: "Record actual time for a block",
		Args:  cobra.MinimumNArgs(2),
		Run:   cmdActual,
	}
	actualCmd.Flags().Bool("unplanned", false, "Record an unplanned block")
	actualCmd.Flags().StringP("tag", "t", "", "Tag for unplanned blocks (or use #hashtag in description)")

	// wk done <id>
	doneCmd := &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a block as done",
		Args:  cobra.ExactArgs(1),
		Run:   cmdDone,
	}

	// wk undone <id>
	undoneCmd := &cobra.Command{
		Use:   "undone <id>",
		Short: "Unmark a block as done",
		Args:  cobra.ExactArgs(1),
		Run:   cmdUndone,
	}

	// wk rm <id>
	rmCmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a block",
		Args:  cobra.ExactArgs(1),
		Run:   cmdRm,
	}

	// wk ls
	lsCmd := &cobra.Command{
		Use:   "ls [day]",
		Short: "List blocks for current week or specific day",
		Args:  cobra.MaximumNArgs(1),
		Run:   cmdLs,
	}
	lsCmd.Flags().Bool("last", false, "Show last week")
	lsCmd.Flags().Bool("next", false, "Show next week")
	lsCmd.Flags().String("week", "", "Show specific week (e.g., 2025-W06)")

	// wk serve
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web viewer",
		Run:   cmdServe,
	}
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")

	rootCmd.AddCommand(addCmd, noteCmd, actualCmd, doneCmd, undoneCmd, rmCmd, lsCmd, serveCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func initDB() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home dir: %v\n", err)
		os.Exit(1)
	}

	dbDir := filepath.Join(home, ".wk")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating db dir: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(dbDir, "week.db")
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS blocks (
		id TEXT PRIMARY KEY,
		week TEXT NOT NULL,
		day TEXT NOT NULL,
		description TEXT NOT NULL,
		planned_start TEXT,
		planned_end TEXT,
		actual_start TEXT,
		actual_end TEXT,
		is_note INTEGER DEFAULT 0,
		is_unplanned INTEGER DEFAULT 0,
		is_done INTEGER DEFAULT 0,
		tags TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_week ON blocks(week);
	CREATE INDEX IF NOT EXISTS idx_week_day ON blocks(week, day);
	`
	if _, err := db.Exec(schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating schema: %v\n", err)
		os.Exit(1)
	}

	// Migration: add tags column to existing databases (ignore error if exists)
	db.Exec(`ALTER TABLE blocks ADD COLUMN tags TEXT`)
}

func generateID() string {
	bytes := make([]byte, 3) // 3 bytes = 6 hex chars
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// parseDay handles: monday, +monday (next week), 2025-02-10 (explicit date), today
func parseDay(input string) (week string, day string, err error) {
	input = strings.ToLower(strings.TrimSpace(input))

	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	dayMap := make(map[string]int)
	for i, d := range days {
		dayMap[d] = i
	}

	// Handle "today"
	if input == "today" {
		now := time.Now()
		year, isoWeek := now.ISOWeek()
		week = fmt.Sprintf("%d-W%02d", year, isoWeek)
		day = strings.ToLower(now.Weekday().String())
		return week, day, nil
	}

	// Explicit date: 2025-02-10
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, input); matched {
		t, err := time.Parse("2006-01-02", input)
		if err != nil {
			return "", "", fmt.Errorf("invalid date format: %s", input)
		}
		year, isoWeek := t.ISOWeek()
		week = fmt.Sprintf("%d-W%02d", year, isoWeek)
		day = strings.ToLower(t.Weekday().String())
		if day == "sunday" {
			day = "sunday"
		}
		return week, day, nil
	}

	// Next week: +monday
	nextWeek := false
	if strings.HasPrefix(input, "+") {
		nextWeek = true
		input = input[1:]
	}

	if _, ok := dayMap[input]; !ok {
		return "", "", fmt.Errorf("invalid day: %s", input)
	}

	now := time.Now()
	year, isoWeek := now.ISOWeek()
	if nextWeek {
		isoWeek++
		if isoWeek > 52 {
			year++
			isoWeek = 1
		}
	}

	return fmt.Sprintf("%d-W%02d", year, isoWeek), input, nil
}

// parseTimeRange parses "14:00-16:00" into start and end
func parseTimeRange(input string) (start string, end string, err error) {
	parts := strings.Split(input, "-")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid time range: %s (expected HH:MM-HH:MM)", input)
	}

	timeRegex := regexp.MustCompile(`^\d{1,2}:\d{2}$`)
	start = strings.TrimSpace(parts[0])
	end = strings.TrimSpace(parts[1])

	if !timeRegex.MatchString(start) || !timeRegex.MatchString(end) {
		return "", "", fmt.Errorf("invalid time format: %s (expected HH:MM-HH:MM)", input)
	}

	// Normalize to HH:MM
	if len(start) == 4 {
		start = "0" + start
	}
	if len(end) == 4 {
		end = "0" + end
	}

	return start, end, nil
}

// isTimeRange checks if a string looks like "HH:MM-HH:MM"
func isTimeRange(s string) bool {
	matched, _ := regexp.MatchString(`^\d{1,2}:\d{2}-\d{1,2}:\d{2}$`, s)
	return matched
}

// isDayArg checks if a string looks like a day argument
func isDayArg(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday", "today"}
	for _, d := range days {
		if s == d || s == "+"+d {
			return true
		}
	}
	// Check for explicit date YYYY-MM-DD
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, s); matched {
		return true
	}
	// Check for +day
	if strings.HasPrefix(s, "+") {
		rest := s[1:]
		for _, d := range days[:7] { // exclude "today" for +today
			if rest == d {
				return true
			}
		}
	}
	return false
}

// extractTags extracts #hashtags from description and returns cleaned desc + tags
func extractTags(desc string, flagTag string) (cleanDesc string, tags string) {
	hashtagRe := regexp.MustCompile(`#(\w+)`)
	matches := hashtagRe.FindAllStringSubmatch(desc, -1)

	var tagList []string

	// Add flag tag first if present
	if flagTag != "" {
		tagList = append(tagList, strings.ToLower(flagTag))
	}

	// Extract hashtags from description
	for _, m := range matches {
		tagList = append(tagList, strings.ToLower(m[1]))
	}

	// Remove hashtags from description
	cleanDesc = strings.TrimSpace(hashtagRe.ReplaceAllString(desc, ""))

	// Dedupe tags
	seen := make(map[string]bool)
	var uniqueTags []string
	for _, t := range tagList {
		if !seen[t] {
			seen[t] = true
			uniqueTags = append(uniqueTags, t)
		}
	}

	if len(uniqueTags) > 0 {
		tags = strings.Join(uniqueTags, ",")
	}
	return cleanDesc, tags
}

func cmdAdd(cmd *cobra.Command, args []string) {
	var dayArg, timeArg string
	var descArgs []string

	// Detect if first arg is a time range (day omitted, defaults to today)
	if isTimeRange(args[0]) {
		dayArg = "today"
		timeArg = args[0]
		descArgs = args[1:]
	} else {
		dayArg = args[0]
		timeArg = args[1]
		descArgs = args[2:]
	}

	week, day, err := parseDay(dayArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	start, end, err := parseTimeRange(timeArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	flagTag, _ := cmd.Flags().GetString("tag")
	rawDesc := strings.Join(descArgs, " ")
	desc, tags := extractTags(rawDesc, flagTag)

	id := generateID()

	_, err = db.Exec(`
		INSERT INTO blocks (id, week, day, description, planned_start, planned_end, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, week, day, desc, start, end, tags)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding block: %v\n", err)
		os.Exit(1)
	}

	tagStr := ""
	if tags != "" {
		tagStr = fmt.Sprintf(" [%s]", tags)
	}
	fmt.Printf("[%s] Added: %s %s-%s %s%s\n", id, day, start, end, desc, tagStr)
}

func cmdNote(cmd *cobra.Command, args []string) {
	var dayArg string
	var descArgs []string

	// Check if first arg looks like a day
	if isDayArg(args[0]) {
		dayArg = args[0]
		descArgs = args[1:]
	} else {
		dayArg = "today"
		descArgs = args
	}

	if len(descArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: note text required\n")
		os.Exit(1)
	}

	week, day, err := parseDay(dayArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	flagTag, _ := cmd.Flags().GetString("tag")
	rawDesc := strings.Join(descArgs, " ")
	desc, tags := extractTags(rawDesc, flagTag)

	id := generateID()

	_, err = db.Exec(`
		INSERT INTO blocks (id, week, day, description, is_note, tags)
		VALUES (?, ?, ?, ?, 1, ?)
	`, id, week, day, desc, tags)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding note: %v\n", err)
		os.Exit(1)
	}

	tagStr := ""
	if tags != "" {
		tagStr = fmt.Sprintf(" [%s]", tags)
	}
	fmt.Printf("[%s] Note added to %s: %s%s\n", id, day, desc, tagStr)
}

func cmdActual(cmd *cobra.Command, args []string) {
	unplanned, _ := cmd.Flags().GetBool("unplanned")

	if unplanned {
		// wk actual --unplanned [day] <start>-<end> "<desc>"
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: wk actual --unplanned [day] <start>-<end> <description>\n")
			os.Exit(1)
		}

		var dayArg, timeArg string
		var descArgs []string

		// Detect if first arg is a time range (day omitted, defaults to today)
		if isTimeRange(args[0]) {
			dayArg = "today"
			timeArg = args[0]
			descArgs = args[1:]
		} else {
			dayArg = args[0]
			timeArg = args[1]
			descArgs = args[2:]
		}

		week, day, err := parseDay(dayArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		start, end, err := parseTimeRange(timeArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(descArgs) == 0 {
			fmt.Fprintf(os.Stderr, "Error: description required\n")
			os.Exit(1)
		}

		flagTag, _ := cmd.Flags().GetString("tag")
		rawDesc := strings.Join(descArgs, " ")
		desc, tags := extractTags(rawDesc, flagTag)

		id := generateID()

		_, err = db.Exec(`
			INSERT INTO blocks (id, week, day, description, actual_start, actual_end, is_unplanned, is_done, tags)
			VALUES (?, ?, ?, ?, ?, ?, 1, 1, ?)
		`, id, week, day, desc, start, end, tags)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error adding unplanned block: %v\n", err)
			os.Exit(1)
		}

		tagStr := ""
		if tags != "" {
			tagStr = fmt.Sprintf(" [%s]", tags)
		}
		fmt.Printf("[%s] ⚡ Unplanned: %s %s-%s %s%s\n", id, day, start, end, desc, tagStr)
		return
	}

	// wk actual <id> <start>-<end>
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: wk actual <id> <start>-<end>\n")
		os.Exit(1)
	}

	id := args[0]
	start, end, err := parseTimeRange(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	result, err := db.Exec(`
		UPDATE blocks SET actual_start = ?, actual_end = ? WHERE id = ?
	`, start, end, id)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating block: %v\n", err)
		os.Exit(1)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Fprintf(os.Stderr, "Block not found: %s\n", id)
		os.Exit(1)
	}

	fmt.Printf("[%s] Actual time recorded: %s-%s\n", id, start, end)
}

func cmdDone(cmd *cobra.Command, args []string) {
	result, err := db.Exec(`UPDATE blocks SET is_done = 1 WHERE id = ?`, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Fprintf(os.Stderr, "Block not found: %s\n", args[0])
		os.Exit(1)
	}
	fmt.Printf("[%s] ✓ Marked done\n", args[0])
}

func cmdUndone(cmd *cobra.Command, args []string) {
	result, err := db.Exec(`UPDATE blocks SET is_done = 0 WHERE id = ?`, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Fprintf(os.Stderr, "Block not found: %s\n", args[0])
		os.Exit(1)
	}
	fmt.Printf("[%s] Unmarked done\n", args[0])
}

func cmdRm(cmd *cobra.Command, args []string) {
	result, err := db.Exec(`DELETE FROM blocks WHERE id = ?`, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Fprintf(os.Stderr, "Block not found: %s\n", args[0])
		os.Exit(1)
	}
	fmt.Printf("[%s] Deleted\n", args[0])
}

func getWeek(cmd *cobra.Command) string {
	if w, _ := cmd.Flags().GetString("week"); w != "" {
		return w
	}

	now := time.Now()
	year, isoWeek := now.ISOWeek()

	if last, _ := cmd.Flags().GetBool("last"); last {
		isoWeek--
		if isoWeek < 1 {
			year--
			isoWeek = 52
		}
	} else if next, _ := cmd.Flags().GetBool("next"); next {
		isoWeek++
		if isoWeek > 52 {
			year++
			isoWeek = 1
		}
	}

	return fmt.Sprintf("%d-W%02d", year, isoWeek)
}

func weekDateRange(week string) string {
	// Parse 2025-W06 and return "Feb 3 - Feb 9"
	var year, weekNum int
	fmt.Sscanf(week, "%d-W%d", &year, &weekNum)

	// Find the Monday of that ISO week
	jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	daysToMonday := int(time.Monday - jan1.Weekday())
	if daysToMonday > 0 {
		daysToMonday -= 7
	}
	firstMonday := jan1.AddDate(0, 0, daysToMonday)
	monday := firstMonday.AddDate(0, 0, (weekNum-1)*7)
	sunday := monday.AddDate(0, 0, 6)

	return fmt.Sprintf("%s - %s", monday.Format("Jan 2"), sunday.Format("Jan 2"))
}

func dayDate(week, day string) string {
	var year, weekNum int
	fmt.Sscanf(week, "%d-W%d", &year, &weekNum)

	jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	daysToMonday := int(time.Monday - jan1.Weekday())
	if daysToMonday > 0 {
		daysToMonday -= 7
	}
	firstMonday := jan1.AddDate(0, 0, daysToMonday)
	monday := firstMonday.AddDate(0, 0, (weekNum-1)*7)

	dayOffsets := map[string]int{
		"monday": 0, "tuesday": 1, "wednesday": 2, "thursday": 3,
		"friday": 4, "saturday": 5, "sunday": 6,
	}
	targetDate := monday.AddDate(0, 0, dayOffsets[day])
	return targetDate.Format("Jan 2")
}

func cmdLs(cmd *cobra.Command, args []string) {
	week := getWeek(cmd)
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

	var filterDay string
	if len(args) > 0 {
		_, parsedDay, err := parseDay(args[0])
		if err == nil {
			filterDay = parsedDay
		} else {
			filterDay = strings.ToLower(args[0])
		}
	}

	fmt.Printf("\nWeek %s (%s)\n", week, weekDateRange(week))
	fmt.Println(strings.Repeat("─", 50))

	for _, day := range days {
		if filterDay != "" && filterDay != day {
			continue
		}

		rows, err := db.Query(`
			SELECT id, description, planned_start, planned_end, actual_start, actual_end, is_note, is_unplanned, is_done, tags
			FROM blocks WHERE week = ? AND day = ?
			ORDER BY 
				CASE WHEN planned_start IS NOT NULL THEN planned_start 
				     WHEN actual_start IS NOT NULL THEN actual_start 
				     ELSE '99:99' END,
				created_at
		`, week, day)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error querying: %v\n", err)
			continue
		}

		var blocks []Block
		for rows.Next() {
			var b Block
			rows.Scan(&b.ID, &b.Description, &b.PlannedStart, &b.PlannedEnd,
				&b.ActualStart, &b.ActualEnd, &b.IsNote, &b.IsUnplanned, &b.IsDone, &b.Tags)
			blocks = append(blocks, b)
		}
		rows.Close()

		if len(blocks) == 0 && filterDay == "" {
			continue
		}

		fmt.Printf("\n%s (%s)\n", strings.ToUpper(day), dayDate(week, day))

		for _, b := range blocks {
			tagStr := ""
			if b.Tags.Valid && b.Tags.String != "" {
				tagStr = fmt.Sprintf(" #%s", strings.ReplaceAll(b.Tags.String, ",", " #"))
			}

			if b.IsNote {
				fmt.Printf("  • %s%s\n", b.Description, tagStr)
				continue
			}

			status := " "
			if b.IsDone {
				status = "✓"
			}
			if b.IsUnplanned {
				status = "⚡"
			}

			timeStr := ""
			if b.IsUnplanned {
				timeStr = fmt.Sprintf("%s-%s", b.ActualStart.String, b.ActualEnd.String)
			} else if b.ActualStart.Valid {
				timeStr = fmt.Sprintf("%s-%s → %s-%s",
					b.PlannedStart.String, b.PlannedEnd.String,
					b.ActualStart.String, b.ActualEnd.String)
			} else {
				timeStr = fmt.Sprintf("%s-%s", b.PlannedStart.String, b.PlannedEnd.String)
			}

			fmt.Printf("  [%s] %s %-23s %s%s\n", b.ID, status, timeStr, b.Description, tagStr)
		}
	}
	fmt.Println()
}

func cmdServe(cmd *cobra.Command, args []string) {
	port, _ := cmd.Flags().GetInt("port")

	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing templates: %v\n", err)
		os.Exit(1)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		week := r.URL.Query().Get("week")
		if week == "" {
			year, isoWeek := time.Now().ISOWeek()
			week = fmt.Sprintf("%d-W%02d", year, isoWeek)
		}

		days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
		data := struct {
			Week      string
			DateRange string
			Days      []DayData
			PrevWeek  string
			NextWeek  string
		}{
			Week:      week,
			DateRange: weekDateRange(week),
			Days:      make([]DayData, 0),
		}

		// Calculate prev/next weeks
		var year, weekNum int
		fmt.Sscanf(week, "%d-W%d", &year, &weekNum)
		prevWeek := weekNum - 1
		prevYear := year
		if prevWeek < 1 {
			prevYear--
			prevWeek = 52
		}
		nextWeek := weekNum + 1
		nextYear := year
		if nextWeek > 52 {
			nextYear++
			nextWeek = 1
		}
		data.PrevWeek = fmt.Sprintf("%d-W%02d", prevYear, prevWeek)
		data.NextWeek = fmt.Sprintf("%d-W%02d", nextYear, nextWeek)

		for _, day := range days {
			dayData := DayData{Name: strings.Title(day)}

			rows, _ := db.Query(`
				SELECT id, description, planned_start, planned_end, actual_start, actual_end, is_note, is_unplanned, is_done, tags
				FROM blocks WHERE week = ? AND day = ?
				ORDER BY 
					CASE WHEN planned_start IS NOT NULL THEN planned_start 
					     WHEN actual_start IS NOT NULL THEN actual_start 
					     ELSE '99:99' END,
					created_at
			`, week, day)

			for rows.Next() {
				var b Block
				rows.Scan(&b.ID, &b.Description, &b.PlannedStart, &b.PlannedEnd,
					&b.ActualStart, &b.ActualEnd, &b.IsNote, &b.IsUnplanned, &b.IsDone, &b.Tags)
				dayData.Blocks = append(dayData.Blocks, b)
			}
			rows.Close()

			data.Days = append(data.Days, dayData)
		}

		tmpl.ExecuteTemplate(w, "index.html", data)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	fmt.Printf("🗓️  Week viewer running at http://%s\n", addr)
	fmt.Println("Press Ctrl+C to stop")

	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

type DayData struct {
	Name   string
	Blocks []Block
}
