package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// =========================================================
// A. Bug Health / Risk Score
// A transparent, deterministic 0-100 score built only from
// data already in the database. Higher = riskier.
// =========================================================

type HealthScoreReason struct {
	Factor string `json:"factor"`
	Points int    `json:"points"`
	Detail string `json:"detail"`
}

type HealthScoreResponse struct {
	IssueKey string               `json:"issue_key"`
	Score    int                  `json:"score"` // 0-100, higher = riskier
	Level    string               `json:"level"` // low / moderate / high / critical
	Reasons  []HealthScoreReason  `json:"reasons"`
}

// GET /issues/{id}/health-score
func getHealthScoreHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	ctx := context.Background()

	var status, priority, severity string
	var reopenCount int
	var createdAt, updatedAt time.Time
	err := dbpool.QueryRow(ctx, `
		SELECT status, priority, severity, reopen_count, created_at, updated_at
		FROM issues WHERE issue_key = $1
	`, issueKey).Scan(&status, &priority, &severity, &reopenCount, &createdAt, &updatedAt)
	if err != nil {
		http.Error(w, "Issue not found", http.StatusNotFound)
		return
	}

	var lastActivity time.Time
	err = dbpool.QueryRow(ctx,
		"SELECT COALESCE(MAX(created_at), $1) FROM activity_stream WHERE issue_key = $2",
		updatedAt, issueKey).Scan(&lastActivity)
	if err != nil {
		lastActivity = updatedAt
	}

	var unresolvedBlockers int
	dbpool.QueryRow(ctx, `
		SELECT COUNT(*) FROM issue_relationships ir
		JOIN issues bi ON bi.issue_key = ir.related_issue_key
		WHERE ir.issue_key = $1 AND ir.relation_type = 'blocked_by'
		AND bi.status NOT IN ('resolved','verified','closed')
	`, issueKey).Scan(&unresolvedBlockers)

	ageDays := int(time.Since(createdAt).Hours() / 24)
	inactivityDays := int(time.Since(lastActivity).Hours() / 24)

	reasons := []HealthScoreReason{}
	score := 0

	addReason := func(factor string, points int, detail string) {
		if points <= 0 {
			return
		}
		score += points
		reasons = append(reasons, HealthScoreReason{Factor: factor, Points: points, Detail: detail})
	}

	sevPoints := map[string]int{"blocker": 30, "critical": 25, "major": 12, "minor": 4}[severity]
	addReason("severity", sevPoints, fmt.Sprintf("Severity is '%s'", severity))

	priPoints := map[string]int{"critical": 20, "high": 14, "medium": 7, "low": 2}[priority]
	addReason("priority", priPoints, fmt.Sprintf("Priority is '%s'", priority))

	agePoints := ageDays
	if agePoints > 20 {
		agePoints = 20
	}
	addReason("age", agePoints, fmt.Sprintf("Open for %d day(s)", ageDays))

	inactivityPoints := inactivityDays * 2
	if inactivityPoints > 20 {
		inactivityPoints = 20
	}
	addReason("inactivity", inactivityPoints, fmt.Sprintf("No activity for %d day(s)", inactivityDays))

	reopenPoints := reopenCount * 10
	if reopenPoints > 20 {
		reopenPoints = 20
	}
	addReason("reopen_count", reopenPoints, fmt.Sprintf("Reopened %d time(s)", reopenCount))

	blockerPoints := unresolvedBlockers * 15
	if blockerPoints > 30 {
		blockerPoints = 30
	}
	addReason("unresolved_blockers", blockerPoints, fmt.Sprintf("%d unresolved blocker(s)", unresolvedBlockers))

	if score > 100 {
		score = 100
	}

	level := "low"
	switch {
	case score >= 70:
		level = "critical"
	case score >= 45:
		level = "high"
	case score >= 20:
		level = "moderate"
	}

	// A closed/verified issue isn't "at risk" regardless of the raw score.
	if status == "closed" || status == "verified" {
		score = 0
		level = "low"
		reasons = []HealthScoreReason{{Factor: "status", Points: 0, Detail: fmt.Sprintf("Issue is %s", status)}}
	}

	resp := HealthScoreResponse{IssueKey: issueKey, Score: score, Level: level, Reasons: reasons}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// =========================================================
