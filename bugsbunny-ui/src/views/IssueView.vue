<!-- src/views/IssueView.vue -->
<script setup>
import { ref, computed, onMounted, defineAsyncComponent, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { API_URL } from '@/config'

const API = API_URL
const route = useRoute()
const router = useRouter()
const issue = ref(null)
const loading = ref(true)

// Edit Mode State
const isEditing = ref(false)
const editForm = ref({ title: '', body: '', status: '', priority: '', severity: '', component: '', environment: '', module: '', release: '' })
const isSaving = ref(false)
const saveError = ref('')

// Comments
const comments = ref([])
const newComment = ref('')
const isSubmittingComment = ref(false)

// Attachments
const attachments = ref([])
const stagedFiles = ref([])

// P2: assignment, labels, relationships, activity, watch, insights
const users = ref([])
const isAssigning = ref(false)
const projectLabels = ref([])
const issueLabels = ref([])
const newLabelName = ref('')
const relationships = ref([])
const newRelation = ref({ related_issue_key: '', relation_type: 'related_to' })
const activity = ref([])
const watchers = ref([])
const isWatching = ref(false)
const healthScore = ref(null)
const whyStuck = ref(null)
const duplicates = ref(null)
const insightsLoading = ref(false)

const currentUserEmail = () => localStorage.getItem('bunny_email') || ''

// --- Lifecycle: mirrors the backend's validTransitions map ---
const VALID_TRANSITIONS = {
  open: ['triaged'],
  triaged: ['in_progress'],
  in_progress: ['resolved'],
  resolved: ['verified', 'open'],
  verified: ['closed', 'open'],
  closed: ['open']
}
const STATUS_LABELS = {
  open: 'Open', triaged: 'Triaged', in_progress: 'In Progress',
  resolved: 'Resolved', verified: 'Verified', closed: 'Closed'
}
const availableStatuses = computed(() => {
  if (!issue.value) return []
  const current = issue.value.status
  const next = VALID_TRANSITIONS[current] || []
  // Always allow keeping the current status (no-op) plus valid forward/reopen moves.
  return [current, ...next]
})

const handleFileSelect = (event) => {
  stagedFiles.value.push(...Array.from(event.target.files))
  event.target.value = ''
}
const removeFile = (index) => stagedFiles.value.splice(index, 1)

const authHeaders = (extra = {}) => {
  const token = localStorage.getItem('bunny_token')
  return { 'Authorization': `Bearer ${token}`, ...extra }
}

const postComment = async () => {
  if (!newComment.value.trim() && stagedFiles.value.length === 0) return
  isSubmittingComment.value = true

  if (newComment.value.trim()) {
    await fetch(`${API}/issues/${route.params.id}/comments`, {
      method: 'POST',
      headers: authHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ body: newComment.value })
    })
  }

  if (stagedFiles.value.length > 0) {
    const uploadPromises = stagedFiles.value.map(file => {
      const formData = new FormData()
      formData.append('file', file)
      return fetch(`${API}/issues/${route.params.id}/attachments`, {
        method: 'POST',
        headers: authHeaders(),
        body: formData
      })
    })
    await Promise.all(uploadPromises)
  }

  newComment.value = ''
  stagedFiles.value = []
  await fetchComments(route.params.id)
  await fetchAttachments(route.params.id)
  await fetchActivity(route.params.id)
  isSubmittingComment.value = false
}

const fetchAttachments = async (id) => {
  const res = await fetch(`${API}/issues/${id}/attachments`, { headers: authHeaders() })
  if (res.ok) attachments.value = await res.json()
}

const fetchComments = async (id) => {
  const res = await fetch(`${API}/issues/${id}/comments`, { headers: authHeaders() })
  if (res.ok) comments.value = await res.json()
}

const fetchActivity = async (id) => {
  const res = await fetch(`${API}/issues/${id}/activity`, { headers: authHeaders() })
  if (res.ok) activity.value = await res.json()
}

const fetchUsers = async () => {
  const res = await fetch(`${API}/users/list`, { headers: authHeaders() })
  if (res.ok) users.value = await res.json()
}

const fetchIssueLabels = async (id) => {
  const res = await fetch(`${API}/issues/${id}/labels`, { headers: authHeaders() })
  if (res.ok) issueLabels.value = await res.json()
}

const fetchProjectLabels = async (projectId) => {
  if (!projectId) return
  const res = await fetch(`${API}/projects/${projectId}/labels`, { headers: authHeaders() })
  if (res.ok) projectLabels.value = await res.json()
}

const fetchRelationships = async (id) => {
  const res = await fetch(`${API}/issues/${id}/relationships`, { headers: authHeaders() })
  if (res.ok) relationships.value = await res.json()
}

const fetchWatchers = async (id) => {
  const res = await fetch(`${API}/issues/${id}/watchers`, { headers: authHeaders() })
  if (res.ok) {
    watchers.value = await res.json()
    isWatching.value = watchers.value.some(w => w.email === currentUserEmail())
  }
}

