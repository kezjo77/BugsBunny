<!-- src/views/QueueView.vue -->
<script setup>
import { ref, onMounted, watch } from 'vue'
import { API_URL } from '@/config'

const issues = ref([])
const loading = ref(true)
const users = ref([])

// P2: Bug DNA (root cause clustering)
const bugDNA = ref(null)
const bugDNALoading = ref(false)

// Filter State
const searchQuery = ref('')
const priorityFilter = ref('all')
const severityFilter = ref('all') // P2: severity filter, missing from P3
const statusFilter = ref('all')
const assigneeFilter = ref('all')
let debounceTimeout = null

// P2 backend only exposes GET /users/list (not GET /users) for populating
// the assignee dropdown, so we adapt to that endpoint instead of changing the backend.
const fetchUsers = async () => {
  const token = localStorage.getItem('bunny_token')
  const res = await fetch(`${API_URL}/users/list`, {
    headers: { 'Authorization': `Bearer ${token}` }
  })
  if (res.ok) {
    users.value = await res.json()
  } else {
    console.error('Failed to fetch users')
  }
}

// P2's GET /issues supports search/priority/severity/status query params server-side,
// but has no "assignee" param — so assignee filtering is applied client-side below.
const fetchAllIssues = async () => {
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (searchQuery.value) params.append('search', searchQuery.value)
    if (priorityFilter.value && priorityFilter.value !== 'all') params.append('priority', priorityFilter.value)
    if (severityFilter.value && severityFilter.value !== 'all') params.append('severity', severityFilter.value)
    if (statusFilter.value && statusFilter.value !== 'all') params.append('status', statusFilter.value)

    const token = localStorage.getItem('bunny_token')
    const queryString = params.toString()
    const url = queryString ? `${API_URL}/issues?${queryString}` : `${API_URL}/issues`

    const response = await fetch(url, {
      headers: { 'Authorization': `Bearer ${token}` }
    })

    if (!response.ok) {
      const errText = await response.text()
      throw new Error(errText)
    }

    let result = await response.json()

    // Client-side assignee filter (P2 API doesn't support this server-side).
    if (assigneeFilter.value && assigneeFilter.value !== 'all') {
      if (assigneeFilter.value === 'unassigned') {
        result = result.filter(issue => !issue.assignee_id)
      } else {
        const assigneeId = parseInt(assigneeFilter.value)
        result = result.filter(issue => issue.assignee_id === assigneeId)
      }
    }

    issues.value = result
  } catch (error) {
    console.error('Failed to fetch queue:', error)
  } finally {
    loading.value = false
  }
}

// P2: Bug DNA (root-cause clustering) panel
const loadBugDNA = async () => {
  bugDNALoading.value = true
  try {
    const token = localStorage.getItem('bunny_token')
    const projRes = await fetch(`${API_URL}/projects`, { headers: { 'Authorization': `Bearer ${token}` } })
    const projects = await projRes.json()
    if (projects.length === 0) { bugDNALoading.value = false; return }
    const res = await fetch(`${API_URL}/projects/${projects[0].id}/bug-dna`, { headers: { 'Authorization': `Bearer ${token}` } })
    if (res.ok) bugDNA.value = await res.json()
  } catch (e) {
    console.error('Failed to load Bug DNA', e)
  }
  bugDNALoading.value = false
}

onMounted(() => {
  fetchUsers()
  fetchAllIssues()
})

// Watch ALL dropdowns and trigger an instant fetch when any of them change
watch([priorityFilter, severityFilter, statusFilter, assigneeFilter], () => {
  fetchAllIssues()
})

watch(searchQuery, () => {
  clearTimeout(debounceTimeout)
  debounceTimeout = setTimeout(() => { fetchAllIssues() }, 300)
})
</script>

