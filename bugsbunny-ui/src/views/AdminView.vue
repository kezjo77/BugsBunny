<!-- src/views/AdminView.vue -->
<script setup>
import { ref, onMounted } from 'vue'
import { API_URL } from '@/config'

const projectForm = ref({ name: '', project_key: '', description: '' })
const userForm = ref({ email: '', password: '', role: 'developer' })
const message = ref('')

const users = ref([])
const projects = ref([])

const fetchUsers = async () => {
  const token = localStorage.getItem('bunny_token')
  const response = await fetch(`${API_URL}/admin/users`, {
    headers: { 'Authorization': `Bearer ${token}` }
  })
  if (response.ok) users.value = await response.json()
}

const fetchProjects = async () => {
  const token = localStorage.getItem('bunny_token')
  const response = await fetch(`${API_URL}/admin/projects`, {
    headers: { 'Authorization': `Bearer ${token}` }
  })
  if (response.ok) projects.value = await response.json()
}

// Load data when the page opens
onMounted(() => {
  fetchUsers()
  fetchProjects()
})

const toggleUserStatus = async (id) => {
  const token = localStorage.getItem('bunny_token')
  await fetch(`${API_URL}/admin/users/${id}/toggle`, {
    method: 'PUT',
    headers: { 'Authorization': `Bearer ${token}` }
  })
  fetchUsers()
}

const resetPassword = async (id) => {
  const newPassword = prompt("Enter new password for this user:")
  if (!newPassword) return

  const token = localStorage.getItem('bunny_token')
  await fetch(`${API_URL}/admin/users/${id}/password`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ password: newPassword })
  })
  alert("Password updated successfully!")
}

const archiveProject = async (id) => {
  if (!confirm("Are you sure you want to archive this project? It will no longer accept new bugs.")) return
  
  const token = localStorage.getItem('bunny_token')
  await fetch(`${API_URL}/admin/projects/${id}/archive`, {
    method: 'PUT',
    headers: { 'Authorization': `Bearer ${token}` }
  })
  fetchProjects()
}

const createProject = async () => {
  const token = localStorage.getItem('bunny_token')
  const res = await fetch(`${API_URL}/projects`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify(projectForm.value)
  })
  if (res.ok) {
    message.value = `Project ${projectForm.value.project_key} created successfully!`
    projectForm.value = { name: '', project_key: '', description: '' }
    await fetchProjects() 
    
    setTimeout(() => message.value = '', 3000)
  }
}

const createUser = async () => {
  const token = localStorage.getItem('bunny_token')
  const res = await fetch(`${API_URL}/users`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify(userForm.value)
  })
  if (res.ok) {
    message.value = `User ${userForm.value.email} created successfully!`
    userForm.value = { email: '', password: '', role: 'developer' }
    await fetchUsers() 
    
    setTimeout(() => message.value = '', 3000)
  }
}
</script>

