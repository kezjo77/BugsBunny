<!-- src/views/ProfileView.vue -->
<!--
  P2 note: the backend has no GET /profile endpoint. This page reconstructs
  the same "assigned issues" profile view purely from existing P2 endpoints
  (GET /issues, GET /users/list) — no backend changes required.
-->
<script setup>
import { ref, computed, onMounted } from 'vue'
import { API_URL } from '@/config'

const loading = ref(true)
const loadError = ref('')
const allIssues = ref([])
const currentUserEmail = localStorage.getItem('bunny_email') || ''

const authHeaders = () => ({ 'Authorization': `Bearer ${localStorage.getItem('bunny_token')}` })

const fetchProfileData = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const response = await fetch(`${API_URL}/issues`, { headers: authHeaders() })
    if (!response.ok) throw new Error('Unable to load profile data.')
    allIssues.value = await response.json()
  } catch (e) {
    console.error('Failed to load profile', e)
    loadError.value = e.message || 'Unable to load profile data.'
  } finally {
    loading.value = false
  }
}

// Issues assigned to the logged-in user, derived client-side from /issues.
const myIssues = computed(() =>
  (allIssues.value || []).filter(issue => issue.assignee_email === currentUserEmail)
)

const resolvedStatuses = new Set(['resolved', 'closed'])
const resolvedCount = computed(() =>
  myIssues.value.filter(issue => resolvedStatuses.has(issue.status)).length
)

onMounted(fetchProfileData)
</script>

<template>
  <main class="page-container">
    <!-- Loading State -->
    <div v-if="loading" class="loading-container">
      <div class="spinner"></div>
      <p class="loading-text">Loading profile...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="loadError" class="empty-state">
      <div class="empty-icon">⚠️</div>
      <p class="empty-message">{{ loadError }}</p>
    </div>

    <!-- Profile Content -->
    <div v-else class="profile-content">
      <!-- Profile Header -->
      <div class="profile-header">
        <div class="profile-avatar">👤</div>
        <div class="profile-info">
          <h1 class="profile-email">{{ currentUserEmail }}</h1>
          <p class="profile-role">Bugsbunny User</p>
        </div>
      </div>

      <!-- Stats Grid -->
      <div class="stats-grid">
        <div class="stat-card">
          <h3 class="stat-label">Total Issues Assigned</h3>
          <div class="stat-number">{{ myIssues.length }}</div>
        </div>
        <div class="stat-card stat-card-resolved">
          <h3 class="stat-label">Issues Resolved</h3>
          <div class="stat-number">{{ resolvedCount }}</div>
        </div>
      </div>

      <!-- Assigned Issues Section -->
      <section class="issues-section">
        <h2 class="section-title">Your Assigned Queue</h2>

        <!-- Empty State -->
        <div v-if="myIssues.length === 0" class="empty-state">
          <div class="empty-icon">📋</div>
          <p class="empty-message">No issues assigned to you yet</p>
        </div>

        <!-- Issues Table -->
        <div v-else class="issues-table-wrapper">
          <table class="issues-table">
            <thead>
              <tr>
                <th class="col-id">ID</th>
                <th class="col-title">Title</th>
                <th class="col-priority">Priority</th>
                <th class="col-status">Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="issue in myIssues" :key="issue.issue_key" class="issue-row">
                <td class="col-id">
                  <router-link :to="'/issue/' + issue.issue_key" class="issue-id-link">
                    {{ issue.issue_key }}
                  </router-link>
                </td>
                <td class="col-title">
                  <router-link :to="'/issue/' + issue.issue_key" class="issue-title-link">
                    {{ issue.title }}
                  </router-link>
                </td>
                <td class="col-priority">
                  <span :class="['badge', 'badge-priority', 'priority-' + (issue.priority || 'medium')]">
                    {{ (issue.priority || 'medium').toUpperCase() }}
                  </span>
                </td>
                <td class="col-status">
                  <span :class="['badge', 'badge-status', 'status-' + issue.status]">
                    {{ (issue.status || 'open').replace('_', ' ').charAt(0).toUpperCase() + (issue.status || 'open').replace('_', ' ').slice(1) }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
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
  --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
  --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.07), 0 2px 4px -1px rgba(0, 0, 0, 0.05);
  --color-blue: #3b82f6;
  --color-green: #10b981;
  --color-green-light: #dcfce7;
}

:global(html[data-theme="dark"]) .page-container {
  --bg-primary: #1f2937;
  --bg-secondary: #111827;
  --bg-tertiary: #0f1419;
  --text-primary: #f3f4f6;
  --text-secondary: #d1d5db;
  --text-tertiary: #9ca3af;
  --border-color: #374151;
  --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.3);
  --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.3), 0 2px 4px -1px rgba(0, 0, 0, 0.2);
  --color-green-light: #064e3b;
}

