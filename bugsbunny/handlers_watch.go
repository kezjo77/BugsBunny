package main

import (
	"context"
	"encoding/json"
	"net/http"
)

// GET /issues/{id}/watchers
func getWatchersHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")

	rows, err := dbpool.Query(context.Background(), `
		SELECT u.id, u.email FROM issue_watchers iw
		JOIN users u ON iw.user_id = u.id
		WHERE iw.issue_key = $1
		ORDER BY u.email ASC
	`, issueKey)
	if err != nil {
		http.Error(w, "Failed to fetch watchers", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type WatcherOption struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	}
	var watchers []WatcherOption
	for rows.Next() {
		var wt WatcherOption
		if err := rows.Scan(&wt.ID, &wt.Email); err == nil {
			watchers = append(watchers, wt)
		}
	}
	if watchers == nil {
		watchers = []WatcherOption{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(watchers)
}

// POST /issues/{id}/watch — the logged-in user starts watching this issue
func watchIssueHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	userID := r.Context().Value("userID").(int)

	_, err := dbpool.Exec(context.Background(),
		"INSERT INTO issue_watchers (issue_key, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		issueKey, userID)
	if err != nil {
		http.Error(w, "Failed to watch issue", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// DELETE /issues/{id}/watch — the logged-in user stops watching this issue
func unwatchIssueHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	userID := r.Context().Value("userID").(int)

	_, err := dbpool.Exec(context.Background(),
		"DELETE FROM issue_watchers WHERE issue_key = $1 AND user_id = $2", issueKey, userID)
	if err != nil {
		http.Error(w, "Failed to unwatch issue", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