<template>
  <main class="page-container">
    <!-- Page Header -->
    <div class="page-header">
      <div class="header-content">
        <h1 class="page-title">Platform Administration</h1>
        <p class="page-subtitle">Manage users, projects, and platform settings</p>
      </div>
    </div>

    <!-- Success Message -->
    <div v-if="message" class="alert alert-success">{{ message }}</div>

    <!-- Admin Grid -->
    <div class="admin-grid">
      <!-- Create Project Card -->
      <section class="admin-card">
        <h2 class="card-title">Create New Project</h2>
        <form @submit.prevent="createProject" class="admin-form">
          <div class="form-group">
            <label for="project-name">Project Name</label>
            <input 
              id="project-name"
              v-model="projectForm.name" 
              type="text"
              placeholder="e.g. Mobile App" 
              required 
              class="form-input"
            />
          </div>
          
          <div class="form-group">
            <label for="project-key">Project Key (Prefix)</label>
            <input 
              id="project-key"
              v-model="projectForm.project_key" 
              type="text"
              placeholder="e.g. MOB" 
              required 
              maxlength="10" 
              class="form-input"
              style="text-transform: uppercase;"
            />
          </div>
          
          <div class="form-group">
            <label for="project-desc">Description</label>
            <textarea 
              id="project-desc"
              v-model="projectForm.description" 
              rows="2"
              class="form-input"
            ></textarea>
          </div>
          
          <button type="submit" class="btn btn-primary">Create Project</button>
        </form>
      </section>

      <!-- Create User Card -->
      <section class="admin-card">
        <h2 class="card-title">Invite User</h2>
        <form @submit.prevent="createUser" class="admin-form">
          <div class="form-group">
            <label for="user-email">Email Address</label>
            <input 
              id="user-email"
              type="email" 
              v-model="userForm.email" 
              required 
              class="form-input"
            />
          </div>
          
          <div class="form-group">
            <label for="user-password">Temporary Password</label>
            <input 
              id="user-password"
              type="password" 
              v-model="userForm.password" 
              required 
              class="form-input"
            />
          </div>
          
          <div class="form-group">
            <label for="user-role">Platform Role</label>
            <select id="user-role" v-model="userForm.role" class="form-input">
              <option value="developer">Developer</option>
              <option value="manager">Manager</option>
              <option value="admin">Administrator</option>
            </select>
          </div>
          
          <button type="submit" class="btn btn-primary">Create User</button>
        </form>
      </section>
    </div>

    <!-- Manage Projects Section -->
    <section class="admin-card admin-card-wide">
      <h2 class="card-title">Manage Projects</h2>
      <div v-if="projects.length === 0" class="empty-state">
        <p>No projects yet. Create one to get started.</p>
      </div>
      <div v-else class="admin-table-wrapper">
        <table class="admin-table">
          <thead>
            <tr>
              <th>Key</th>
              <th>Project Name</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="project in projects" :key="project.id">
              <td class="col-key"><strong>{{ project.project_key }}</strong></td>
              <td>{{ project.name }}</td>
              <td>
                <span v-if="project.is_archived" class="badge badge-archived">Archived</span>
                <span v-else class="badge badge-active">Active</span>
              </td>
              <td>
                <button 
                  v-if="!project.is_archived" 
                  @click="archiveProject(project.id)" 
                  class="btn btn-sm btn-danger"
                >
                  Archive
                </button>
                <span v-else class="text-muted">Archived</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- Manage Users Section -->
    <section class="admin-card admin-card-wide">
      <h2 class="card-title">Manage Users</h2>
      <div v-if="users.length === 0" class="empty-state">
        <p>No users found.</p>
      </div>
      <div v-else class="admin-table-wrapper">
        <table class="admin-table">
          <thead>
            <tr>
              <th>Email</th>
              <th>Role</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="user in users" :key="user.id">
              <td>{{ user.email }}</td>
              <td>
                <span class="badge" :class="'badge-' + user.role">
                  {{ user.role }}
                </span>
              </td>
              <td>
                <span v-if="user.is_active === false" class="badge badge-inactive">Inactive</span>
                <span v-else class="badge badge-active">Active</span>
              </td>
              <td>
                <div class="action-buttons">
                  <button 
                    @click="resetPassword(user.id)" 
                    class="btn btn-sm btn-secondary"
                    title="Reset password"
                  >
                    Reset Password
                  </button>
                  <button 
                    @click="toggleUserStatus(user.id)" 
                    class="btn btn-sm"
                    :class="user.is_active !== false ? 'btn-danger' : 'btn-success'"
                  >
                    {{ user.is_active !== false ? 'Deactivate' : 'Reactivate' }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </main>
</template>

<style scoped>
/* Root Variables and Theming */
:root {
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
  --color-red: #ef4444;
  --color-green: #10b981;
  --color-red-light: #fee2e2;
  --color-green-light: #dcfce7;
}

[data-theme="dark"] {
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

/* Alerts */
.alert {
  padding: 1rem 1.25rem;
  border-radius: 0.75rem;
  margin-bottom: 1.5rem;
  font-weight: 500;
}

.alert-success {
  background: var(--color-green-light);
  color: #166534;
  border: 1px solid #a7f3d0;
}

[data-theme="dark"] .alert-success {
  color: #86efac;
  border-color: #059669;
}

/* Admin Grid */
.admin-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
  gap: 1.5rem;
  margin-bottom: 2rem;
}

/* Admin Card */
.admin-card {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 0.75rem;
  padding: 1.5rem;
  box-shadow: var(--shadow-sm);
}

.admin-card-wide {
  grid-column: 1 / -1;
}

.card-title {
  margin: 0 0 1.25rem 0;
  font-size: 1.125rem;
  font-weight: 700;
  color: var(--text-primary);
}

/* Form */
.admin-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.form-group label {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-primary);
}

