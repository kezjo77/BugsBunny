package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// =========================================================
// 1. Bug DNA — Root Cause Clustering
// Groups open issues by component + shared keywords to surface
// which parts of the codebase are producing the most bugs.
// =========================================================

type BugDNACluster struct {
	Component      string   `json:"component"`
	IssueCount     int      `json:"issue_count"`
	CriticalCount  int      `json:"critical_count"` // severity blocker+critical
	TopKeywords    []string `json:"top_keywords"`
	IssueKeys      []string `json:"issue_keys"`
}

// GET /projects/{id}/bug-dna
func getBugDNAHandler(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	ctx := context.Background()

	rows, err := dbpool.Query(ctx, `
		SELECT issue_key, title, body, component, severity
		FROM issues
		WHERE project_id = $1 AND status NOT IN ('closed')
	`, projectID)
	if err != nil {
		http.Error(w, "Failed to fetch issues", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type row struct{ key, title, body, component, severity string }
	var all []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.key, &rr.title, &rr.body, &rr.component, &rr.severity); err == nil {
			if rr.component == "" {
				rr.component = "(uncategorized)"
			}
			all = append(all, rr)
		}
	}

	clusterMap := map[string]*BugDNACluster{}
	keywordCounts := map[string]map[string]int{} // component -> word -> count

	for _, rr := range all {
		c, ok := clusterMap[rr.component]
		if !ok {
			c = &BugDNACluster{Component: rr.component}
			clusterMap[rr.component] = c
			keywordCounts[rr.component] = map[string]int{}
		}
		c.IssueCount++
		c.IssueKeys = append(c.IssueKeys, rr.key)
		if rr.severity == "blocker" || rr.severity == "critical" {
			c.CriticalCount++
		}
		for word := range tokenize(rr.title + " " + rr.body) {
			keywordCounts[rr.component][word]++
		}
	}

	var clusters []BugDNACluster
	for comp, c := range clusterMap {
		// Top 5 keywords by frequency for this cluster.
		type kw struct {
			word  string
			count int
		}
		var kws []kw
		for w, n := range keywordCounts[comp] {
			if n > 1 { // only words that recur across more than one issue
				kws = append(kws, kw{w, n})
			}
		}
		for i := 1; i < len(kws); i++ {
			for j := i; j > 0 && kws[j].count > kws[j-1].count; j-- {
				kws[j], kws[j-1] = kws[j-1], kws[j]
			}
		}
		limit := 5
		if len(kws) < limit {
			limit = len(kws)
		}
		for i := 0; i < limit; i++ {
			c.TopKeywords = append(c.TopKeywords, kws[i].word)
		}
		clusters = append(clusters, *c)
	}

	// Sort clusters by issue count descending — worst offenders first.
	for i := 1; i < len(clusters); i++ {
		for j := i; j > 0 && clusters[j].IssueCount > clusters[j-1].IssueCount; j-- {
			clusters[j], clusters[j-1] = clusters[j-1], clusters[j]
		}
	}
	if clusters == nil {
		clusters = []BugDNACluster{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusters)
}

// =========================================================
// 2. Predictive SLA Breach Warning
// Deterministic SLA hours by severity, adjusted by priority
// multiplier. Forecasts breach before it happens.
// =========================================================

var slaBaseHours = map[string]float64{"blocker": 24, "critical": 48, "major": 120, "minor": 240}
var priorityMultiplier = map[string]float64{"critical": 0.5, "high": 0.75, "medium": 1.0, "low": 1.5}

type SLAForecast struct {
	IssueKey       string  `json:"issue_key"`
	SLAHours       float64 `json:"sla_hours"`
	ElapsedHours   float64 `json:"elapsed_hours"`
	RemainingHours float64 `json:"remaining_hours"`
	Status         string  `json:"status"` // on_track / at_risk / breached
	Reason         string  `json:"reason"`
}

// GET /issues/{id}/sla-forecast
func getSLAForecastHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	ctx := context.Background()

	var status, severity, priority string
	var createdAt time.Time
	err := dbpool.QueryRow(ctx,
		"SELECT status, severity, priority, created_at FROM issues WHERE issue_key = $1", issueKey).
		Scan(&status, &severity, &priority, &createdAt)
	if err != nil {
		http.Error(w, "Issue not found", http.StatusNotFound)
		return
	}

	if status == "resolved" || status == "verified" || status == "closed" {
		resp := SLAForecast{IssueKey: issueKey, Status: "on_track", Reason: fmt.Sprintf("Issue is already %s", status)}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	slaHours := slaBaseHours[severity] * priorityMultiplier[priority]
	if slaHours == 0 {
		slaHours = 168 // fallback: 1 week
	}
	elapsedHours := time.Since(createdAt).Hours()
	remainingHours := slaHours - elapsedHours

	forecastStatus := "on_track"
	reason := fmt.Sprintf("%.0fh elapsed of %.0fh SLA budget", elapsedHours, slaHours)
	switch {
	case remainingHours < 0:
		forecastStatus = "breached"
		reason = fmt.Sprintf("SLA breached %.0fh ago (budget was %.0fh)", -remainingHours, slaHours)
	case remainingHours < slaHours*0.2:
		forecastStatus = "at_risk"
		reason = fmt.Sprintf("Only %.0fh remain of a %.0fh budget — likely to breach soon", remainingHours, slaHours)
	}

	resp := SLAForecast{
		IssueKey: issueKey, SLAHours: slaHours, ElapsedHours: elapsedHours,
		RemainingHours: remainingHours, Status: forecastStatus, Reason: reason,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// =========================================================
// 3. Blast Radius — cascade impact if this issue is resolved
// Walks the "blocks" graph transitively (depth-limited, cycle-safe).
// =========================================================

type BlastRadiusEntry struct {
	IssueKey string `json:"issue_key"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Depth    int    `json:"depth"`
}

type BlastRadiusResponse struct {
	IssueKey      string              `json:"issue_key"`
	TotalUnblocked int                `json:"total_unblocked"`
	CriticalCount int                 `json:"critical_count"`
	Cascade       []BlastRadiusEntry  `json:"cascade"`
}

// GET /issues/{id}/blast-radius
func getBlastRadiusHandler(w http.ResponseWriter, r *http.Request) {
	rootKey := r.PathValue("id")
	ctx := context.Background()

	visited := map[string]bool{rootKey: true}
	queue := []struct {
		key   string
		depth int
	}{{rootKey, 0}}

	var cascade []BlastRadiusEntry
	criticalCount := 0
	const maxDepth = 5

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.depth >= maxDepth {
			continue
		}

		// Issues that `current.key` blocks — i.e. what frees up if it's resolved.
		rows, err := dbpool.Query(ctx, `
			SELECT i.issue_key, i.title, i.severity
			FROM issue_relationships ir
			JOIN issues i ON i.issue_key = ir.related_issue_key
			WHERE ir.issue_key = $1 AND ir.relation_type = 'blocks'
		`, current.key)
		if err != nil {
			continue
		}
		for rows.Next() {
			var key, title, severity string
			if err := rows.Scan(&key, &title, &severity); err != nil {
				continue
			}
			if visited[key] {
				continue
			}
			visited[key] = true
			cascade = append(cascade, BlastRadiusEntry{IssueKey: key, Title: title, Severity: severity, Depth: current.depth + 1})
			if severity == "blocker" || severity == "critical" {
				criticalCount++
			}
			queue = append(queue, struct {
				key   string
				depth int
			}{key, current.depth + 1})
		}
		rows.Close()
	}

	if cascade == nil {
		cascade = []BlastRadiusEntry{}
	}

	resp := BlastRadiusResponse{
		IssueKey: rootKey, TotalUnblocked: len(cascade), CriticalCount: criticalCount, Cascade: cascade,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// =========================================================
// 4. Auto-Generated Bug Report Summary
// Pulls activity_stream + comments into a structured, deterministic
// summary — no AI, just aggregation of data already logged.
// =========================================================

type SummaryReport struct {
	IssueKey          string   `json:"issue_key"`
	Title             string   `json:"title"`
	CreatedAt         string   `json:"created_at"`
	CurrentStatus     string   `json:"current_status"`
	TotalAgeHours     float64  `json:"total_age_hours"`
	ResolutionHours   *float64 `json:"resolution_hours,omitempty"` // nil if not yet resolved
	ReopenCount       int      `json:"reopen_count"`
	CommentCount      int      `json:"comment_count"`
	AttachmentCount   int      `json:"attachment_count"`
	Contributors      []string `json:"contributors"` // distinct actor emails across activity + comments
	StatusTimeline    []string `json:"status_timeline"` // human-readable ordered list
}

// GET /issues/{id}/summary-report
func getSummaryReportHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	ctx := context.Background()

	var title, status string
	var createdAt time.Time
	var reopenCount, attachmentCount int
	err := dbpool.QueryRow(ctx, `
		SELECT title, status, created_at, reopen_count,
			(SELECT COUNT(*) FROM issue_attachments WHERE issue_key = $1)
		FROM issues WHERE issue_key = $1
	`, issueKey).Scan(&title, &status, &createdAt, &reopenCount, &attachmentCount)
	if err != nil {
		http.Error(w, "Issue not found", http.StatusNotFound)
		return
	}

	var commentCount int
	dbpool.QueryRow(ctx, "SELECT COUNT(*) FROM issue_comments WHERE issue_key = $1", issueKey).Scan(&commentCount)

	contributorSet := map[string]bool{}
	var statusTimeline []string
	var resolvedAt *time.Time

	rows, err := dbpool.Query(ctx, `
		SELECT u.email, a.action_type, a.old_value, a.new_value, a.created_at
		FROM activity_stream a
		LEFT JOIN users u ON a.actor_id = u.id
		WHERE a.issue_key = $1
		ORDER BY a.created_at ASC
	`, issueKey)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var email *string
			var action string
			var oldVal, newVal *string
			var ts time.Time
			if err := rows.Scan(&email, &action, &oldVal, &newVal, &ts); err != nil {
				continue
			}
			if email != nil {
				contributorSet[*email] = true
			}
			if action == "status_changed" || action == "reopened" {
				ov, nv := "", ""
				if oldVal != nil {
					ov = *oldVal
				}
				if newVal != nil {
					nv = *newVal
				}
				statusTimeline = append(statusTimeline, fmt.Sprintf("%s: %s → %s", ts.Format("Jan 2 15:04"), ov, nv))
				if nv == "resolved" {
					t := ts
					resolvedAt = &t
				}
			}
		}
	}

	var commenterRows, _ = dbpool.Query(ctx, "SELECT u.email FROM issue_comments c JOIN users u ON c.user_id = u.id WHERE c.issue_key = $1", issueKey)
	if commenterRows != nil {
		defer commenterRows.Close()
		for commenterRows.Next() {
			var email string
			if commenterRows.Scan(&email) == nil {
				contributorSet[email] = true
			}
		}
	}

	var contributors []string
	for e := range contributorSet {
		contributors = append(contributors, e)
	}
	if contributors == nil {
		contributors = []string{}
	}
	if statusTimeline == nil {
		statusTimeline = []string{}
	}

	var resolutionHours *float64
	if resolvedAt != nil {
		h := resolvedAt.Sub(createdAt).Hours()
		resolutionHours = &h
	}

	resp := SummaryReport{
		IssueKey: issueKey, Title: title, CreatedAt: createdAt.Format("Jan 2, 2006 15:04"),
		CurrentStatus: status, TotalAgeHours: time.Since(createdAt).Hours(),
		ResolutionHours: resolutionHours, ReopenCount: reopenCount, CommentCount: commentCount,
		AttachmentCount: attachmentCount, Contributors: contributors, StatusTimeline: statusTimeline,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
