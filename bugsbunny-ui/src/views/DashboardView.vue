<!-- src/views/DashboardView.vue -->
<script setup>
import { computed, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { API_URL } from '@/config'

const router = useRouter()
// P2: full issue field set (severity + environment preserved alongside P3's component/module/release)
const form = ref({ project_id: '', title: '', body: '', priority: 'low', severity: 'minor', component: '', environment: '', module: '', release: '' })
const isSubmitting = ref(false)
const projects = ref([]) // State for our dropdown
const dashboardIssues = ref([])
const pageError = ref('')
const validationError = ref('')
const successMessage = ref('')

// Add staging state at the top
const stagedFiles = ref([])

const submitStatus = ref('Submit Issue')

const handleFileSelect = (event) => {
  // Convert FileList to Array and append to existing staged files
  const newFiles = Array.from(event.target.files)
  stagedFiles.value.push(...newFiles)
  event.target.value = '' // Clear input so the same file can be selected again if removed
}

const removeFile = (index) => {
  stagedFiles.value.splice(index, 1)
}

// Update your submitIssue function:
const submitIssue = async () => {
  validationError.value = ''
  successMessage.value = ''

  if (!form.value.project_id || !form.value.title.trim() || !form.value.body.trim()) {
    validationError.value = 'Project, title, and description are required.'
    return
  }

  isSubmitting.value = true
  submitStatus.value = 'Creating Issue...'

  const token = localStorage.getItem('bunny_token')

  try {
    // STEP 1: Create the Issue — server generates a collision-safe issue_key
    const response = await fetch(`${API_URL}/issues`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({
        project_id: form.value.project_id,
        title: form.value.title,
        body: form.value.body,
        priority: form.value.priority,
        severity: form.value.severity,
        component: form.value.component,
        environment: form.value.environment,
        module: form.value.module,
        release: form.value.release
      })
    })

    if (!response.ok) throw new Error(await response.text() || 'Failed to create issue')
    const created = await response.json()
    const newIssueKey = created.issue_key

    // STEP 2: Upload Attachments (Run in parallel for speed)
    if (stagedFiles.value.length > 0) {
      const uploadPromises = stagedFiles.value.map(file => {
        const formData = new FormData()
        formData.append('file', file)
        return fetch(`${API_URL}/issues/${newIssueKey}/attachments`, {
          method: 'POST',
          headers: { 'Authorization': `Bearer ${token}` }, // No Content-Type!
          body: formData
        })
      })
      await Promise.all(uploadPromises) // Wait for all uploads to finish
    }

    // STEP 3: Redirect
    submitStatus.value = 'Redirecting...'
    successMessage.value = 'Issue created successfully.'
    router.push(`/issue/${newIssueKey}`)
  } catch (error) {
    console.error(error)
    validationError.value = error.message || 'Unable to create the issue. Please try again.'
  } finally {
    isSubmitting.value = false
  }
}

// P3 metrics strip, adapted to P2's flat issue fields (no custom_data on this backend).
const metrics = computed(() => {
  const normalized = dashboardIssues.value.map(issue => ({
    ...issue,
    priority: (issue.priority || 'low').toString().toLowerCase(),
    status: String(issue.status || 'open').toLowerCase()
  }))

  const resolvedStatuses = new Set(['resolved', 'closed'])
  const now = Date.now()

  return [
    { label: 'Total bugs', value: normalized.length, tone: 'neutral', detail: 'All visible issues' },
    { label: 'Open bugs', value: normalized.filter(issue => !resolvedStatuses.has(issue.status)).length, tone: 'open', detail: 'Still in workflow' },
    { label: 'Critical bugs', value: normalized.filter(issue => issue.priority === 'critical').length, tone: 'critical', detail: 'Highest priority' },
    {
      label: 'Overdue bugs',
      value: normalized.filter(issue => {
        const dueDate = issue.due_date
        if (!dueDate || resolvedStatuses.has(issue.status)) return false
        return new Date(dueDate).getTime() < now
      }).length,
      tone: 'overdue',
      detail: 'Past due date'
    },
    { label: 'Resolved bugs', value: normalized.filter(issue => resolvedStatuses.has(issue.status)).length, tone: 'resolved', detail: 'Resolved or closed' },
    {
      label: 'Aging bugs',
      value: normalized.filter(issue => {
        if (resolvedStatuses.has(issue.status) || !issue.created_at) return false
        return now - new Date(issue.created_at).getTime() > 14 * 24 * 60 * 60 * 1000
      }).length,
      tone: 'aging',
      detail: 'Open for 14+ days'
    }
  ]
})