const fetchIssue = async (id) => {
  loading.value = true
  try {
    const response = await fetch(`${API}/issues/${id}`, { headers: authHeaders() })
    issue.value = await response.json()

    await Promise.all([
      fetchAttachments(id),
      fetchComments(id),
      fetchActivity(id),
      fetchUsers(),
      fetchIssueLabels(id),
      fetchRelationships(id),
      fetchWatchers(id)
    ])
    if (issue.value?.project_id) await fetchProjectLabels(issue.value.project_id)

    // Reset insight panels — they're fetched on demand per issue.
    healthScore.value = null
    whyStuck.value = null
    duplicates.value = null
    slaForecast.value = null
    blastRadius.value = null
    summaryReport.value = null
    showReplay.value = false
    replayIndex.value = 0
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const SlaPluginWidget = defineAsyncComponent(() => import('../ui-plugins/SlaWidget.vue'))

const startEditing = () => {
  editForm.value = {
    title: issue.value.title,
    body: issue.value.body,
    status: issue.value.status,
    priority: issue.value.priority || 'medium',
    severity: issue.value.severity || 'minor',
    component: issue.value.component || '',
    environment: issue.value.environment || '',
    module: issue.value.module || '',
    release: issue.value.release || ''
  }
  saveError.value = ''
  isEditing.value = true
}

const saveEdit = async () => {
  isSaving.value = true
  saveError.value = ''
  try {
    const response = await fetch(`${API}/issues/${route.params.id}`, {
      method: 'PUT',
      headers: authHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify(editForm.value)
    })

    if (response.ok) {
      isEditing.value = false
      await fetchIssue(route.params.id)
    } else {
      saveError.value = await response.text()
    }
  } catch (error) {
    saveError.value = 'Failed to update issue.'
    console.error("Failed to update issue:", error)
  } finally {
    isSaving.value = false
  }
}

const deleteIssue = async () => {
  if (!window.confirm(`Are you sure you want to delete ${issue.value.issue_key}? This cannot be undone.`)) return
  try {
    const response = await fetch(`${API}/issues/${route.params.id}`, {
      method: 'DELETE',
      headers: authHeaders()
    })
    if (response.ok) router.push('/queue')
  } catch (error) {
    console.error("Failed to delete issue:", error)
  }
}

// --- Assignment ---
const assignTo = async (event) => {
  const val = event.target.value
  isAssigning.value = true
  await fetch(`${API}/issues/${route.params.id}/assign`, {
    method: 'PUT',
    headers: authHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({ assignee_id: val ? parseInt(val) : null })
  })
  await fetchIssue(route.params.id)
  isAssigning.value = false
}

// --- Labels ---
const addExistingLabel = async (event) => {
  const labelId = event.target.value
  if (!labelId) return
  await fetch(`${API}/issues/${route.params.id}/labels`, {
    method: 'POST',
    headers: authHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({ label_id: parseInt(labelId) })
  })
  event.target.value = ''
  await fetchIssueLabels(route.params.id)
  await fetchActivity(route.params.id)
}

const createAndAddLabel = async () => {
  if (!newLabelName.value.trim()) return
  const res = await fetch(`${API}/projects/${issue.value.project_id}/labels`, {
    method: 'POST',
    headers: authHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify({ name: newLabelName.value.trim() })
  })
  if (res.ok) {
    const label = await res.json()
    await fetch(`${API}/issues/${route.params.id}/labels`, {
      method: 'POST',
      headers: authHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ label_id: label.id })
    })
    newLabelName.value = ''
    await fetchProjectLabels(issue.value.project_id)
    await fetchIssueLabels(route.params.id)
  }
}

const removeLabel = async (labelId) => {
  await fetch(`${API}/issues/${route.params.id}/labels/${labelId}`, {
    method: 'DELETE', headers: authHeaders()
  })
  await fetchIssueLabels(route.params.id)
  await fetchActivity(route.params.id)
}

// --- Relationships ---
const addRelationship = async () => {
  if (!newRelation.value.related_issue_key.trim()) return
  const res = await fetch(`${API}/issues/${route.params.id}/relationships`, {
    method: 'POST',
    headers: authHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(newRelation.value)
  })
  if (res.ok) {
    newRelation.value.related_issue_key = ''
    await fetchRelationships(route.params.id)
    await fetchActivity(route.params.id)
  } else {
    alert(await res.text())
  }
}

const removeRelationship = async (relId) => {
  await fetch(`${API}/issues/${route.params.id}/relationships/${relId}`, {
    method: 'DELETE', headers: authHeaders()
  })
  await fetchRelationships(route.params.id)
  await fetchActivity(route.params.id)
}

// --- Watch/Follow ---
const toggleWatch = async () => {
  const method = isWatching.value ? 'DELETE' : 'POST'
  await fetch(`${API}/issues/${route.params.id}/watch`, { method, headers: authHeaders() })
  await fetchWatchers(route.params.id)
}