<template>
  <main class="page-container">
    <!-- Page Header -->
    <div class="page-header">
      <div class="header-content">
        <h1 class="page-title">Issue Queue</h1>
        <p class="page-subtitle">Browse and manage issues across the project</p>
      </div>
    </div>

    <!-- P2: Bug DNA (root cause clustering) -->
    <div class="bug-dna-section">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="bugDNALoading" @click="loadBugDNA">
        {{ bugDNA ? 'Refresh Bug DNA' : 'Show Bug DNA (Root Cause Clusters)' }}
      </button>
      <div v-if="bugDNA" class="bug-dna-panel">
        <div v-if="bugDNA.length === 0" class="empty-note">No open issues to cluster.</div>
        <div v-for="c in bugDNA" :key="c.component" class="dna-cluster">
          <div class="dna-header">
            <strong>{{ c.component }}</strong>
            <span class="dna-count">{{ c.issue_count }} open</span>
            <span v-if="c.critical_count > 0" class="dna-critical">{{ c.critical_count }} critical/blocker</span>
          </div>
          <div v-if="c.top_keywords && c.top_keywords.length" class="dna-keywords">
            Recurring keywords: {{ c.top_keywords.join(', ') }}
          </div>
        </div>
      </div>
    </div>

    <!-- Filter Panel -->
    <div class="filter-panel">
      <div class="filter-group">
        <label for="search-input" class="filter-label">Search</label>
        <div class="search-input-wrapper">
          <svg class="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8"></circle>
            <path d="m21 21-4.35-4.35"></path>
          </svg>
          <input
            id="search-input"
            v-model="searchQuery"
            type="text"
            class="filter-input search-input"
            placeholder="Search titles & descriptions..."
          />
        </div>
      </div>

      <div class="filter-group">
        <label for="status-filter" class="filter-label">Status</label>
        <select id="status-filter" v-model="statusFilter" class="filter-select">
          <option value="all">All Statuses</option>
          <option value="open">Open</option>
          <option value="triaged">Triaged</option>
          <option value="in_progress">In Progress</option>
          <option value="resolved">Resolved</option>
          <option value="verified">Verified</option>
          <option value="closed">Closed</option>
        </select>
      </div>

      <div class="filter-group">
        <label for="priority-filter" class="filter-label">Priority</label>
        <select id="priority-filter" v-model="priorityFilter" class="filter-select">
          <option value="all">All Priorities</option>
          <option value="critical">Critical</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </select>
      </div>

      <!-- P2: severity filter, missing from P3 -->
      <div class="filter-group">
        <label for="severity-filter" class="filter-label">Severity</label>
        <select id="severity-filter" v-model="severityFilter" class="filter-select">
          <option value="all">All Severities</option>
          <option value="blocker">Blocker</option>
          <option value="critical">Critical</option>
          <option value="major">Major</option>
          <option value="minor">Minor</option>
        </select>
      </div>

      <div class="filter-group">
        <label for="assignee-filter" class="filter-label">Assignee</label>
        <select id="assignee-filter" v-model="assigneeFilter" class="filter-select">
          <option value="all">All Assignees</option>
          <option value="unassigned">Unassigned</option>
          <option v-for="user in users" :key="user.id" :value="user.id">
            {{ user.email.split('@')[0] }}
          </option>
        </select>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-container">
      <div class="spinner"></div>
      <p class="loading-text">Loading queue...</p>
    </div>

    <!-- Empty State -->
    <div v-else-if="issues.length === 0" class="empty-state">
      <div class="empty-icon">📋</div>
      <h3 class="empty-title">No issues found</h3>
      <p class="empty-message">No issues match your current filters. Try adjusting your search criteria.</p>
    </div>

    <!-- Issues Table -->
    <div v-else class="table-wrapper">
      <table class="queue-table">
        <thead>
          <tr>
            <th class="col-id">ID</th>
            <th class="col-title">Title</th>
            <th class="col-priority">Priority</th>
            <th class="col-severity">Severity</th>
            <th class="col-status">Status</th>
            <th class="col-assignee">Assignee</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="issue in issues" :key="issue.issue_key" class="issue-row">
            <td class="col-id">
              <router-link :to="`/issue/${issue.issue_key}`" class="issue-id-link">
                {{ issue.issue_key }}
              </router-link>
              <span v-if="issue.attachment_count > 0" class="attachment-badge" title="Has Attachments">
                📎 {{ issue.attachment_count }}
              </span>
            </td>

            <td class="col-title">
              <router-link :to="`/issue/${issue.issue_key}`" class="issue-title-link">
                {{ issue.title }}
              </router-link>
            </td>

            <td class="col-priority">
              <span :class="['badge', 'badge-priority', 'priority-' + (issue.priority || 'none')]">
                {{ (issue.priority || 'none').charAt(0).toUpperCase() + (issue.priority || 'none').slice(1) }}
              </span>
            </td>

            <!-- P2: severity column, missing from P3 -->
            <td class="col-severity">
              <span :class="['badge', 'badge-severity', 'severity-' + (issue.severity || 'minor')]">
                {{ (issue.severity || 'minor').charAt(0).toUpperCase() + (issue.severity || 'minor').slice(1) }}
              </span>
            </td>

            <td class="col-status">
              <span :class="['badge', 'badge-status', 'status-' + issue.status]">
                {{ (issue.status || 'open').replace('_', ' ').charAt(0).toUpperCase() + (issue.status || 'open').replace('_', ' ').slice(1) }}
              </span>
            </td>

            <td class="col-assignee">
              <span class="assignee-text">
                {{ issue.assignee_email ? issue.assignee_email.split('@')[0] : 'Unassigned' }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </main>
</template>

<style scoped>
/* Root Variables and Theming */
.page-container {
  --bg-primary: #ffffff;
  --bg-secondary: #f8f9fa;
  --bg-tertiary: #f3f4f6;
  --text-primary: #1f2937;
  --text-secondary: #6b7280;
  --text-tertiary: #9ca3af;
  --border-color: #e5e7eb;
  --border-light: #f3f4f6;
  --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
  --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.07), 0 2px 4px -1px rgba(0, 0, 0, 0.05);
  --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
  --color-blue: #3b82f6;
  --color-blue-light: #eff6ff;
  --color-red: #ef4444;
  --color-red-light: #fee2e2;
  --color-orange: #f97316;
  --color-orange-light: #ffedd5;
  --color-green: #10b981;
  --color-green-light: #dcfce7;
  --color-indigo: #6366f1;
  --color-indigo-light: #e0e7ff;
  --color-gray-light: #f9fafb;
}

