package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type IssueRelationship struct {
	ID               int    `json:"id"`
	IssueKey         string `json:"issue_key"`
	RelatedIssueKey  string `json:"related_issue_key"`
	RelatedTitle     string `json:"related_title,omitempty"`
	RelationType     string `json:"relation_type"`
	CreatedAt        time.Time `json:"created_at"`
}

var validRelationTypes = map[string]bool{
	"related_to":   true,
	"duplicate_of": true,
	"blocks":       true,
	"blocked_by":   true,
}

// inverseRelation returns the mirror relation type recorded on the other
// issue, so "A blocks B" also shows up as "B blocked_by A".
func inverseRelation(rel string) string {
	switch rel {
	case "blocks":
		return "blocked_by"
	case "blocked_by":
		return "blocks"
	default:
		return rel // related_to and duplicate_of are symmetric enough for display
	}
}

// GET /issues/{id}/relationships
func getRelationshipsHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")

	rows, err := dbpool.Query(context.Background(), `
		SELECT ir.id, ir.issue_key, ir.related_issue_key, i.title, ir.relation_type, ir.created_at
		FROM issue_relationships ir
		JOIN issues i ON i.issue_key = ir.related_issue_key
		WHERE ir.issue_key = $1
		ORDER BY ir.created_at ASC
	`, issueKey)
	if err != nil {
		http.Error(w, "Failed to fetch relationships", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var rels []IssueRelationship
	for rows.Next() {
		var rel IssueRelationship
		if err := rows.Scan(&rel.ID, &rel.IssueKey, &rel.RelatedIssueKey, &rel.RelatedTitle, &rel.RelationType, &rel.CreatedAt); err == nil {
			rels = append(rels, rel)
		}
	}
	if rels == nil {
		rels = []IssueRelationship{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rels)
}

// POST /issues/{id}/relationships  body: {"related_issue_key": "BUNNY-2", "relation_type": "blocks"}
func createRelationshipHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	actorID := r.Context().Value("userID").(int)

	var req struct {
		RelatedIssueKey string `json:"related_issue_key"`
		RelationType    string `json:"relation_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	if !validRelationTypes[req.RelationType] {
		http.Error(w, "Invalid relation_type", http.StatusBadRequest)
		return
	}
	if req.RelatedIssueKey == issueKey {
		http.Error(w, "An issue cannot relate to itself", http.StatusBadRequest)
		return
	}

	// Confirm the target issue actually exists so we don't create a dangling link.
	var exists bool
	dbpool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM issues WHERE issue_key = $1)", req.RelatedIssueKey).Scan(&exists)
	if !exists {
		http.Error(w, "Related issue not found", http.StatusNotFound)
		return
	}

	ctx := context.Background()
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	// Forward link. ON CONFLICT DO NOTHING enforces "no duplicate entries".
	_, err = tx.Exec(ctx, `
		INSERT INTO issue_relationships (issue_key, related_issue_key, relation_type)
		VALUES ($1, $2, $3)
		ON CONFLICT (issue_key, related_issue_key, relation_type) DO NOTHING
	`, issueKey, req.RelatedIssueKey, req.RelationType)
	if err != nil {
		http.Error(w, "Failed to create relationship", http.StatusInternalServerError)
		return
	}

	// Mirror link on the other issue so both sides see it.
	inverse := inverseRelation(req.RelationType)
	_, err = tx.Exec(ctx, `
		INSERT INTO issue_relationships (issue_key, related_issue_key, relation_type)
		VALUES ($1, $2, $3)
		ON CONFLICT (issue_key, related_issue_key, relation_type) DO NOTHING
	`, req.RelatedIssueKey, issueKey, inverse)
	if err != nil {
		http.Error(w, "Failed to create inverse relationship", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Failed to commit relationship", http.StatusInternalServerError)
		return
	}

	desc := fmt.Sprintf("%s %s", req.RelationType, req.RelatedIssueKey)
	logActivity(ctx, issueKey, actorID, "relationship_added", nil, &desc)

	w.WriteHeader(http.StatusCreated)
}

// DELETE /issues/{id}/relationships/{relId}
func deleteRelationshipHandler(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("id")
	relID := r.PathValue("relId")
	actorID := r.Context().Value("userID").(int)

	// Look up the pair before deleting so we can remove the mirror too.
	var relatedKey, relType string
	err := dbpool.QueryRow(context.Background(),
		"SELECT related_issue_key, relation_type FROM issue_relationships WHERE id = $1 AND issue_key = $2",
		relID, issueKey).Scan(&relatedKey, &relType)
	if err != nil {
		http.Error(w, "Relationship not found", http.StatusNotFound)
		return
	}

	ctx := context.Background()
	dbpool.Exec(ctx, "DELETE FROM issue_relationships WHERE id = $1", relID)
	dbpool.Exec(ctx, "DELETE FROM issue_relationships WHERE issue_key = $1 AND related_issue_key = $2 AND relation_type = $3",
		relatedKey, issueKey, inverseRelation(relType))

	desc := fmt.Sprintf("%s %s", relType, relatedKey)
	logActivity(ctx, issueKey, actorID, "relationship_removed", &desc, nil)

	w.WriteHeader(http.StatusNoContent)
}