// --- Insights (fetched on demand) ---
const loadHealthScore = async () => {
  insightsLoading.value = true
  const res = await fetch(`${API}/issues/${route.params.id}/health-score`, { headers: authHeaders() })
  if (res.ok) healthScore.value = await res.json()
  insightsLoading.value = false
}
const loadWhyStuck = async () => {
  insightsLoading.value = true
  const res = await fetch(`${API}/issues/${route.params.id}/why-stuck`, { headers: authHeaders() })
  if (res.ok) whyStuck.value = await res.json()
  insightsLoading.value = false
}
const loadDuplicates = async () => {
  insightsLoading.value = true
  const res = await fetch(`${API}/issues/${route.params.id}/duplicates`, { headers: authHeaders() })
  if (res.ok) duplicates.value = await res.json()
  insightsLoading.value = false
}

// --- P2+ advanced features ---
const slaForecast = ref(null)
const blastRadius = ref(null)
const summaryReport = ref(null)
const replayIndex = ref(0)
const showReplay = ref(false)

const loadSLAForecast = async () => {
  insightsLoading.value = true
  const res = await fetch(`${API}/issues/${route.params.id}/sla-forecast`, { headers: authHeaders() })
  if (res.ok) slaForecast.value = await res.json()
  insightsLoading.value = false
}
const loadBlastRadius = async () => {
  insightsLoading.value = true
  const res = await fetch(`${API}/issues/${route.params.id}/blast-radius`, { headers: authHeaders() })
  if (res.ok) blastRadius.value = await res.json()
  insightsLoading.value = false
}
const loadSummaryReport = async () => {
  insightsLoading.value = true
  const res = await fetch(`${API}/issues/${route.params.id}/summary-report`, { headers: authHeaders() })
  if (res.ok) summaryReport.value = await res.json()
  insightsLoading.value = false
}
const copySummary = () => {
  const s = summaryReport.value
  if (!s) return
  const text = `${s.issue_key}: ${s.title}\nStatus: ${s.current_status} | Age: ${s.total_age_hours.toFixed(1)}h | Reopened: ${s.reopen_count}x\nComments: ${s.comment_count} | Attachments: ${s.attachment_count}\nContributors: ${s.contributors.join(', ')}\n\nTimeline:\n${s.status_timeline.join('\n')}`
  navigator.clipboard.writeText(text)
  alert('Summary copied to clipboard.')
}

// Time-Travel Replay — reconstructs issue state at any point by replaying
// the activity log client-side. Reuses data already fetched via fetchActivity.
const replayableEvents = computed(() => activity.value.filter(a =>
  ['status_changed', 'reopened', 'priority_changed', 'severity_changed', 'assignee_changed', 'environment_changed', 'component_changed'].includes(a.action_type)
))
const replayState = computed(() => {
  const state = { status: 'open', priority: '-', severity: '-', assignee: 'unassigned', environment: '-', component: '-' }
  for (let i = 0; i <= replayIndex.value && i < replayableEvents.value.length; i++) {
    const a = replayableEvents.value[i]
    if (a.action_type === 'status_changed' || a.action_type === 'reopened') state.status = a.new_value
    if (a.action_type === 'priority_changed') state.priority = a.new_value
    if (a.action_type === 'severity_changed') state.severity = a.new_value
    if (a.action_type === 'assignee_changed') state.assignee = a.new_value
    if (a.action_type === 'environment_changed') state.environment = a.new_value
    if (a.action_type === 'component_changed') state.component = a.new_value
  }
  return state
})

const formatAction = (a) => {
  const labels = {
    issue_created: 'created the issue',
    status_changed: `changed status: ${a.old_value} → ${a.new_value}`,
    reopened: `reopened the issue (${a.old_value} → ${a.new_value})`,
    priority_changed: `changed priority: ${a.old_value} → ${a.new_value}`,
    severity_changed: `changed severity: ${a.old_value} → ${a.new_value}`,
    component_changed: `changed component: ${a.old_value || '(none)'} → ${a.new_value || '(none)'}`,
    environment_changed: `changed environment: ${a.old_value || '(none)'} → ${a.new_value || '(none)'}`,
    mentioned: `mentioned ${a.new_value}`,
    assignee_changed: `reassigned: ${a.old_value} → ${a.new_value}`,
    comment_added: `commented: "${a.new_value}"`,
    attachment_added: `attached ${a.new_value}`,
    label_added: `added label "${a.new_value}"`,
    label_removed: `removed label "${a.old_value}"`,
    relationship_added: `linked issue: ${a.new_value}`,
    relationship_removed: `removed link: ${a.old_value}`,
    resolved: 'marked as resolved',
    verified: 'verified the fix',
    closed: 'closed the issue'
  }
  return labels[a.action_type] || a.action_type
}

onMounted(() => fetchIssue(route.params.id))
watch(() => route.params.id, (newId) => { if (newId) fetchIssue(newId) })
</script>

