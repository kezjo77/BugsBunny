<!-- src/App.vue -->
<script setup>
import { ref } from 'vue'

// Check if a token exists
const isAuthenticated = ref(!!localStorage.getItem('bunny_token'))
const role = ref(localStorage.getItem('bunny_role'))
const userEmail = ref(localStorage.getItem('bunny_email'))

// P3: light/dark theme support
const theme = ref(localStorage.getItem('bunny_theme') || 'light')

const applyTheme = (mode) => {
  const nextMode = mode === 'dark' ? 'dark' : 'light'
  document.documentElement.setAttribute('data-theme', nextMode)
  document.documentElement.style.colorScheme = nextMode
  localStorage.setItem('bunny_theme', nextMode)
  theme.value = nextMode
}

const toggleTheme = () => {
  applyTheme(theme.value === 'dark' ? 'light' : 'dark')
}

applyTheme(theme.value)

const logout = () => {
  localStorage.removeItem('bunny_token')
  localStorage.removeItem('bunny_role')
  localStorage.removeItem('bunny_email')
  window.location.href = '/login' // Hard reload to clear state
}
</script>

<template>
  <div id="app">
    <nav class="top-nav">
      <div class="logo">Bugsbunny 🥕</div>

      <!-- Only show navigation if logged in -->
      <div v-if="isAuthenticated" class="nav-links">
        <router-link to="/">Dashboard</router-link>
        <router-link to="/queue">Issue Queue</router-link>

        <!-- Admin Console Link -->
        <router-link v-if="role === 'admin'" to="/admin" style="color: #fbbf24; margin-left: 10px;">🛡️ Admin</router-link>

        <!-- P3: routed Profile page instead of the old "coming soon" popup -->
        <router-link to="/profile" class="user-profile" style="text-decoration: none;">
          <span class="avatar">👤</span>
          <span class="email">{{ userEmail }}</span>
        </router-link>

        <!-- P3: theme toggle -->
        <button type="button" class="theme-toggle" @click="toggleTheme">
          {{ theme === 'dark' ? '☀️ Light' : '🌙 Dark' }}
        </button>

        <button @click="logout" class="logout-btn">Log Out</button>
      </div>
      <button v-else type="button" class="theme-toggle theme-toggle-inline" @click="toggleTheme">
        {{ theme === 'dark' ? '☀️ Light' : '🌙 Dark' }}
      </button>
    </nav>

    <router-view />
  </div>
</template>

<style>
/* P3: theme variables */
:root {
  --app-bg: #f4f4f5;
  --page-text: #111827;
  --nav-bg: #18181b;
  --nav-text: #ffffff;
  --nav-link: #a1a1aa;
  --nav-border: #3f3f46;
}

html[data-theme='light'] {
  --app-bg: #f4f4f5;
  --page-text: #111827;
}

html[data-theme='dark'] {
  --app-bg: #111827;
  --page-text: #f3f4f6;
  --nav-bg: #0f172a;
  --nav-text: #f3f4f6;
  --nav-link: #cbd5e1;
  --nav-border: #334155;
}

/* Global styles for the app shell */
body { margin: 0; font-family: system-ui, sans-serif; background: var(--app-bg); color: var(--page-text); }
.top-nav { background: var(--nav-bg); color: var(--nav-text); padding: 15px 40px; display: flex; justify-content: space-between; align-items: center; }
.logo { font-weight: bold; font-size: 1.2em; }
.nav-links { display: flex; gap: 20px; align-items: center; }
.nav-links a { color: var(--nav-link); text-decoration: none; font-weight: 500; transition: color 0.2s; }
.nav-links a:hover, .nav-links a.router-link-active { color: var(--nav-text); }
.admin-link { color: #fbbf24 !important; margin-left: 10px; }
.user-profile { display: flex; align-items: center; gap: 8px; margin-left: auto; padding-left: 20px; border-left: 1px solid var(--nav-border); cursor: pointer; transition: opacity 0.2s; }
.user-profile:hover { opacity: 0.8; }
.avatar { font-size: 1.2em; }
.email { color: #e4e4e7; font-size: 0.9em; font-weight: 500; }
.theme-toggle, .logout-btn {
  background: transparent;
  color: var(--nav-text);
  border: 1px solid var(--nav-border);
  border-radius: 999px;
  padding: 8px 12px;
  font-weight: 600;
  cursor: pointer;
}
.theme-toggle-inline { margin-left: auto; }
</style>
