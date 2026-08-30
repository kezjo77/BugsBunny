package main

import (
	"context"	
    	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"io"
	"time"
	"strings"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Global database pool
var dbpool *pgxpool.Pool
var pluginManager *PluginManager

// baseURL returns the public-facing URL this server is reachable at, used
// for building absolute links (e.g. attachment downloads). Set BASE_URL in
// the environment for Codespaces/deployment; defaults to local dev.
func baseURL() string {
	if v := os.Getenv("BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

// --- V2 DATA MODELS ---

// Secret key for signing tokens. Reads JWT_SECRET from the environment in
// production; falls back to a dev-only default so local `go run .` still
// works without extra setup.
var jwtSecret = []byte(func() string {
	if v := os.Getenv("JWT_SECRET"); v != "" {
		return v
	}
	return "super_secret_bunny_key"
}())

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

// The data we pack inside the JWT token
type Claims struct {
    UserID int    `json:"user_id"`
    Role   string `json:"role"`
    jwt.RegisteredClaims
}

type User struct {
    ID           int    `json:"id"`
    Email        string `json:"email"`
    PasswordHash string `json:"-"` // The hyphen ensures passwords NEVER leak to the Vue frontend!
    Role         string `json:"role"`
    IsActive     bool   `json:"is_active"`
}

type Project struct {
    ID          int    `json:"id"`
    ProjectKey  string `json:"project_key"`
    Name        string `json:"name"`
    Description string `json:"description"`
    IsArchived  bool   `json:"is_archived"`
}

type Comment struct {
    ID        int       `json:"id"`
    IssueKey  string    `json:"issue_key"`
    UserID    int       `json:"user_id"`
    UserEmail string    `json:"user_email"` 
    Body      string    `json:"body"`
    CreatedAt time.Time `json:"created_at"`
}

type Attachment struct {
    ID        int       `json:"id"`
    IssueKey  string    `json:"issue_key"`
    Filename  string    `json:"filename"`
    FileURL   string    `json:"file_url"`
    UserEmail *string   `json:"user_email"`
    CreatedAt time.Time `json:"created_at"`
}

// This handles both reading from the DB and receiving from Vue.
// NOTE: priority/severity/component/module/release used to be smuggled
// through a hand-built JSON string into custom_data (fmt.Sprintf'ing raw
// user input into JSON — an injection bug). They're now real, validated
// columns.
type Issue struct {
    IssueKey     string          `json:"issue_key"`
    ProjectID    int             `json:"project_id"`
    Title        string          `json:"title"`
    Body         string          `json:"body"`
    Status       string          `json:"status"`
    Priority     string          `json:"priority"`
    Severity     string          `json:"severity"`
    Component    string          `json:"component"`
    Environment  string          `json:"environment"`
    Module       string          `json:"module"`
    Release      string          `json:"release"`
    ReporterID   int             `json:"reporter_id"`
    AssigneeID   *int            `json:"assignee_id"`   // Pointer because it can be unassigned (NULL)
    AssigneeEmail *string        `json:"assignee_email,omitempty"`
    ReopenCount  int             `json:"reopen_count"`
    CustomData   json.RawMessage `json:"custom_data,omitempty"` // free-form extension bucket
    AttachmentCount int          `json:"attachment_count"`
}

func getCommentsHandler(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("id")
    
    // We JOIN with the users table to get the email address of the commenter
    query := `
        SELECT c.id, c.issue_key, c.user_id, u.email, c.body, c.created_at 
        FROM issue_comments c
        JOIN users u ON c.user_id = u.id
        WHERE c.issue_key = $1
        ORDER BY c.created_at ASC
    `
    
    rows, err := dbpool.Query(context.Background(), query, issueKey)
    if err != nil {
        http.Error(w, "Failed to fetch comments", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var comments []Comment
    for rows.Next() {
        var c Comment
        if err := rows.Scan(&c.ID, &c.IssueKey, &c.UserID, &c.UserEmail, &c.Body, &c.CreatedAt); err != nil {
            continue
        }
        comments = append(comments, c)
    }
    
    if comments == nil { comments = []Comment{} }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(comments)
}

func addCommentHandler(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("id")
    userID := r.Context().Value("userID").(int) // Extracted from JWT!

    var c Comment
    if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
        http.Error(w, "Invalid data", http.StatusBadRequest)
        return
    }

    err := dbpool.QueryRow(context.Background(),
        "INSERT INTO issue_comments (issue_key, user_id, body) VALUES ($1, $2, $3) RETURNING id, created_at",
        issueKey, userID, c.Body).Scan(&c.ID, &c.CreatedAt)
        
    if err != nil {
        http.Error(w, "Failed to add comment", http.StatusInternalServerError)
        return
    }

    preview := c.Body
    if len(preview) > 80 {
        preview = preview[:80] + "..."
    }
    logActivity(context.Background(), issueKey, userID, "comment_added", nil, &preview)
    logMentions(context.Background(), issueKey, userID, c.Body)

    w.WriteHeader(http.StatusCreated)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    var user User
	// Fetch the user from the database
	    err := dbpool.QueryRow(context.Background(), 
	    "SELECT id, password_hash, role, is_active FROM users WHERE email = $1", req.Email).
	    Scan(&user.ID, &user.PasswordHash, &user.Role, &user.IsActive)

	if err == nil && !user.IsActive {
	    http.Error(w, "Account deactivated", http.StatusForbidden)
	    return
	}

    // Compare the submitted password with the database hash
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
        http.Error(w, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    // Create the JWT Token valid for 24 hours
    expirationTime := time.Now().Add(24 * time.Hour)
    claims := &Claims{
        UserID: user.ID,
        Role:   user.Role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(expirationTime),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString(jwtSecret)
    if err != nil {
        http.Error(w, "Error generating token", http.StatusInternalServerError)
        return
    }

    // Send the token back to Vue!
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "token": tokenString,
        "role":  user.Role,
        "email": req.Email,
    })
}
func adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
    // 1. First, pass the request through the standard authMiddleware to verify the JWT
    return authMiddleware(func(w http.ResponseWriter, r *http.Request) {
        
        // 2. Extract the authenticated user's ID from the context (set by authMiddleware)
        userID := r.Context().Value("userID").(int)

        // 3. Query the database to check their role in real-time
        var role string
        err := dbpool.QueryRow(context.Background(), "SELECT role FROM users WHERE id = $1", userID).Scan(&role)
        
        // 4. Block the request if they aren't an admin
        if err != nil || role != "admin" {
            http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
            return
        }

        // 5. If they are an admin, allow the request to proceed!
        next.ServeHTTP(w, r)
    })
}

// The Bouncer: Protects our API routes
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        
        if authHeader == "" {
            http.Error(w, "Forbidden: No token provided", http.StatusUnauthorized)
            return
        }

        // Strip the "Bearer " prefix from the header
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")

        // Parse and validate the JWT signature
        claims := &Claims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return jwtSecret, nil // Use our global secret key
        })

        if err != nil || !token.Valid {
            http.Error(w, "Forbidden: Invalid or expired token", http.StatusUnauthorized)
            return
        }

       	// Attach the userID from the token directly into the HTTP Request context
	// (You will need to create a custom type for context keys in production, but a string is fine for now)
	ctx := context.WithValue(r.Context(), "userID", claims.UserID)

	// Pass the upgraded request to the handler
	next.ServeHTTP(w, r.WithContext(ctx))
    }
}