<template>
  <main class="page-container">
    <router-link to="/queue" class="back-link">← Back to Queue</router-link>

    <div v-if="loading" class="loading-container">
      <div class="spinner"></div>
      <p class="loading-text">Loading issue...</p>
    </div>

    <div v-else-if="issue" class="issue-card">

      <!-- VIEW MODE -->
      <div v-if="!isEditing">

        <div class="card-header">
          <span class="badge">{{ issue.issue_key }}</span>
          <div class="header-actions">
            <button class="btn btn-secondary" :class="{ active: isWatching }" @click="toggleWatch">
              {{ isWatching ? '★ Watching' : '☆ Watch' }}
            </button>
            <button class="btn btn-secondary" @click="startEditing">Edit</button>
            <button class="btn btn-danger" @click="deleteIssue">Delete</button>
          </div>
        </div>

        <h2 class="issue-title-text">{{ issue.title }}</h2>

        <div class="issue-meta">
          <span :class="['status-badge', issue.status]">{{ (issue.status || '').replace('_', ' ').toUpperCase() }}</span>
          <span :class="['meta-badge', issue.priority]">Priority: {{ issue.priority }}</span>
          <span :class="['meta-badge', issue.severity]">Severity: {{ issue.severity }}</span>
          <span v-if="issue.component" class="meta-tag">📦 {{ issue.component }}</span>
          <span v-if="issue.environment" class="meta-tag">🌐 {{ issue.environment }}</span>
          <span v-if="issue.reopen_count > 0" class="meta-tag reopen-tag">🔁 Reopened {{ issue.reopen_count }}×</span>
        </div>

        <div class="assign-row">
          <label>Assignee:</label>
          <select :value="issue.assignee_id || ''" @change="assignTo" :disabled="isAssigning" class="filter-select">
            <option value="">Unassigned</option>
            <option v-for="u in users" :key="u.id" :value="u.id">{{ u.email }}</option>
          </select>
        </div>

        <div class="label-row">
          <span v-for="l in issueLabels" :key="l.id" class="label-chip">
            {{ l.name }}
            <button class="chip-remove" @click="removeLabel(l.id)">×</button>
          </span>
          <select @change="addExistingLabel" class="label-select">
            <option value="">+ Add label</option>
            <option v-for="l in projectLabels" :key="l.id" :value="l.id">{{ l.name }}</option>
          </select>
          <input v-model="newLabelName" placeholder="New label..." class="new-label-input" @keyup.enter="createAndAddLabel" />
          <button class="btn btn-primary btn-sm" @click="createAndAddLabel">Add</button>
        </div>

        <p class="body-text">{{ issue.body }}</p>

        <div class="plugin-slot">
          <p class="slot-label">-- Plugin Slot: Issue Bottom --</p>
          <component :is="SlaPluginWidget" :issueData="issue" />
        </div>

        <!-- RELATIONSHIPS -->
        <div class="section">
          <h3>Relationships</h3>
          <div v-if="relationships.length === 0" class="empty-note">No linked issues yet.</div>
          <div v-else class="relationship-list">
            <div v-for="rel in relationships" :key="rel.id" class="relationship-row">
              <span class="rel-type">{{ rel.relation_type.replace('_', ' ') }}</span>
              <router-link :to="`/issue/${rel.related_issue_key}`">{{ rel.related_issue_key }}</router-link>
              <span class="rel-title">{{ rel.related_title }}</span>
              <button class="chip-remove" @click="removeRelationship(rel.id)">×</button>
            </div>
          </div>
          <div class="relation-form">
            <input v-model="newRelation.related_issue_key" placeholder="Issue key, e.g. BUNNY-2" />
            <select v-model="newRelation.relation_type">
              <option value="related_to">Related to</option>
              <option value="duplicate_of">Duplicate of</option>
              <option value="blocks">Blocks</option>
              <option value="blocked_by">Blocked by</option>
            </select>
            <button class="btn btn-primary btn-sm" @click="addRelationship">Link</button>
          </div>
        </div>

        <!-- INSIGHTS -->
        <div class="section insights-section">
          <h3>Insights</h3>
          <div class="insight-buttons">
            <button class="btn btn-secondary btn-sm" :disabled="insightsLoading" @click="loadHealthScore">Health Score</button>
            <button class="btn btn-secondary btn-sm" :disabled="insightsLoading" @click="loadWhyStuck">Why Is This Stuck?</button>
            <button class="btn btn-secondary btn-sm" :disabled="insightsLoading" @click="loadDuplicates">Find Duplicates</button>
            <button class="btn btn-secondary btn-sm" :disabled="insightsLoading" @click="loadSLAForecast">SLA Forecast</button>
            <button class="btn btn-secondary btn-sm" :disabled="insightsLoading" @click="loadBlastRadius">Blast Radius</button>
            <button class="btn btn-secondary btn-sm" :disabled="insightsLoading" @click="loadSummaryReport">Summary Report</button>
            <button class="btn btn-secondary btn-sm" @click="showReplay = !showReplay">{{ showReplay ? 'Hide' : 'Time-Travel Replay' }}</button>
          </div>

          <div v-if="healthScore" class="insight-panel">
            <strong>Risk score: {{ healthScore.score }}/100 ({{ healthScore.level }})</strong>
            <ul>
              <li v-for="rsn in healthScore.reasons" :key="rsn.factor">{{ rsn.detail }} — +{{ rsn.points }}</li>
            </ul>
          </div>

          <div v-if="whyStuck" class="insight-panel">
            <strong v-if="whyStuck.is_stuck">Stagnation level: {{ whyStuck.stagnation_level }}</strong>
            <strong v-else>Not stuck</strong>
            <ul>
              <li v-for="(reason, i) in whyStuck.reasons" :key="i">{{ reason }}</li>
            </ul>
            <p class="suggested-action">→ {{ whyStuck.suggested_action }}</p>
          </div>

          <div v-if="duplicates" class="insight-panel">
            <div v-if="duplicates.length === 0">No likely duplicates found.</div>
            <div v-for="d in duplicates" :key="d.issue_key" class="duplicate-row">
              <router-link :to="`/issue/${d.issue_key}`">{{ d.issue_key }}</router-link>
              — {{ d.title }} <span class="dup-score">({{ d.reason }})</span>
            </div>
          </div>

          <div v-if="slaForecast" class="insight-panel">
            <strong :class="`sla-${slaForecast.status}`">SLA: {{ slaForecast.status.replace('_', ' ').toUpperCase() }}</strong>
            <p>{{ slaForecast.reason }}</p>
          </div>

          <div v-if="blastRadius" class="insight-panel">
            <strong>Resolving this unblocks {{ blastRadius.total_unblocked }} issue(s)</strong>
            <span v-if="blastRadius.critical_count > 0" class="dup-score"> — {{ blastRadius.critical_count }} critical/blocker</span>
            <div v-if="blastRadius.cascade.length === 0">No issues are downstream of this one.</div>
            <ul v-else>
              <li v-for="c in blastRadius.cascade" :key="c.issue_key">
                <router-link :to="`/issue/${c.issue_key}`">{{ c.issue_key }}</router-link>
                — {{ c.title }} ({{ c.severity }}, depth {{ c.depth }})
              </li>
            </ul>
          </div>

          <div v-if="summaryReport" class="insight-panel">
            <strong>{{ summaryReport.issue_key }}: {{ summaryReport.title }}</strong>
            <p>
              Status: {{ summaryReport.current_status }} · Age: {{ summaryReport.total_age_hours.toFixed(1) }}h
              <span v-if="summaryReport.resolution_hours"> · Resolved in {{ summaryReport.resolution_hours.toFixed(1) }}h</span>
              · Reopened {{ summaryReport.reopen_count }}x · {{ summaryReport.comment_count }} comments · {{ summaryReport.attachment_count }} attachments
            </p>
            <p>Contributors: {{ summaryReport.contributors.join(', ') || 'none yet' }}</p>
            <ul>
              <li v-for="(t, i) in summaryReport.status_timeline" :key="i">{{ t }}</li>
            </ul>
            <button class="btn btn-secondary btn-sm" @click="copySummary">Copy as text</button>
          </div>

          <div v-if="showReplay" class="insight-panel">
            <strong>Time-Travel Replay</strong>
            <div v-if="replayableEvents.length === 0">No state changes recorded yet.</div>
            <div v-else>
              <input type="range" min="0" :max="replayableEvents.length - 1" v-model.number="replayIndex" class="replay-slider" />
              <p class="replay-caption">Step {{ replayIndex + 1 }} / {{ replayableEvents.length }} — {{ new Date(replayableEvents[replayIndex].created_at).toLocaleString() }}</p>
              <div class="replay-state">
                <span :class="['status-badge', replayState.status]">{{ replayState.status.toUpperCase() }}</span>
                <span class="meta-tag">Priority: {{ replayState.priority }}</span>
                <span class="meta-tag">Severity: {{ replayState.severity }}</span>
                <span class="meta-tag">Assignee: {{ replayState.assignee }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- ACTIVITY TIMELINE -->
        <div class="section">
          <h3>Activity</h3>
          <div v-if="activity.length === 0" class="empty-note">No activity yet.</div>
          <div v-else class="activity-list">
            <div v-for="a in activity" :key="a.id" class="activity-row">
              <span class="activity-actor">{{ a.actor_email || 'System' }}</span>
              <span class="activity-text">{{ formatAction(a) }}</span>
              <span class="activity-date">{{ new Date(a.created_at).toLocaleString() }}</span>
            </div>
          </div>
        </div>

        <!-- UNIFIED ATTACHMENTS GALLERY -->
        <div v-if="attachments && attachments.length > 0" class="unified-gallery">
          <div v-for="att in attachments" :key="att.id" class="attachment-card-wrapper">
            <a :href="att.file_url" target="_blank" download class="attachment-card">
              📎 {{ att.filename }} (Download)
            </a>
            <span class="attachment-meta">Uploaded by: {{ att.user_email?.split('@')[0] || 'System' }}</span>
          </div>
        </div>

        <!-- COMMENTS SECTION -->
        <div class="comments-section">
          <h3>Discussion</h3>

          <div v-if="comments.length === 0" class="no-comments">
            No comments yet. Start the conversation!
          </div>

          <div class="comment-list">
            <div v-for="c in comments" :key="c.id" class="comment-bubble">
              <div class="comment-header">
                <strong>👤 {{ c.user_email }}</strong>
                <span class="comment-date">{{ new Date(c.created_at).toLocaleString() }}</span>
              </div>
              <div class="comment-body">{{ c.body }}</div>
            </div>
          </div>

          <form @submit.prevent="postComment" class="comment-form">
            <textarea v-model="newComment" rows="3" placeholder="Add a comment or attach files..."></textarea>

            <div v-if="stagedFiles.length > 0" class="staged-list">
              <div v-for="(file, index) in stagedFiles" :key="index" class="staged-item">
                📎 {{ file.name }}
                <button type="button" @click="removeFile(index)" class="btn-remove">❌</button>
              </div>
            </div>

            <div class="comment-actions">
              <input type="file" @change="handleFileSelect" multiple class="file-input-sm" id="commentFile" />
              <label for="commentFile" class="btn-attach">📎 Attach</label>

              <button type="submit" class="btn btn-primary" :disabled="isSubmittingComment">
                {{ isSubmittingComment ? 'Posting...' : 'Post' }}
              </button>
            </div>
          </form>
        </div>
      </div>

      <!-- EDIT MODE -->
      <form v-else @submit.prevent="saveEdit" class="edit-form">
        <div class="card-header">
          <span class="badge">{{ issue.issue_key }} (Editing)</span>
        </div>

        <div v-if="saveError" class="error-banner">{{ saveError }}</div>

        <div class="form-group">
          <label>Title:</label>
          <input v-model="editForm.title" required class="form-input" />
        </div>

        <div class="form-group">
          <label>Description:</label>
          <textarea v-model="editForm.body" rows="4" required class="form-input"></textarea>
        </div>

        <div style="display: flex; gap: 10px; flex-wrap: wrap;">
          <div class="form-group" style="flex: 1; min-width: 140px;">
            <label>Priority (urgency):</label>
            <select v-model="editForm.priority" class="form-input">
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="critical">Critical</option>
            </select>
          </div>
          <div class="form-group" style="flex: 1; min-width: 140px;">
            <label>Severity (impact):</label>
            <select v-model="editForm.severity" class="form-input">
              <option value="minor">Minor</option>
              <option value="major">Major</option>
              <option value="critical">Critical</option>
              <option value="blocker">Blocker</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label>Status:</label>
          <select v-model="editForm.status" class="form-input">
            <option v-for="s in availableStatuses" :key="s" :value="s">
              {{ STATUS_LABELS[s] }}{{ s === issue.status ? ' (current)' : '' }}
            </option>
          </select>
          <small class="hint">Lifecycle: OPEN → TRIAGED → IN PROGRESS → RESOLVED → VERIFIED → CLOSED. Reopening is allowed from RESOLVED, VERIFIED, or CLOSED.</small>
        </div>

        <div style="display: flex; gap: 10px; flex-wrap: wrap;">
          <div class="form-group" style="flex: 1; min-width: 140px;">
            <label>Component:</label>
            <input v-model="editForm.component" class="form-input" />
          </div>
          <div class="form-group" style="flex: 1; min-width: 140px;">
            <label>Environment:</label>
            <input v-model="editForm.environment" placeholder="Production, Staging..." class="form-input" />
          </div>
          <div class="form-group" style="flex: 1; min-width: 140px;">
            <label>Module:</label>
            <input v-model="editForm.module" class="form-input" />
          </div>
          <div class="form-group" style="flex: 1; min-width: 140px;">
            <label>Release:</label>
            <input v-model="editForm.release" class="form-input" />
          </div>
        </div>

        <div class="action-buttons">
          <button type="button" @click="isEditing = false" class="btn btn-secondary">Cancel</button>
          <button type="submit" :disabled="isSaving" class="btn btn-primary">
            {{ isSaving ? 'Saving...' : 'Save Changes' }}
          </button>
        </div>
      </form>

    </div>
  </main>
</template>

<style scoped>
/* Theming — matches the variable system used across Queue/Profile/Dashboard */
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
  --color-red-light: #fee2e2;
  --color-green-light: #dcfce7;

  max-width: 900px;
  margin: 0 auto;
  padding: 2rem 1rem;
  background: var(--bg-primary);
  color: var(--text-primary);
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
  --color-red-light: #7f1d1d;
  --color-green-light: #064e3b;
}