:global(html[data-theme="dark"]) .page-container {
  --bg-primary: #1f2937;
  --bg-secondary: #111827;
  --bg-tertiary: #0f1419;
  --text-primary: #f3f4f6;
  --text-secondary: #d1d5db;
  --text-tertiary: #9ca3af;
  --border-color: #374151;
  --border-light: #1f2937;
  --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.3);
  --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.3), 0 2px 4px -1px rgba(0, 0, 0, 0.2);
  --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.4), 0 4px 6px -2px rgba(0, 0, 0, 0.3);
  --color-blue-light: #1e3a8a;
  --color-red-light: #7f1d1d;
  --color-orange-light: #7c2d12;
  --color-green-light: #064e3b;
  --color-indigo-light: #3730a3;
  --color-gray-light: #111827;
}

* {
  box-sizing: border-box;
}

/* Page Container */
.page-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem 1rem;
  background: var(--bg-primary);
  color: var(--text-primary);
}

/* Page Header */
.page-header {
  margin-bottom: 2rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid var(--border-color);
}

.header-content {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.page-title {
  margin: 0;
  font-size: 2rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--text-primary);
}

.page-subtitle {
  margin: 0;
  font-size: 0.95rem;
  color: var(--text-secondary);
  font-weight: 400;
}

/* P2: Bug DNA panel */
.bug-dna-section {
  margin-bottom: 1.5rem;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 0.5rem;
  font-family: inherit;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s ease, opacity 0.2s ease;
}