// validPriorities / validSeverities gate what can be written — priority is
// urgency, severity is impact, and they are intentionally never merged.
var validPriorities = map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
var validSeverities = map[string]bool{"minor": true, "major": true, "critical": true, "blocker": true}

// 2. Add the Create Handler
func createIssueHandler(w http.ResponseWriter, r *http.Request) {
	var issue Issue
	if err := json.NewDecoder(r.Body).Decode(&issue); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Extract the dynamic User ID from the context injected by the middleware
	userID := r.Context().Value("userID").(int)

	if issue.Priority == "" {
		issue.Priority = "medium"
	}
	if issue.Severity == "" {
		issue.Severity = "minor"
	}
	if !validPriorities[issue.Priority] {
		http.Error(w, "Invalid priority", http.StatusBadRequest)
		return
	}
	if !validSeverities[issue.Severity] {
		http.Error(w, "Invalid severity", http.StatusBadRequest)
		return
	}

	// New issues always start at OPEN — status is not settable on create.
	// Issue key is generated server-side atomically (project_key-N), not
	// trusted from the client — a client-random key can and will collide.
	ctx := context.Background()
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	var projectKey string
	var counter int
	err = tx.QueryRow(ctx,
		"UPDATE projects SET issue_counter = issue_counter + 1 WHERE id = $1 RETURNING project_key, issue_counter",
		issue.ProjectID).Scan(&projectKey, &counter)
	if err != nil {
		http.Error(w, "Invalid project", http.StatusBadRequest)
		return
	}
	issue.IssueKey = fmt.Sprintf("%s-%d", projectKey, counter)

	_, err = tx.Exec(ctx,
		`INSERT INTO issues (project_id, reporter_id, status, priority, severity, component, environment, module, release, issue_key, title, body)
		 VALUES ($1, $2, 'open', $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		issue.ProjectID, userID, issue.Priority, issue.Severity, issue.Component, issue.Environment, issue.Module, issue.Release,
		issue.IssueKey, issue.Title, issue.Body)

	if err != nil {
		fmt.Println("❌ DATABASE INSERT ERROR:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Failed to commit issue creation", http.StatusInternalServerError)
		return
	}

	logActivity(context.Background(), issue.IssueKey, userID, "issue_created", nil, strPtr(issue.Title))

	fmt.Println("✅ Successfully saved to DB:", issue.IssueKey)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "issue_key": issue.IssueKey})
}



// The new handler to fetch all issues
func getAllIssuesHandler(w http.ResponseWriter, r *http.Request) {
    search := r.URL.Query().Get("search")
    priority := r.URL.Query().Get("priority")
    severity := r.URL.Query().Get("severity")
    status := r.URL.Query().Get("status")

    query := `
    SELECT i.issue_key, i.project_id, i.title, i.body, i.status, i.priority, i.severity,
           i.component, i.environment, i.module, i.release, i.reporter_id, i.assignee_id, u.email, i.reopen_count,
           (SELECT COUNT(*) FROM issue_attachments WHERE issue_key = i.issue_key) AS attachment_count
    FROM issues i
    LEFT JOIN users u ON i.assignee_id = u.id
    WHERE 1=1
`
    var args []interface{}
    var argIndex = 1

    if search != "" {
        query += fmt.Sprintf(" AND (i.title ILIKE $%d OR i.body ILIKE $%d)", argIndex, argIndex)
        args = append(args, "%"+search+"%")
        argIndex++
    }

    if priority != "" && priority != "all" {
        query += fmt.Sprintf(" AND i.priority = $%d", argIndex)
        args = append(args, priority)
        argIndex++
    }

    if severity != "" && severity != "all" {
        query += fmt.Sprintf(" AND i.severity = $%d", argIndex)
        args = append(args, severity)
        argIndex++
    }

    if status != "" && status != "all" {
        query += fmt.Sprintf(" AND i.status = $%d", argIndex)
        args = append(args, status)
        argIndex++
    }

    query += " ORDER BY i.issue_key DESC"

    rows, err := dbpool.Query(context.Background(), query, args...)
    if err != nil {
        fmt.Println("❌ DATABASE QUERY ERROR:", err)
        http.Error(w, "Failed to fetch issues", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var issues []Issue
    for rows.Next() {
        var issue Issue
        if err := rows.Scan(&issue.IssueKey, &issue.ProjectID, &issue.Title, &issue.Body, &issue.Status, &issue.Priority,
            &issue.Severity, &issue.Component, &issue.Environment, &issue.Module, &issue.Release, &issue.ReporterID,
            &issue.AssigneeID, &issue.AssigneeEmail, &issue.ReopenCount, &issue.AttachmentCount); err != nil {
            continue
        }
        issues = append(issues, issue)
    }

    if issues == nil {
        issues = []Issue{}
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(issues)
}

// The handler to update an existing issue. Enforces the lifecycle,
// validates priority/severity, tracks reopen_count, and writes one
// activity_stream row per field that actually changed.
func updateIssueHandler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    actorID := r.Context().Value("userID").(int)

    var issue Issue
    if err := json.NewDecoder(r.Body).Decode(&issue); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Fetch current state so we can validate the transition and diff fields.
    var current Issue
    err := dbpool.QueryRow(context.Background(), `
        SELECT title, body, status, priority, severity, component, environment, module, release, reopen_count
        FROM issues WHERE issue_key = $1
    `, id).Scan(&current.Title, &current.Body, &current.Status, &current.Priority, &current.Severity,
        &current.Component, &current.Environment, &current.Module, &current.Release, &current.ReopenCount)
    if err != nil {
        http.Error(w, "Issue not found", http.StatusNotFound)
        return
    }

    if issue.Status == "" {
        issue.Status = current.Status
    }
    if !isValidTransition(current.Status, issue.Status) {
        http.Error(w, fmt.Sprintf("Invalid transition: %s -> %s", current.Status, issue.Status), http.StatusBadRequest)
        return
    }
    if issue.Priority == "" {
        issue.Priority = current.Priority
    }
    if issue.Severity == "" {
        issue.Severity = current.Severity
    }
    if !validPriorities[issue.Priority] {
        http.Error(w, "Invalid priority", http.StatusBadRequest)
        return
    }
    if !validSeverities[issue.Severity] {
        http.Error(w, "Invalid severity", http.StatusBadRequest)
        return
    }

    // Reopening = moving OUT of resolved/verified/closed back to open.
    reopenCount := current.ReopenCount
    wasClosedState := current.Status == "resolved" || current.Status == "verified" || current.Status == "closed"
    if wasClosedState && issue.Status == "open" {
        reopenCount++
    }

    _, err = dbpool.Exec(context.Background(), `
        UPDATE issues
        SET title = $1, body = $2, status = $3, priority = $4, severity = $5,
            component = $6, environment = $7, module = $8, release = $9, reopen_count = $10, updated_at = NOW()
        WHERE issue_key = $11
    `, issue.Title, issue.Body, issue.Status, issue.Priority, issue.Severity,
        issue.Component, issue.Environment, issue.Module, issue.Release, reopenCount, id)

    if err != nil {
        fmt.Println("❌ DATABASE UPDATE ERROR:", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Log exactly what changed — this is the audit trail the spec asks for.
    ctx := context.Background()
    if current.Status != issue.Status {
        action := "status_changed"
        if reopenCount > current.ReopenCount {
            action = "reopened"
        }
        logActivity(ctx, id, actorID, action, &current.Status, &issue.Status)
    }
    if current.Priority != issue.Priority {
        logActivity(ctx, id, actorID, "priority_changed", &current.Priority, &issue.Priority)
    }
    if current.Severity != issue.Severity {
        logActivity(ctx, id, actorID, "severity_changed", &current.Severity, &issue.Severity)
    }
    if current.Component != issue.Component {
        logActivity(ctx, id, actorID, "component_changed", &current.Component, &issue.Component)
    }
    if current.Environment != issue.Environment {
        logActivity(ctx, id, actorID, "environment_changed", &current.Environment, &issue.Environment)
    }
    if issue.Status == "resolved" && current.Status != "resolved" {
        logActivity(ctx, id, actorID, "resolved", nil, nil)
    }
    if issue.Status == "verified" && current.Status != "verified" {
        logActivity(ctx, id, actorID, "verified", nil, nil)
    }
    if issue.Status == "closed" && current.Status != "closed" {
        logActivity(ctx, id, actorID, "closed", nil, nil)
    }

    fmt.Println("✅ Successfully updated DB:", id)
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "success", "issue_key": id})
}
func deleteIssueHandler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    
    // 1. Fetch attachments to clean up the physical hard drive
    rows, _ := dbpool.Query(context.Background(), "SELECT file_url FROM issue_attachments WHERE issue_key = $1", id)
    for rows.Next() {
        var fileURL string
        if rows.Scan(&fileURL) == nil {
            // Extract "123_image.png" from the stored absolute file_url
            parts := strings.Split(fileURL, "/")
            diskFilename := parts[len(parts)-1]
            
            // Delete the physical file from the OS!
            os.Remove(filepath.Join("uploads", diskFilename)) 
        }
    }
    rows.Close() // Close the rows before executing the next query

    // 2. Delete the issue (PostgreSQL ON DELETE CASCADE will handle the DB rows)
    _, err := dbpool.Exec(context.Background(), "DELETE FROM issues WHERE issue_key = $1", id)
    if err != nil {
        http.Error(w, "Failed to delete issue", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func getProjectsHandler(w http.ResponseWriter, r *http.Request) {
    // Inside getProjectsHandler
	rows, err := dbpool.Query(context.Background(), "SELECT id, name, project_key, description FROM projects WHERE is_archived = FALSE")
    if err != nil {
        http.Error(w, "Failed to fetch projects", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var projects []Project
    for rows.Next() {
        var p Project
        if err := rows.Scan(&p.ID, &p.Name, &p.ProjectKey, &p.Description); err != nil {
            continue
        }
        projects = append(projects, p)
    }

    if projects == nil { projects = []Project{} }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(projects)
}

func createProjectHandler(w http.ResponseWriter, r *http.Request) {
    var p Project
    if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
        http.Error(w, "Invalid data", http.StatusBadRequest)
        return
    }

    err := dbpool.QueryRow(context.Background(),
        "INSERT INTO projects (project_key, name, description) VALUES ($1, $2, $3) RETURNING id",
        p.ProjectKey, p.Name, p.Description).Scan(&p.ID)
        
    if err != nil {
        http.Error(w, "Failed to create project", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(p)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
    // 1. Create a temporary struct to catch the exact JSON sent from Vue
    var req struct {
        Email    string `json:"email"`
        Password string `json:"password"`
        Role     string `json:"role"`
    }

    // 2. Decode the stream exactly once
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid data", http.StatusBadRequest)
        return
    }

    // 3. Hash the raw password
    hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

    // 4. Insert into the database
    var newUserID int
    err := dbpool.QueryRow(context.Background(),
        "INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3) RETURNING id",
        req.Email, string(hash), req.Role).Scan(&newUserID)
        
    if err != nil {
        fmt.Println("❌ Error creating user:", err)
        http.Error(w, "Failed to create user", http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusCreated)
}

func getAttachmentsHandler(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("id")
    
    rows, err := dbpool.Query(context.Background(), `
        SELECT a.id, a.issue_key, a.filename, a.file_url, u.email, a.created_at
        FROM issue_attachments a
        LEFT JOIN users u ON a.user_id = u.id
        WHERE a.issue_key = $1
        ORDER BY a.created_at DESC
    `, issueKey)
    if err != nil {
        http.Error(w, "Failed to fetch attachments", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var attachments []Attachment
    for rows.Next() {
        var a Attachment
        if err := rows.Scan(&a.ID, &a.IssueKey, &a.Filename, &a.FileURL, &a.UserEmail, &a.CreatedAt); err != nil {
            continue
        }
        attachments = append(attachments, a)
    }
    
    if attachments == nil { attachments = []Attachment{} }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(attachments)
}

func uploadAttachmentHandler(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("id")
    userID := r.Context().Value("userID").(int)

    // Parse the incoming multipart form (max 10 MB)
    r.ParseMultipartForm(10 << 20)

    // Retrieve the file from form data
    file, handler, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "Error retrieving file", http.StatusBadRequest)
        return
    }
    defer file.Close()

    // Create a unique filename and path
    safeFilename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), handler.Filename)
    filePath := filepath.Join("uploads", safeFilename)
    fileURL := baseURL() + "/uploads/" + safeFilename

    // Save the file physically to the disk
    dst, err := os.Create(filePath)
    if err != nil {
        http.Error(w, "Error saving file", http.StatusInternalServerError)
        return
    }
    defer dst.Close()
    io.Copy(dst, file)

    // Save the metadata to PostgreSQL
    _, err = dbpool.Exec(context.Background(), 
        "INSERT INTO issue_attachments (issue_key, filename, file_url, user_id) VALUES ($1, $2, $3, $4)", 
        issueKey, handler.Filename, fileURL, userID)

    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    logActivity(context.Background(), issueKey, userID, "attachment_added", nil, strPtr(handler.Filename))
    w.WriteHeader(http.StatusCreated)
}

func toggleUserStatusHandler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    _, err := dbpool.Exec(context.Background(), "UPDATE users SET is_active = NOT is_active WHERE id = $1", id)
    if err != nil {
        http.Error(w, "Failed to update user", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}

func resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    var req struct { Password string `json:"password"` }
    json.NewDecoder(r.Body).Decode(&req)

    hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    _, err := dbpool.Exec(context.Background(), "UPDATE users SET password_hash = $1 WHERE id = $2", string(hash), id)
    
    if err != nil {
        http.Error(w, "Failed to reset password", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}

func archiveProjectHandler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    _, err := dbpool.Exec(context.Background(), "UPDATE projects SET is_archived = TRUE WHERE id = $1", id)
    if err != nil {
        http.Error(w, "Failed to archive project", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}

func getAdminUsersHandler(w http.ResponseWriter, r *http.Request) {
    rows, err := dbpool.Query(context.Background(), "SELECT id, email, role, is_active FROM users ORDER BY id ASC")
    if err != nil {
        http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        var u User
        if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.IsActive); err == nil {
            users = append(users, u)
        }
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}

func getAdminProjectsHandler(w http.ResponseWriter, r *http.Request) {
    rows, err := dbpool.Query(context.Background(), "SELECT id, name, project_key, description, is_archived FROM projects ORDER BY id ASC")
    if err != nil {
        http.Error(w, "Failed to fetch projects", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var projects []Project
    for rows.Next() {
        var p Project
        if err := rows.Scan(&p.ID, &p.Name, &p.ProjectKey, &p.Description, &p.IsArchived); err == nil {
            projects = append(projects, p)
        }
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(projects)
}

func main() {
	// 1. Connect to PostgreSQL. Reads DATABASE_URL from the environment
	// (Render/Railway/Neon/Supabase all set this); falls back to the local
	// docker-compose instance for `go run .` on a laptop.
	databaseUrl := os.Getenv("DATABASE_URL")
	if databaseUrl == "" {
		databaseUrl = "postgres://postgres:supersecret@localhost:5432/bugsbunny"
	}

	var err error
	dbpool, err = pgxpool.New(context.Background(), databaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()
	fmt.Println("✅ Connected to Bugsbunny Database!")

	// Initialize the Plugin System
	pluginManager, err = NewPluginManager(context.Background(), "./plugins")
	if err != nil {
		log.Fatalf("Failed to initialize plugin manager: %v", err)
	}
	
	// Generate a secure hash for "admin123" and update our dummy admin
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	dbpool.Exec(context.Background(), "UPDATE users SET password_hash = $1 WHERE email = 'admin@bugsbunny.local'", string(hash))

	// 2. Setup the Go 1.22+ Router (Just one mux!)
	mux := http.NewServeMux()
	
	// Ensure the uploads directory exists on disk
	os.MkdirAll("./uploads", os.ModePerm)

	// Serve the uploaded files as static assets (unprotected so `<img>` tags work)
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// Unprotected Route
	mux.HandleFunc("POST /login", loginHandler)

	// Protected Routes (Wrapped in the Bouncer!)
	mux.HandleFunc("GET /issues", authMiddleware(getAllIssuesHandler))
	mux.HandleFunc("GET /issues/{id}", authMiddleware(getIssueHandler))
	mux.HandleFunc("POST /issues", authMiddleware(createIssueHandler))
	mux.HandleFunc("PUT /issues/{id}", authMiddleware(updateIssueHandler))
	mux.HandleFunc("DELETE /issues/{id}", authMiddleware(deleteIssueHandler))
	mux.HandleFunc("GET /projects", authMiddleware(getProjectsHandler))
	mux.HandleFunc("POST /projects", authMiddleware(createProjectHandler))
	mux.HandleFunc("POST /users", authMiddleware(createUserHandler))
	mux.HandleFunc("GET /issues/{id}/comments", authMiddleware(getCommentsHandler))
	mux.HandleFunc("POST /issues/{id}/comments", authMiddleware(addCommentHandler))
	mux.HandleFunc("GET /issues/{id}/attachments", authMiddleware(getAttachmentsHandler))
	mux.HandleFunc("POST /issues/{id}/attachments", authMiddleware(uploadAttachmentHandler))

	// P2: assignment, activity, labels, relationships, watch/follow
	mux.HandleFunc("PUT /issues/{id}/assign", authMiddleware(assignIssueHandler))
	mux.HandleFunc("GET /issues/{id}/activity", authMiddleware(getActivityHandler))
	mux.HandleFunc("GET /users/list", authMiddleware(getUsersListHandler))

	mux.HandleFunc("GET /projects/{id}/labels", authMiddleware(getProjectLabelsHandler))
	mux.HandleFunc("POST /projects/{id}/labels", authMiddleware(createLabelHandler))
	mux.HandleFunc("GET /issues/{id}/labels", authMiddleware(getIssueLabelsHandler))
	mux.HandleFunc("POST /issues/{id}/labels", authMiddleware(attachLabelHandler))
	mux.HandleFunc("DELETE /issues/{id}/labels/{labelId}", authMiddleware(detachLabelHandler))

	mux.HandleFunc("GET /issues/{id}/relationships", authMiddleware(getRelationshipsHandler))
	mux.HandleFunc("POST /issues/{id}/relationships", authMiddleware(createRelationshipHandler))
	mux.HandleFunc("DELETE /issues/{id}/relationships/{relId}", authMiddleware(deleteRelationshipHandler))

	mux.HandleFunc("GET /issues/{id}/watchers", authMiddleware(getWatchersHandler))
	mux.HandleFunc("POST /issues/{id}/watch", authMiddleware(watchIssueHandler))
	mux.HandleFunc("DELETE /issues/{id}/watch", authMiddleware(unwatchIssueHandler))

	// P2 advanced: health score, duplicate suggestions, "why is this bug stuck?"
	mux.HandleFunc("GET /issues/{id}/health-score", authMiddleware(getHealthScoreHandler))
	mux.HandleFunc("GET /issues/{id}/duplicates", authMiddleware(getDuplicateSuggestionsHandler))
	mux.HandleFunc("GET /issues/{id}/why-stuck", authMiddleware(getWhyStuckHandler))

	// P2+: Bug DNA clustering, SLA forecast, blast radius, summary report
	mux.HandleFunc("GET /projects/{id}/bug-dna", authMiddleware(getBugDNAHandler))
	mux.HandleFunc("GET /issues/{id}/sla-forecast", authMiddleware(getSLAForecastHandler))
	mux.HandleFunc("GET /issues/{id}/blast-radius", authMiddleware(getBlastRadiusHandler))
	mux.HandleFunc("GET /issues/{id}/summary-report", authMiddleware(getSummaryReportHandler))

	mux.HandleFunc("PUT /admin/users/{id}/toggle", adminMiddleware(toggleUserStatusHandler))
	mux.HandleFunc("PUT /admin/users/{id}/password", adminMiddleware(resetPasswordHandler))
	mux.HandleFunc("PUT /admin/projects/{id}/archive", adminMiddleware(archiveProjectHandler))
	mux.HandleFunc("GET /admin/users", adminMiddleware(getAdminUsersHandler))
	mux.HandleFunc("GET /admin/projects", adminMiddleware(getAdminProjectsHandler))

	// 3. The Global CORS Wrapper
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// 4. Start the Server. Reads PORT from the environment (Render/Railway
	// assign this dynamically); defaults to 8080 for local dev.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("🚀 Bugsbunny Engine starting on " + baseURL() + " (port " + port + ")")
	log.Fatal(http.ListenAndServe(":"+port, corsMiddleware(mux)))
}

func getIssueHandler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id") // Extract the bug ID from the URL

    var issue Issue
    err := dbpool.QueryRow(context.Background(), `
        SELECT i.issue_key, i.project_id, i.title, i.body, i.status, i.priority, i.severity,
               i.component, i.environment, i.module, i.release, i.reporter_id, i.assignee_id, u.email, i.reopen_count
        FROM issues i
        LEFT JOIN users u ON i.assignee_id = u.id
        WHERE i.issue_key = $1
    `, id).Scan(&issue.IssueKey, &issue.ProjectID, &issue.Title, &issue.Body, &issue.Status, &issue.Priority,
        &issue.Severity, &issue.Component, &issue.Environment, &issue.Module, &issue.Release, &issue.ReporterID,
        &issue.AssigneeID, &issue.AssigneeEmail, &issue.ReopenCount)

    if err != nil {
        http.Error(w, "Issue not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(issue)
}

