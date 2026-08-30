package main

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"time"
)

// --- Lifecycle definition ---
// OPEN -> TRIAGED -> IN PROGRESS -> RESOLVED -> VERIFIED -> CLOSED
// Reopening is supported from RESOLVED, VERIFIED, or CLOSED back to OPEN.
var validTransitions = map[string][]string{
	"open":        {"triaged"},
	"triaged":     {"in_progress"},
	"in_progress": {"resolved"},
	"resolved":    {"verified", "open"}, // verify, or reopen
	"verified":    {"closed", "open"},   // close, or reopen
	"closed":      {"open"},             // reopen
}

// isValidTransition reports whether moving from `from` to `to` is allowed.
// A no-op (from == to) is always allowed so callers can PUT other fields
// without touching status.
func isValidTransition(from, to string) bool {
	if from == to {
		return true
	}
	for _, allowed := range validTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Activity is a single row of the audit trail.
type Activity struct {
	ID         int     `json:"id"`
	IssueKey   string  `json:"issue_key"`
	ActorID    *int    `json:"actor_id"`
	ActorEmail *string `json:"actor_email"`
	ActionType string  `json:"action_type"`
	OldValue   *string `json:"old_value"`
	NewValue   *string `json:"new_value"`
	CreatedAt  time.Time `json:"created_at"`
}

// logActivity writes one row to activity_stream. Safe to call with a nil
// old/new value (e.g. "issue_created" has no "old" state).
func logActivity(ctx context.Context, issueKey string, actorID int, actionType string, oldValue, newValue *string) {
	_, err := dbpool.Exec(ctx,
		`INSERT INTO activity_stream (issue_key, actor_id, action_type, old_value, new_value)
		 VALUES ($1, $2, $3, $4, $5)`,
		issueKey, actorID, actionType, oldValue, newValue)
	if err != nil {
		// Activity logging must never break the primary request — just log it.
		println("⚠️ failed to write activity_stream row:", err.Error())
	}
}

// strPtr is a tiny helper for building *string literals inline.
func strPtr(s string) *string { return &s }

// GET /issues/{id}/activity
func getActivityHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")

	rows, err := dbpool.Query(context.Background(), `
		SELECT a.id, a.issue_key, a.actor_id, u.email, a.action_type, a.old_value, a.new_value, a.created_at
		FROM activity_stream a
		LEFT JOIN users u ON a.actor_id = u.id
		WHERE a.issue_key = $1
		ORDER BY a.created_at ASC
	`, issueKey)
	if err != nil {
		http.Error(w, "Failed to fetch activity", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.IssueKey, &a.ActorID, &a.ActorEmail, &a.ActionType, &a.OldValue, &a.NewValue, &a.CreatedAt); err != nil {
			continue
		}
		activities = append(activities, a)
	}
	if activities == nil {
		activities = []Activity{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activities)
}

// PUT /issues/{id}/assign  body: {"assignee_id": 3}  (assignee_id: null to unassign)
func assignIssueHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	actorID := r.Context().Value("userID").(int)

	var req struct {
		AssigneeID *int `json:"assignee_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	// Fetch current assignee (for the activity diff) and confirm the issue exists.
	var currentAssignee *int
	err := dbpool.QueryRow(context.Background(),
		"SELECT assignee_id FROM issues WHERE issue_key = $1", issueKey).Scan(&currentAssignee)
	if err != nil {
		http.Error(w, "Issue not found", http.StatusNotFound)
		return
	}

	_, err = dbpool.Exec(context.Background(),
		"UPDATE issues SET assignee_id = $1, updated_at = NOW() WHERE issue_key = $2",
		req.AssigneeID, issueKey)
	if err != nil {
		http.Error(w, "Failed to assign issue", http.StatusInternalServerError)
		return
	}

	oldVal := emailOrUnassigned(currentAssignee)
	newVal := emailOrUnassigned(req.AssigneeID)
	logActivity(context.Background(), issueKey, actorID, "assignee_changed", &oldVal, &newVal)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// emailOrUnassigned resolves a nullable user id to a readable label for the
// activity log ("unassigned" or the user's email).
func emailOrUnassigned(userID *int) string {
	if userID == nil {
		return "unassigned"
	}
	var email string
	err := dbpool.QueryRow(context.Background(), "SELECT email FROM users WHERE id = $1", *userID).Scan(&email)
	if err != nil {
		return "unknown"
	}
	return email
}

// GET /users/list — lightweight list for populating assignee dropdowns.
// (Deliberately separate from the admin-only /admin/users, since any
// authenticated user needs this to assign issues.)
func getUsersListHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := dbpool.Query(context.Background(), "SELECT id, email FROM users WHERE is_active = TRUE ORDER BY email ASC")
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type UserOption struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	}
	var users []UserOption
	for rows.Next() {
		var u UserOption
		if err := rows.Scan(&u.ID, &u.Email); err == nil {
			users = append(users, u)
		}
	}
	if users == nil {
		users = []UserOption{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// --- @mentions ---
// Matches @email.style.mentions inside comment bodies.
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)

// logMentions scans a comment body for @email mentions and logs one
// activity_stream row per mention that matches a real user.
func logMentions(ctx context.Context, issueKey string, actorID int, body string) {
	for _, m := range mentionRegex.FindAllStringSubmatch(body, -1) {
		email := m[1]
		var exists bool
		dbpool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)", email).Scan(&exists)
		if exists {
			e := email
			logActivity(ctx, issueKey, actorID, "mentioned", nil, &e)
		}
	}
}