.btn-secondary {
  background: var(--bg-secondary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

.btn-secondary:hover:not(:disabled) {
  background: var(--bg-tertiary);
}

.btn-sm {
  padding: 0.5rem 0.9rem;
  font-size: 0.85rem;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.bug-dna-panel {
  margin-top: 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
}

.dna-cluster {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  padding: 0.75rem 0.9rem;
}

.dna-header {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  color: var(--text-primary);
}

.dna-count {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  padding: 0.125rem 0.5rem;
  border-radius: 0.625rem;
  font-size: 0.8em;
}

.dna-critical {
  background: var(--color-red-light);
  color: #991b1b;
  padding: 0.125rem 0.5rem;
  border-radius: 0.625rem;
  font-size: 0.8em;
  font-weight: bold;
}

[data-theme="dark"] .dna-critical {
  color: #fca5a5;
}

.dna-keywords {
  color: var(--text-secondary);
  font-size: 0.85em;
  margin-top: 0.25rem;
}

.empty-note {
  color: var(--text-tertiary);
  font-style: italic;
  font-size: 0.9em;
}

/* Filter Panel */
.filter-panel {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
  padding: 1.5rem;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 0.75rem;
  margin-bottom: 2rem;
  box-shadow: var(--shadow-sm);
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.filter-label {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-primary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.filter-input,
.filter-select {
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 0.9375rem;
  font-family: inherit;
  transition: all 0.2s ease;
}

.filter-input:focus,
.filter-select:focus {
  outline: none;
  border-color: var(--color-blue);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.filter-input::placeholder {
  color: var(--text-tertiary);
}

.search-input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 0.75rem;
  width: 1.125rem;
  height: 1.125rem;
  color: var(--text-tertiary);
  pointer-events: none;
}

.search-input {
  padding-left: 2.25rem;
  width: 100%;
}

/* Table Wrapper */
.table-wrapper {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 0.75rem;
  overflow: hidden;
  box-shadow: var(--shadow-md);
}

/* Table */
.queue-table {
  width: 100%;
  border-collapse: collapse;
  background: var(--bg-primary);
}

.queue-table thead {
  background: var(--bg-secondary);
  border-bottom: 2px solid var(--border-color);
}

.queue-table th {
  padding: 1rem 0.75rem;
  text-align: left;
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.queue-table td {
  padding: 0.875rem 0.75rem;
  border-bottom: 1px solid var(--border-color);
  color: var(--text-primary);
  vertical-align: middle;
}

.queue-table tbody tr {
  transition: background-color 0.15s ease;
}

.queue-table tbody tr:hover {
  background-color: var(--bg-secondary);
}

.queue-table tbody tr:last-child td {
  border-bottom: none;
}

/* Column Specific Widths */
.col-id { width: 100px; }
.col-priority { width: 100px; text-align: center; }
.col-severity { width: 100px; text-align: center; }
.col-status { width: 140px; text-align: center; }
.col-assignee { width: 130px; text-align: center; }

/* Issue Links */
.issue-id-link,
.issue-title-link {
  color: var(--color-blue);
  text-decoration: none;
  font-weight: 500;
  transition: color 0.2s ease;
}

.issue-id-link:hover,
.issue-title-link:hover {
  color: var(--color-blue);
  text-decoration: underline;
  opacity: 0.8;
}

.issue-id-link {
  font-weight: 600;
  font-size: 0.9375rem;
}

.issue-title-link {
  font-size: 0.95rem;
  word-break: break-word;
}

.col-title {
  max-width: 320px;
  overflow: hidden;
}

.attachment-badge {
  display: inline-flex;
  align-items: center;
  margin-left: 0.5rem;
  font-size: 0.75em;
  color: var(--text-secondary);
  background: var(--bg-tertiary);
  padding: 0.125rem 0.4rem;
  border-radius: 0.625rem;
  border: 1px solid var(--border-color);
}

/* Badges */
.badge {
  display: inline-block;
  padding: 0.375rem 0.625rem;
  border-radius: 0.375rem;
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: capitalize;
  white-space: nowrap;
}

/* Status Badges */
.badge-status {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}

.badge-status.status-open,
.badge-status.status-rejected {
  background: var(--color-red-light);
  color: #991b1b;
}

[data-theme="dark"] .badge-status.status-open,
[data-theme="dark"] .badge-status.status-rejected {
  color: #fca5a5;
}

.badge-status.status-triaged {
  background: #fef9c3;
  color: #854d0e;
}

[data-theme="dark"] .badge-status.status-triaged {
  color: #fde68a;
}

.badge-status.status-in_progress,
.badge-status.status-accepted {
  background: var(--color-orange-light);
  color: #92400e;
}

[data-theme="dark"] .badge-status.status-in_progress,
[data-theme="dark"] .badge-status.status-accepted {
  color: #fed7aa;
}

.badge-status.status-resolved,
.badge-status.status-closed {
  background: var(--color-green-light);
  color: #166534;
}

[data-theme="dark"] .badge-status.status-resolved,
[data-theme="dark"] .badge-status.status-closed {
  color: #86efac;
}

.badge-status.status-verified {
  background: var(--color-green-light);
  color: #065f46;
}

[data-theme="dark"] .badge-status.status-verified {
  color: #6ee7b7;
}

.badge-status.status-on_hold,
.badge-status.status-deferred {
  background: var(--color-indigo-light);
  color: #4338ca;
}

[data-theme="dark"] .badge-status.status-on_hold,
[data-theme="dark"] .badge-status.status-deferred {
  color: #c4b5fd;
}

/* Priority Badges */
.badge-priority {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}

.badge-priority.priority-critical {
  background: var(--color-red-light);
  color: #991b1b;
}

[data-theme="dark"] .badge-priority.priority-critical {
  color: #fca5a5;
}

.badge-priority.priority-high {
  background: var(--color-orange-light);
  color: #92400e;
}

[data-theme="dark"] .badge-priority.priority-high {
  color: #fed7aa;
}

.badge-priority.priority-medium {
  background: var(--color-indigo-light);
  color: #4338ca;
}

[data-theme="dark"] .badge-priority.priority-medium {
  color: #c4b5fd;
}

.badge-priority.priority-low {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}

[data-theme="dark"] .badge-priority.priority-low {
  background: #374151;
  color: #d1d5db;
}

.badge-priority.priority-none {
  background: var(--bg-tertiary);
  color: var(--text-tertiary);
}

/* P2: Severity Badges */
.badge-severity {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}

.badge-severity.severity-blocker,
.badge-severity.severity-critical {
  background: var(--color-red-light);
  color: #991b1b;
}

[data-theme="dark"] .badge-severity.severity-blocker,
[data-theme="dark"] .badge-severity.severity-critical {
  color: #fca5a5;
}

.badge-severity.severity-major {
  background: var(--color-orange-light);
  color: #9a3412;
}

[data-theme="dark"] .badge-severity.severity-major {
  color: #fed7aa;
}

.badge-severity.severity-minor {
  background: var(--color-green-light);
  color: #166534;
}

[data-theme="dark"] .badge-severity.severity-minor {
  color: #86efac;
}

/* Assignee */
.assignee-text {
  font-size: 0.9375rem;
  color: var(--text-secondary);
  text-transform: capitalize;
}

/* Loading State */
.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem 1rem;
  min-height: 300px;
}

.spinner {
  width: 2.5rem;
  height: 2.5rem;
  border: 3px solid var(--border-color);
  border-top-color: var(--color-blue);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin-bottom: 1rem;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.loading-text {
  font-size: 0.9375rem;
  color: var(--text-secondary);
  margin: 0;
}

/* Empty State */
.empty-state {
  padding: 3rem 1.5rem;
  text-align: center;
  background: var(--bg-secondary);
  border: 1px dashed var(--border-color);
  border-radius: 0.75rem;
  min-height: 250px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.empty-icon {
  font-size: 2.5rem;
  margin-bottom: 1rem;
  opacity: 0.7;
}

.empty-title {
  margin: 0 0 0.5rem 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text-primary);
}

.empty-message {
  margin: 0;
  font-size: 0.9375rem;
  color: var(--text-secondary);
  max-width: 400px;
}

/* Responsive Design */
@media (max-width: 768px) {
  .page-container {
    padding: 1.5rem 1rem;
  }

  .page-title {
    font-size: 1.5rem;
  }

  .filter-panel {
    grid-template-columns: 1fr;
    padding: 1rem;
    gap: 0.75rem;
  }

  .queue-table th {
    padding: 0.75rem 0.5rem;
    font-size: 0.75rem;
  }

  .queue-table td {
    padding: 0.75rem 0.5rem;
    font-size: 0.875rem;
  }

  .col-id,
  .col-priority,
  .col-severity,
  .col-status,
  .col-assignee {
    width: auto;
  }

  .col-title {
    max-width: 200px;
  }

  .badge {
    font-size: 0.7rem;
    padding: 0.25rem 0.5rem;
  }
}

@media (max-width: 640px) {
  .page-container {
    padding: 1rem;
  }

  .page-header {
    margin-bottom: 1.5rem;
    padding-bottom: 1rem;
  }

  .page-title {
    font-size: 1.35rem;
  }

  .page-subtitle {
    font-size: 0.875rem;
  }

  .filter-panel {
    margin-bottom: 1.5rem;
  }

  .table-wrapper {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }

  .queue-table {
    min-width: 700px;
  }

  .col-title {
    max-width: 150px;
  }

  .issue-id-link,
  .issue-title-link {
    font-size: 0.875rem;
  }
}
</style>