// Fetch projects and issue data from the existing API
const loadDashboardData = async () => {
  pageError.value = ''
  const token = localStorage.getItem('bunny_token')
  try {
    const [projectResponse, issueResponse] = await Promise.all([
      fetch(`${API_URL}/projects`, {
        headers: { 'Authorization': `Bearer ${token}` }
      }),
      fetch(`${API_URL}/issues`, {
        headers: { 'Authorization': `Bearer ${token}` }
      })
    ])

    if (!projectResponse.ok || !issueResponse.ok) {
      throw new Error('Unable to load dashboard data — your session may have expired, try logging in again.')
    }

    projects.value = await projectResponse.json()
    dashboardIssues.value = await issueResponse.json()

    // Auto-select the first project if one exists
    if (projects.value.length > 0 && !form.value.project_id) {
      form.value.project_id = projects.value[0].id
    } else if (projects.value.length === 0) {
      pageError.value = 'No active projects exist yet. Ask an admin to create one first.'
    }
  } catch (e) {
    console.error('Failed to load dashboard data', e)
    pageError.value = e.message || 'Unable to load dashboard data.'
  }
}

onMounted(loadDashboardData)
</script>

<template>
  <main class="page-container">
    <!-- Page Header -->
    <div class="page-header">
      <div class="header-content">
        <h1 class="page-title">Project Dashboard</h1>
        <p class="page-subtitle">Create a new issue or review project metrics</p>
      </div>
    </div>

    <!-- Metrics Section -->
    <section class="metrics-section" aria-label="Bug overview">
      <h2 class="section-title">Overview</h2>
      <div class="metrics-grid">
        <article v-for="metric in metrics" :key="metric.label" :class="['metric-card', metric.tone]">
          <h3 class="metric-label">{{ metric.label }}</h3>
          <div class="metric-value">{{ metric.value }}</div>
          <p class="metric-detail">{{ metric.detail }}</p>
        </article>
      </div>
    </section>

    <!-- Error and Success Messages -->
    <div v-if="pageError" class="alert alert-error" role="alert">
      <span class="alert-icon">⚠️</span>
      {{ pageError }}
      <button type="button" @click="loadDashboardData" class="retry-btn">Retry</button>
    </div>
    <div v-if="successMessage" class="alert alert-success" role="status">
      <span class="alert-icon">✓</span>
      {{ successMessage }}
    </div>

    <!-- Create Issue Form -->
    <section class="create-form-section">
      <h2 class="section-title">Create New Issue</h2>
      <form @submit.prevent="submitIssue" class="create-form">

        <!-- Row 1: Project & Title -->
        <div class="form-row">
          <div class="form-group form-group-full">
            <label for="project-select">Project <span class="required">*</span></label>
            <select id="project-select" v-model="form.project_id" required class="form-input" :disabled="!projects.length">
              <option value="" disabled>{{ projects.length ? 'Select a project...' : 'No projects available' }}</option>
              <option v-for="p in projects" :key="p.id" :value="p.id">
                {{ p.name }} ({{ p.project_key }})
              </option>
            </select>
          </div>
        </div>

        <div class="form-row">
          <div class="form-group form-group-full">
            <label for="issue-title">Title <span class="required">*</span></label>
            <input
              id="issue-title"
              v-model="form.title"
              type="text"
              placeholder="Brief summary of the issue"
              required
              class="form-input"
            />
          </div>
        </div>

        <!-- Row 2: Description -->
        <div class="form-row">
          <div class="form-group form-group-full">
            <label for="issue-description">Description <span class="required">*</span></label>
            <textarea
              id="issue-description"
              v-model="form.body"
              rows="4"
              placeholder="Provide detailed information about the issue..."
              required
              class="form-input"
            ></textarea>
          </div>
        </div>

        <!-- Row 3: Priority, Severity & Custom Fields (P2 severity preserved) -->
        <div class="form-row">
          <div class="form-group">
            <label for="priority-select">Priority</label>
            <select id="priority-select" v-model="form.priority" class="form-input">
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="critical">Critical</option>
            </select>
          </div>

          <div class="form-group">
            <label for="severity-select">Severity</label>
            <select id="severity-select" v-model="form.severity" class="form-input">
              <option value="minor">Minor</option>
              <option value="major">Major</option>
              <option value="critical">Critical</option>
              <option value="blocker">Blocker</option>
            </select>
          </div>

          <div class="form-group">
            <label for="component-input">Component</label>
            <input
              id="component-input"
              v-model="form.component"
              type="text"
              placeholder="e.g. Database, UI, Auth"
              class="form-input"
            />
          </div>

          <div class="form-group">
            <label for="environment-input">Environment</label>
            <input
              id="environment-input"
              v-model="form.environment"
              type="text"
              placeholder="e.g. Production, Staging, Local"
              class="form-input"
            />
          </div>
        </div>

        <div class="form-row">
          <div class="form-group">
            <label for="module-input">Module</label>
            <input
              id="module-input"
              v-model="form.module"
              type="text"
              placeholder="e.g. Login Screen, API"
              class="form-input"
            />
          </div>

          <div class="form-group">
            <label for="release-input">Target Release</label>
            <input
              id="release-input"
              v-model="form.release"
              type="text"
              placeholder="e.g. v1.2.0"
              class="form-input"
            />
          </div>
        </div>

        <!-- Row 4: Attachments -->
        <div class="form-row">
          <div class="form-group form-group-full">
            <label for="file-input">Attachments (Logs, Screenshots, etc.)</label>
            <div class="file-input-wrapper">
              <input
                id="file-input"
                type="file"
                @change="handleFileSelect"
                multiple
                class="file-input-hidden"
              />
              <label for="file-input" class="file-input-label">📎 Choose Files</label>
              <span class="file-input-hint">or drag and drop</span>
            </div>

            <!-- Staged Files List -->
            <div v-if="stagedFiles.length > 0" class="staged-files-list">
              <div v-for="(file, index) in stagedFiles" :key="index" class="staged-file-item">
                <span class="file-icon">📄</span>
                <span class="file-name">{{ file.name }}</span>
                <span class="file-size">({{ (file.size / 1024).toFixed(1) }} KB)</span>
                <button type="button" @click="removeFile(index)" class="btn-remove" title="Remove file">✕</button>
              </div>
            </div>
          </div>
        </div>

        <!-- Validation Error -->
        <div v-if="validationError" class="alert alert-error" role="alert">
          <span class="alert-icon">⚠️</span>
          {{ validationError }}
        </div>

        <!-- Submit Button -->
        <div class="form-actions">
          <button type="submit" :disabled="isSubmitting" class="btn btn-primary">
            <span v-if="isSubmitting" class="spinner-mini"></span>
            {{ submitStatus }}
          </button>
        </div>
      </form>
    </section>
  </main>
