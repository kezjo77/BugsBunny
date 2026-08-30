package main

import (
	"context"
	"encoding/json"
	"net/http"
)

type Label struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ProjectID int    `json:"project_id"`
}

// GET /projects/{id}/labels — all labels available for a project
func getProjectLabelsHandler(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	rows, err := dbpool.Query(context.Background(),
		"SELECT id, name, project_id FROM labels WHERE project_id = $1 ORDER BY name ASC", projectID)
	if err != nil {
		http.Error(w, "Failed to fetch labels", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.Name, &l.ProjectID); err == nil {
			labels = append(labels, l)
		}
	}
	if labels == nil {
		labels = []Label{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(labels)
}

// POST /projects/{id}/labels  body: {"name": "regression"}
// Creates the label if it doesn't already exist for the project (idempotent).
func createLabelHandler(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	var l Label
	err := dbpool.QueryRow(context.Background(), `
		INSERT INTO labels (name, project_id) VALUES ($1, $2)
		ON CONFLICT (name, project_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name, project_id
	`, req.Name, projectID).Scan(&l.ID, &l.Name, &l.ProjectID)
	if err != nil {
		http.Error(w, "Failed to create label", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(l)
}

// GET /issues/{id}/labels
func getIssueLabelsHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")

	rows, err := dbpool.Query(context.Background(), `
		SELECT l.id, l.name, l.project_id
		FROM issue_labels il
		JOIN labels l ON il.label_id = l.id
		WHERE il.issue_key = $1
		ORDER BY l.name ASC
	`, issueKey)
	if err != nil {
		http.Error(w, "Failed to fetch issue labels", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.Name, &l.ProjectID); err == nil {
			labels = append(labels, l)
		}
	}
	if labels == nil {
		labels = []Label{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(labels)
}

// POST /issues/{id}/labels  body: {"label_id": 4}
func attachLabelHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	actorID := r.Context().Value("userID").(int)

	var req struct {
		LabelID int `json:"label_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	_, err := dbpool.Exec(context.Background(),
		"INSERT INTO issue_labels (issue_key, label_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		issueKey, req.LabelID)
	if err != nil {
		http.Error(w, "Failed to attach label", http.StatusInternalServerError)
		return
	}

	var labelName string
	dbpool.QueryRow(context.Background(), "SELECT name FROM labels WHERE id = $1", req.LabelID).Scan(&labelName)
	logActivity(context.Background(), issueKey, actorID, "label_added", nil, &labelName)

	w.WriteHeader(http.StatusCreated)
}

// DELETE /issues/{id}/labels/{labelId}
func detachLabelHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	labelID := r.PathValue("labelId")
	actorID := r.Context().Value("userID").(int)

	var labelName string
	dbpool.QueryRow(context.Background(), "SELECT name FROM labels WHERE id = $1", labelID).Scan(&labelName)

	_, err := dbpool.Exec(context.Background(),
		"DELETE FROM issue_labels WHERE issue_key = $1 AND label_id = $2", issueKey, labelID)
	if err != nil {
		http.Error(w, "Failed to remove label", http.StatusInternalServerError)
		return
	}

	logActivity(context.Background(), issueKey, actorID, "label_removed", &labelName, nil)
	w.WriteHeader(http.StatusNoContent)
}