* {
  box-sizing: border-box;
}

.page-container {
  max-width: 900px;
  margin: 0 auto;
  padding: 2rem 1rem;
  background: var(--bg-primary);
  color: var(--text-primary);
}

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
  to { transform: rotate(360deg); }
}

.loading-text {
  font-size: 0.9375rem;
  color: var(--text-secondary);
  margin: 0;
}

.profile-content {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.profile-header {
  text-align: center;
  padding: 2rem;
  background: var(--bg-secondary);
  border-radius: 0.875rem;
  border: 1px solid var(--border-color);
}

.profile-avatar {
  font-size: 3.5rem;
  margin-bottom: 1rem;
  display: block;
}

.profile-email {
  margin: 0 0 0.5rem 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

.profile-role {
  margin: 0;
  font-size: 0.9375rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1.5rem;
}

.stat-card {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 0.875rem;
  padding: 1.75rem;
  text-align: center;
  box-shadow: var(--shadow-sm);
  transition: all 0.2s ease;
}

.stat-card:hover {
  box-shadow: var(--shadow-md);
  border-color: var(--color-blue);
}

.stat-label {
  margin: 0 0 1rem 0;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.stat-number {
  font-size: 2.5rem;
  font-weight: 700;
  color: var(--color-blue);
  line-height: 1;
}

.stat-card-resolved .stat-number {
  color: var(--color-green);
}

.issues-section {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.section-title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-primary);
  padding-bottom: 1rem;
  border-bottom: 2px solid var(--border-color);
}

.empty-state {
  padding: 3rem 1.5rem;
  text-align: center;
  background: var(--bg-secondary);
  border: 1px dashed var(--border-color);
  border-radius: 0.75rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1rem;
}

.empty-icon {
  font-size: 2.5rem;
  opacity: 0.6;
}

.empty-message {
  margin: 0;
  font-size: 0.9375rem;
  color: var(--text-secondary);
}

.issues-table-wrapper {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 0.875rem;
  overflow: hidden;
  box-shadow: var(--shadow-sm);
}

.issues-table {
  width: 100%;
  border-collapse: collapse;
  background: var(--bg-primary);
}

.issues-table thead {
  background: var(--bg-secondary);
  border-bottom: 2px solid var(--border-color);
}

.issues-table th {
  padding: 1rem 1rem;
  text-align: left;
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.issues-table td {
  padding: 0.875rem 1rem;
  border-bottom: 1px solid var(--border-color);
  color: var(--text-primary);
  vertical-align: middle;
}

.issues-table tbody tr:hover {
  background-color: var(--bg-secondary);
}

.issues-table tbody tr:last-child td {
  border-bottom: none;
}

.col-id { width: 80px; }
.col-status { width: 140px; text-align: center; }
.col-priority { width: 120px; text-align: center; }
.col-title { max-width: 400px; }

.issue-id-link, .issue-title-link {
  color: var(--color-blue);
  text-decoration: none;
  font-weight: 500;
  transition: all 0.2s ease;
}

.issue-id-link:hover, .issue-title-link:hover {
  text-decoration: underline;
  opacity: 0.8;
}

.issue-id-link { font-weight: 600; font-size: 0.9375rem; }
.issue-title-link { font-size: 0.95rem; word-break: break-word; }

.badge {
  display: inline-block;
  padding: 0.375rem 0.625rem;
  border-radius: 0.375rem;
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: capitalize;
  white-space: nowrap;
}

.badge-status { background: var(--bg-tertiary); color: var(--text-secondary); }
.badge-status.status-open { background: #fee2e2; color: #991b1b; }
.badge-status.status-in_progress { background: #ffedd5; color: #92400e; }
.badge-status.status-resolved, .badge-status.status-closed,
.badge-status.status-verified { background: var(--color-green-light); color: #166534; }

.badge-priority { background: var(--bg-tertiary); color: var(--text-secondary); }
.badge-priority.priority-critical { background: #fee2e2; color: #991b1b; }
.badge-priority.priority-high { background: #ffedd5; color: #9a3412; }
.badge-priority.priority-medium { background: #fef3c7; color: #92400e; }
.badge-priority.priority-low { background: #f0fdf4; color: #166534; }

@media (max-width: 768px) {
  .page-container { padding: 1.5rem 1rem; }
  .profile-email { font-size: 1.25rem; }
  .profile-avatar { font-size: 2.5rem; }
  .stats-grid { grid-template-columns: 1fr; }
  .issues-table { font-size: 0.9375rem; }
  .issues-table th, .issues-table td { padding: 0.75rem 0.5rem; }
  .col-id, .col-status, .col-priority { width: auto; }
  .col-title { max-width: 200px; }
}
</style>