.form-input {
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 0.9375rem;
  font-family: inherit;
  transition: all 0.2s ease;
}

.form-input:focus {
  outline: none;
  border-color: var(--color-blue);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.form-input::placeholder {
  color: var(--text-tertiary);
}

/* Buttons */
.btn {
  padding: 0.625rem 1rem;
  border: none;
  border-radius: 0.5rem;
  font-size: 0.9375rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  font-family: inherit;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.btn-primary {
  background: var(--color-blue);
  color: white;
}

.btn-primary:hover {
  background: #2563eb;
  opacity: 0.9;
}

.btn-secondary {
  background: var(--bg-secondary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

.btn-secondary:hover {
  background: var(--bg-tertiary);
}

.btn-danger {
  background: var(--color-red-light);
  color: #991b1b;
}

[data-theme="dark"] .btn-danger {
  color: #fca5a5;
}

.btn-danger:hover {
  opacity: 0.8;
}

.btn-success {
  background: var(--color-green-light);
  color: #166534;
}

[data-theme="dark"] .btn-success {
  color: #86efac;
}

.btn-sm {
  padding: 0.5rem 0.75rem;
  font-size: 0.8125rem;
}

/* Table */
.admin-table-wrapper {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

.admin-table {
  width: 100%;
  border-collapse: collapse;
  background: var(--bg-primary);
}

.admin-table thead {
  background: var(--bg-secondary);
  border-bottom: 2px solid var(--border-color);
}

.admin-table th {
  padding: 1rem 0.75rem;
  text-align: left;
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.admin-table td {
  padding: 0.875rem 0.75rem;
  border-bottom: 1px solid var(--border-color);
  color: var(--text-primary);
  vertical-align: middle;
}

.admin-table tbody tr:hover {
  background-color: var(--bg-secondary);
}

.admin-table tbody tr:last-child td {
  border-bottom: none;
}

.col-key {
  font-weight: 600;
  font-size: 0.9375rem;
}

.action-buttons {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
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

.badge-active {
  background: var(--color-green-light);
  color: #166534;
}

[data-theme="dark"] .badge-active {
  color: #86efac;
}

.badge-inactive {
  background: var(--color-red-light);
  color: #991b1b;
}

[data-theme="dark"] .badge-inactive {
  color: #fca5a5;
}

.badge-archived {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}

.badge-developer {
  background: #dbeafe;
  color: #1e40af;
}

[data-theme="dark"] .badge-developer {
  background: #1e3a8a;
  color: #93c5fd;
}

.badge-manager {
  background: #fef3c7;
  color: #b45309;
}

[data-theme="dark"] .badge-manager {
  background: #78350f;
  color: #fcd34d;
}

.badge-admin {
  background: var(--color-red-light);
  color: #991b1b;
}

[data-theme="dark"] .badge-admin {
  color: #fca5a5;
}

/* Empty State */
.empty-state {
  padding: 2rem 1.5rem;
  text-align: center;
  color: var(--text-secondary);
  font-size: 0.9375rem;
}

.text-muted {
  color: var(--text-tertiary);
}

/* Responsive */
@media (max-width: 768px) {
  .page-container {
    padding: 1.5rem 1rem;
  }

  .page-title {
    font-size: 1.5rem;
  }

  .admin-grid {
    grid-template-columns: 1fr;
  }

  .action-buttons {
    flex-direction: column;
  }

  .btn-sm {
    width: 100%;
  }
}
</style>