// B. Smart Duplicate Suggestions
// Deterministic title/keyword overlap + component/label match.
// No external service, no paid API.
// =========================================================

type DuplicateSuggestion struct {
	IssueKey   string  `json:"issue_key"`
	Title      string  `json:"title"`
	Score      float64 `json:"score"` // 0.0 - 1.0
	Reason     string  `json:"reason"`
}

func tokenize(s string) map[string]bool {
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	set := map[string]bool{}
	// Ignore very short/common filler words so the score reflects real overlap.
	stopwords := map[string]bool{"the": true, "a": true, "an": true, "is": true, "on": true, "in": true, "to": true, "of": true, "and": true, "for": true}
	for _, wd := range words {
		if len(wd) > 2 && !stopwords[wd] {
			set[wd] = true
		}
	}
	return set
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if b[k] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// GET /issues/{id}/duplicates
func getDuplicateSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	ctx := context.Background()

	var title, body, component string
	var projectID int
	err := dbpool.QueryRow(ctx,
		"SELECT title, body, component, project_id FROM issues WHERE issue_key = $1", issueKey).
		Scan(&title, &body, &component, &projectID)
	if err != nil {
		http.Error(w, "Issue not found", http.StatusNotFound)
		return
	}

	targetTokens := tokenize(title + " " + body)

	rows, err := dbpool.Query(ctx, `
		SELECT issue_key, title, body, component FROM issues
		WHERE project_id = $1 AND issue_key != $2 AND status NOT IN ('closed')
	`, projectID, issueKey)
	if err != nil {
		http.Error(w, "Failed to scan for duplicates", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var suggestions []DuplicateSuggestion
	for rows.Next() {
		var candKey, candTitle, candBody, candComponent string
		if err := rows.Scan(&candKey, &candTitle, &candBody, &candComponent); err != nil {
			continue
		}

		candTokens := tokenize(candTitle + " " + candBody)
		textScore := jaccard(targetTokens, candTokens)

		componentBonus := 0.0
		if component != "" && component == candComponent {
			componentBonus = 0.15
		}

		score := textScore + componentBonus
		if score > 1.0 {
			score = 1.0
		}

		// Only surface genuinely plausible matches.
		if score >= 0.25 {
			reason := fmt.Sprintf("%.0f%% keyword overlap", textScore*100)
			if componentBonus > 0 {
				reason += ", same component"
			}
			suggestions = append(suggestions, DuplicateSuggestion{
				IssueKey: candKey, Title: candTitle, Score: score, Reason: reason,
			})
		}
	}

	// Simple descending sort by score (small N, insertion sort is fine).
	for i := 1; i < len(suggestions); i++ {
		for j := i; j > 0 && suggestions[j].Score > suggestions[j-1].Score; j-- {
			suggestions[j], suggestions[j-1] = suggestions[j-1], suggestions[j]
		}
	}
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}
	if suggestions == nil {
		suggestions = []DuplicateSuggestion{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
}

// =========================================================
// C. "Why is this bug stuck?"
// Explains stagnation using real stored signals and gives one
// practical suggested next action.
// =========================================================

type StuckAnalysis struct {
	IssueKey       string   `json:"issue_key"`
	IsStuck        bool     `json:"is_stuck"`
	StagnationLevel string  `json:"stagnation_level"` // none / mild / moderate / severe
	Reasons        []string `json:"reasons"`
	SuggestedAction string  `json:"suggested_action"`
}

// GET /issues/{id}/why-stuck
func getWhyStuckHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	ctx := context.Background()

	var status string
	var reopenCount int
	var createdAt, updatedAt time.Time
	var assigneeID *int
	err := dbpool.QueryRow(ctx, `
		SELECT status, reopen_count, created_at, updated_at, assignee_id
		FROM issues WHERE issue_key = $1
	`, issueKey).Scan(&status, &reopenCount, &createdAt, &updatedAt, &assigneeID)
	if err != nil {
		http.Error(w, "Issue not found", http.StatusNotFound)
		return
	}

	if status == "resolved" || status == "verified" || status == "closed" {
		resp := StuckAnalysis{
			IssueKey: issueKey, IsStuck: false, StagnationLevel: "none",
			Reasons:         []string{fmt.Sprintf("Issue is already %s", status)},
			SuggestedAction: "No action needed.",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	var lastActivity time.Time
	err = dbpool.QueryRow(ctx,
		"SELECT COALESCE(MAX(created_at), $1) FROM activity_stream WHERE issue_key = $2",
		updatedAt, issueKey).Scan(&lastActivity)
	if err != nil {
		lastActivity = updatedAt
	}
	inactivityDays := int(time.Since(lastActivity).Hours() / 24)
	ageDays := int(time.Since(createdAt).Hours() / 24)

	var unresolvedBlockers int
	dbpool.QueryRow(ctx, `
		SELECT COUNT(*) FROM issue_relationships ir
		JOIN issues bi ON bi.issue_key = ir.related_issue_key
		WHERE ir.issue_key = $1 AND ir.relation_type = 'blocked_by'
		AND bi.status NOT IN ('resolved','verified','closed')
	`, issueKey).Scan(&unresolvedBlockers)

	var assigneeOpenLoad int
	if assigneeID != nil {
		dbpool.QueryRow(ctx, `
			SELECT COUNT(*) FROM issues
			WHERE assignee_id = $1 AND status NOT IN ('resolved','verified','closed')
		`, *assigneeID).Scan(&assigneeOpenLoad)
	}

	reasons := []string{}
	severityScore := 0

	if inactivityDays >= 5 {
		reasons = append(reasons, fmt.Sprintf("No activity for %d days", inactivityDays))
		severityScore += 2
	}
	if unresolvedBlockers > 0 {
		reasons = append(reasons, fmt.Sprintf("Blocked by %d unresolved issue(s)", unresolvedBlockers))
		severityScore += 3
	}
	if reopenCount >= 2 {
		reasons = append(reasons, fmt.Sprintf("Reopened %d times, suggesting the fix isn't sticking", reopenCount))
		severityScore += 2
	}
	if ageDays >= 14 {
		reasons = append(reasons, fmt.Sprintf("Open for %d days", ageDays))
		severityScore += 1
	}
	if assigneeID == nil {
		reasons = append(reasons, "No one is assigned")
		severityScore += 2
	} else if assigneeOpenLoad > 5 {
		reasons = append(reasons, fmt.Sprintf("Assignee already has %d other open issues", assigneeOpenLoad))
		severityScore += 1
	}

	isStuck := len(reasons) > 0
	level := "none"
	action := "No action needed — this issue looks healthy."

	switch {
	case severityScore >= 6:
		level = "severe"
	case severityScore >= 3:
		level = "moderate"
	case severityScore >= 1:
		level = "mild"
	}

	switch {
	case unresolvedBlockers > 0:
		action = "Resolve the blocking issue(s) first, or re-evaluate whether this is still blocked."
	case assigneeID == nil:
		action = "Assign this issue to someone before it goes stale further."
	case inactivityDays >= 5:
		action = "Ping the assignee for a status update, or re-triage if priorities have shifted."
	case reopenCount >= 2:
		action = "Investigate why the previous fix didn't hold before attempting another resolution."
	case assigneeOpenLoad > 5:
		action = "Consider reassigning — the current assignee has a heavy open-issue load."
	}

	resp := StuckAnalysis{
		IssueKey: issueKey, IsStuck: isStuck, StagnationLevel: level,
		Reasons: reasons, SuggestedAction: action,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