.back-link { text-decoration: none; color: var(--text-secondary); font-weight: 600; display: inline-block; margin-bottom: 20px; }
.back-link:hover { color: var(--text-primary); }

/* Loading */
.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem 1rem;
  min-height: 250px;
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
@keyframes spin { to { transform: rotate(360deg); } }
.loading-text { font-size: 0.9375rem; color: var(--text-secondary); margin: 0; }

.issue-card { background: var(--bg-primary); border: 1px solid var(--border-color); padding: 30px; border-radius: 0.875rem; box-shadow: var(--shadow-md); }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; flex-wrap: wrap; gap: 10px; }
.issue-title-text { margin: 0 0 15px 0; color: var(--text-primary); }
.badge { background: var(--text-primary); color: var(--bg-primary); padding: 4px 10px; border-radius: 0.375rem; font-size: 0.8em; font-weight: bold; }
.body-text { white-space: pre-wrap; color: var(--text-secondary); line-height: 1.6; margin-top: 15px; }

.issue-meta { display: flex; gap: 10px; background: var(--bg-secondary); padding: 10px 15px; border-radius: 0.5rem; margin-bottom: 15px; align-items: center; flex-wrap: wrap; }
.meta-tag { font-size: 0.85em; color: var(--text-secondary); }
.reopen-tag { color: #b45309; font-weight: bold; }

.meta-badge { font-size: 0.8em; padding: 3px 10px; border-radius: 0.625rem; font-weight: bold; text-transform: capitalize; background: var(--bg-tertiary); color: var(--text-secondary); }
.meta-badge.critical, .meta-badge.blocker { background: var(--color-red-light); color: #991b1b; }
.meta-badge.high, .meta-badge.major { background: #ffedd5; color: #9a3412; }
.meta-badge.medium { background: #fef3c7; color: #92400e; }
.meta-badge.low, .meta-badge.minor { background: var(--color-green-light); color: #166534; }

.status-badge { font-size: 0.8em; padding: 4px 10px; border-radius: 0.625rem; font-weight: bold; text-transform: uppercase; background: var(--bg-tertiary); color: var(--text-secondary); }
.status-badge.open { background: #e0e7ff; color: #3730a3; }
.status-badge.triaged { background: #fef9c3; color: #854d0e; }
.status-badge.in_progress { background: #dbeafe; color: #1e40af; }
.status-badge.resolved { background: var(--color-green-light); color: #166534; }
.status-badge.verified { background: #d1fae5; color: #065f46; }
.status-badge.closed { background: var(--bg-tertiary); color: var(--text-secondary); }

/* Buttons — shared across the page */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 0.5rem;
  font-family: inherit;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s ease, opacity 0.2s ease;
  padding: 8px 14px;
  font-size: 0.9em;
  border: 1px solid transparent;
}
.btn-sm { padding: 6px 12px; font-size: 0.85em; }
.btn-primary { background: var(--text-primary); color: var(--bg-primary); }
.btn-primary:hover:not(:disabled) { opacity: 0.85; }
.btn-secondary { background: var(--bg-primary); color: var(--text-primary); border-color: var(--border-color); }
.btn-secondary:hover:not(:disabled) { background: var(--bg-secondary); }
.btn-secondary.active { background: #fef9c3; border-color: #fde047; color: #854d0e; }
.btn-danger { background: transparent; border-color: var(--color-red-light); color: #991b1b; }
.btn-danger:hover { background: var(--color-red-light); }
.btn:disabled { opacity: 0.55; cursor: not-allowed; }

.plugin-slot { margin-top: 30px; padding: 20px; border: 2px dashed var(--border-color); border-radius: 0.5rem; background: var(--bg-secondary); }
.slot-label { font-size: 0.75em; color: var(--text-tertiary); text-transform: uppercase; margin-top: 0; font-weight: bold; }

.assign-row { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; font-size: 0.9em; }
.assign-row select { padding: 6px 10px; border-radius: 0.375rem; border: 1px solid var(--border-color); background: var(--bg-primary); color: var(--text-primary); }

.label-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 15px; }
.label-chip { background: #e0e7ff; color: #4338ca; padding: 4px 10px; border-radius: 12px; font-size: 0.8em; font-weight: bold; display: inline-flex; align-items: center; gap: 6px; }
.chip-remove { background: none; border: none; cursor: pointer; color: inherit; font-weight: bold; padding: 0; }
.label-select, .new-label-input { padding: 5px 8px; border-radius: 0.375rem; border: 1px solid var(--border-color); font-size: 0.85em; background: var(--bg-primary); color: var(--text-primary); }
.new-label-input { width: 120px; }

.section { margin-top: 30px; border-top: 1px solid var(--border-color); padding-top: 20px; }
.section h3 { color: var(--text-primary); }
.empty-note { color: var(--text-tertiary); font-style: italic; font-size: 0.9em; }

.relationship-list { display: flex; flex-direction: column; gap: 8px; margin-bottom: 12px; }
.relationship-row { display: flex; align-items: center; gap: 10px; font-size: 0.9em; background: var(--bg-secondary); padding: 8px 12px; border-radius: 0.5rem; }
.rel-type { text-transform: uppercase; font-size: 0.75em; font-weight: bold; color: #6366f1; }
.rel-title { color: var(--text-secondary); }
.relation-form { display: flex; gap: 8px; }
.relation-form input { flex: 1; padding: 6px 10px; border: 1px solid var(--border-color); border-radius: 0.375rem; background: var(--bg-primary); color: var(--text-primary); }
.relation-form select { padding: 6px 10px; border: 1px solid var(--border-color); border-radius: 0.375rem; background: var(--bg-primary); color: var(--text-primary); }

.insight-buttons { display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.insight-panel { background: var(--bg-secondary); border: 1px solid var(--border-color); border-radius: 0.5rem; padding: 12px 15px; margin-bottom: 10px; font-size: 0.9em; color: var(--text-primary); }
.insight-panel ul { margin: 8px 0 0 0; padding-left: 20px; }
.suggested-action { margin-top: 8px; font-weight: bold; color: #1e40af; }
.duplicate-row { padding: 4px 0; }
.dup-score { color: var(--text-tertiary); font-size: 0.85em; }
.sla-on_track { color: #166534; }
.sla-at_risk { color: #92400e; }
.sla-breached { color: #991b1b; }
.replay-slider { width: 100%; margin: 10px 0; }
.replay-caption { color: var(--text-secondary); font-size: 0.85em; margin-bottom: 8px; }
.replay-state { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }

.activity-list { display: flex; flex-direction: column; gap: 8px; }
.activity-row { display: flex; gap: 8px; font-size: 0.85em; align-items: baseline; flex-wrap: wrap; }
.activity-actor { font-weight: bold; color: var(--text-primary); }
.activity-date { color: var(--text-tertiary); margin-left: auto; }

/* Edit Form Styles */
.edit-form { display: flex; flex-direction: column; gap: 15px; }
.form-group { display: flex; flex-direction: column; gap: 5px; }
.form-group label { font-size: 0.9em; font-weight: bold; color: var(--text-primary); }
.hint { color: var(--text-tertiary); font-size: 0.8em; }
.form-input {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  font-family: inherit;
  background: var(--bg-primary);
  color: var(--text-primary);
  outline: none;
  transition: border-color 0.2s, box-shadow 0.2s;
}
.form-input:focus {
  border-color: var(--color-blue);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12);
}
.action-buttons { display: flex; gap: 10px; justify-content: flex-end; margin-top: 10px; }
.header-actions { display: flex; gap: 10px; flex-wrap: wrap; }
.error-banner { background: var(--color-red-light); color: #991b1b; padding: 10px 15px; border-radius: 0.5rem; font-size: 0.9em; }

.comments-section { margin-top: 30px; border-top: 1px solid var(--border-color); padding-top: 20px; }
.comments-section h3 { color: var(--text-primary); }
.no-comments { color: var(--text-tertiary); font-style: italic; margin-bottom: 20px; }
.comment-list { display: flex; flex-direction: column; gap: 15px; margin-bottom: 20px; }
.comment-bubble { background: var(--bg-secondary); border: 1px solid var(--border-color); padding: 15px; border-radius: 0.625rem; }
.comment-header { display: flex; justify-content: space-between; margin-bottom: 8px; font-size: 0.9em; color: var(--text-primary); }
.comment-date { color: var(--text-tertiary); }
.comment-body { white-space: pre-wrap; color: var(--text-secondary); line-height: 1.5; }
.comment-form { display: flex; flex-direction: column; gap: 10px; align-items: flex-end; }
.comment-form textarea {
  width: 100%;
  box-sizing: border-box;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  font-family: inherit;
  background: var(--bg-primary);
  color: var(--text-primary);
}

.staged-list { margin: 10px 0; display: flex; flex-direction: column; gap: 5px; width: 100%; }
.staged-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #fffbeb;
  border: 1px solid #fcd34d;
  padding: 8px 12px;
  border-radius: 0.375rem;
  font-size: 0.85em;
  gap: 15px;
  color: #78350f;
}

.btn-remove {
  background: transparent !important;
  padding: 0 !important;
  border: none;
  cursor: pointer;
  color: #dc2626;
  font-size: 1.1em;
  line-height: 1;
  min-width: auto;
}
.btn-remove:hover { transform: scale(1.1); }

.unified-gallery { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 20px; margin-bottom: 20px; background: var(--bg-secondary); padding: 15px; border-radius: 0.5rem; border: 1px dashed var(--border-color); }
.attachment-card-wrapper { display: flex; flex-direction: column; gap: 4px; }
.attachment-card { background: #e0e7ff; color: #4338ca; padding: 8px 12px; border-radius: 0.375rem; text-decoration: none; font-size: 0.85em; font-weight: bold; }
.attachment-card:hover { background: #c7d2fe; }
.attachment-meta { font-size: 0.7em; color: var(--text-tertiary); padding-left: 2px; }

.comment-actions { display: flex; justify-content: space-between; width: 100%; align-items: center; margin-top: 10px; }
.file-input-sm { display: none; }
.btn-attach { cursor: pointer; background: var(--bg-tertiary); padding: 8px 12px; border-radius: 0.375rem; font-size: 0.85em; font-weight: bold; color: var(--text-primary); }
.btn-attach:hover { background: var(--border-color); }

@media (max-width: 640px) {
  .page-container { padding: 1.5rem 1rem; }
  .issue-card { padding: 20px; }
}
</style>