</template>

<style scoped>
.page-container {
  max-width: 1100px;
  margin: 0 auto;
  padding: 40px 24px 60px;
  box-sizing: border-box;
}

.page-header {
  margin-bottom: 32px;
}

.page-title {
  margin: 0 0 8px;
  font-size: 2rem;
  font-weight: 700;
  color: var(--text-h);
}

.page-subtitle {
  margin: 0;
  color: var(--text);
  font-size: 1rem;
}

/* Section headings */
.section-title {
  margin: 0 0 16px;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-h);
}

/* Metrics */
.metrics-section {
  margin-bottom: 32px;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.metric-card {
  background: var(--bg);
  border: 1px solid var(--border);
  border-left: 4px solid #64748b;
  border-radius: 10px;
  padding: 20px;
  box-shadow: var(--shadow);
  box-sizing: border-box;
}

.metric-card.open { border-left-color: #2563eb; }
.metric-card.critical { border-left-color: #dc2626; }
.metric-card.overdue { border-left-color: #ea580c; }
.metric-card.resolved { border-left-color: #16a34a; }
.metric-card.aging { border-left-color: #ca8a04; }
.metric-card.neutral { border-left-color: #64748b; }

.metric-label {
  display: block;
  margin: 0;
  color: var(--text);
  font-size: 0.9rem;
  font-weight: 600;
}

.metric-value {
  display: block;
  margin: 6px 0;
  color: var(--text-h);
  font-size: 2rem;
  line-height: 1.2;
  font-weight: 700;
}

.metric-detail {
  margin: 0;
  color: var(--text);
  font-size: 0.82rem;
}

/* Alerts */
.alert {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  margin-bottom: 20px;
  border-radius: 8px;
  border: 1px solid;
  font-size: 0.9rem;
}

.alert-error {
  background: #fef2f2;
  border-color: #fecaca;
  color: #991b1b;
}

.alert-success {
  background: #f0fdf4;
  border-color: #bbf7d0;
  color: #166534;
}

.alert-icon {
  font-weight: 700;
}

.retry-btn {
  margin-left: auto;
  background: #991b1b;
  color: white;
  border: none;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 0.85em;
  cursor: pointer;
}

/* Create issue section */
.create-form-section {
  margin-top: 32px;
}

.create-form {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 28px;
  box-shadow: var(--shadow);
  box-sizing: border-box;
}

.form-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
  margin-bottom: 20px;
}

.form-group-full {
  grid-column: 1 / -1;
}

.form-group {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-group label {
  color: var(--text-h);
  font-size: 0.9rem;
  font-weight: 600;
}

.required {
  color: #dc2626;
}

/* IMPORTANT: visible boxes around inputs */
.form-input {
  width: 100%;
  box-sizing: border-box;
  padding: 12px 14px;
  border: 1px solid #cbd5e1;
  border-radius: 7px;
  background: var(--bg);
  color: var(--text-h);
  font-family: inherit;
  font-size: 0.95rem;
  outline: none;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.form-input:hover {
  border-color: #94a3b8;
}

.form-input:focus {
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12);
}

textarea.form-input {
  resize: vertical;
  min-height: 110px;
}

/* Attachments */
.file-input-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px;
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  background: var(--bg);
}

.file-input-hidden {
  display: none;
}

.file-input-label {
  display: inline-flex;
  align-items: center;
  padding: 9px 14px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--bg);
  color: var(--text-h);
  font-weight: 600;
  cursor: pointer;
}

.file-input-label:hover {
  background: var(--code-bg);
}

.file-input-hint {
  color: var(--text);
  font-size: 0.85rem;
}

.staged-files-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 10px;
}

.staged-file-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid #fcd34d;
  border-radius: 7px;
  background: #fffbeb;
  font-size: 0.85rem;
}

.file-icon {
  flex-shrink: 0;
}

.file-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  color: #78716c;
}

.btn-remove {
  flex-shrink: 0;
  padding: 3px 7px;
  border: none;
  background: transparent;
  color: #dc2626;
  cursor: pointer;
  font-size: 1rem;
}

.btn-remove:hover {
  color: #991b1b;
  transform: scale(1.08);
}

/* Submit */
.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 8px;
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 11px 20px;
  border: none;
  border-radius: 7px;
  font-family: inherit;
  font-size: 0.95rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s, transform 0.1s;
}

.btn-primary {
  background: #18181b;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #3f3f46;
}

.btn-primary:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.spinner-mini {
  width: 15px;
  height: 15px;
  border: 2px solid rgba(255, 255, 255, 0.35);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* Dark mode */
[data-theme="dark"] .form-input {
  border-color: #475569;
}

[data-theme="dark"] .form-input:hover {
  border-color: #64748b;
}

[data-theme="dark"] .file-input-wrapper {
  border-color: #475569;
}

[data-theme="dark"] .file-input-label {
  border-color: #475569;
}

/* Responsive */
@media (max-width: 800px) {
  .page-container {
    padding: 28px 16px 40px;
  }

  .metrics-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .form-row {
    grid-template-columns: 1fr;
  }

  .form-group-full {
    grid-column: auto;
  }
}

@media (max-width: 520px) {
  .metrics-grid {
    grid-template-columns: 1fr;
  }

  .create-form {
    padding: 20px 16px;
  }

  .file-input-wrapper {
    flex-direction: column;
    align-items: flex-start;
  }

  .form-actions {
    justify-content: stretch;
  }

  .btn {
    width: 100%;
  }
}
</style>
