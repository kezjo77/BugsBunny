<!-- src/views/LoginView.vue -->
<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { API_URL } from '@/config'

const router = useRouter()
const email = ref('admin@bugsbunny.local') // Defaulting to make testing fast
const password = ref('admin123')
const errorMsg = ref('')
const isLoggingIn = ref(false)

const handleLogin = async () => {
  isLoggingIn.value = true
  errorMsg.value = ''

  try {
    const response = await fetch(`${API_URL}/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email.value, password: password.value })
    })

    if (!response.ok) {
      throw new Error('Invalid email or password')
    }

    const data = await response.json()
    
    // Save the JWT token and Role to the browser
    localStorage.setItem('bunny_token', data.token)
    localStorage.setItem('bunny_role', data.role)
    localStorage.setItem('bunny_email', data.email)

    // Force a hard reload to update the App shell navigation, 
    // then route to the queue
    window.location.href = '/queue'
    
  } catch (error) {
    errorMsg.value = error.message
  } finally {
    isLoggingIn.value = false
  }
}
</script>

<template>
  <div class="login-wrapper">
    <div class="login-container">
      <div class="login-card">
        <!-- Branding -->
        <div class="login-header">
          <div class="app-logo">🐰</div>
          <h1>Bugsbunny</h1>
          <p class="tagline">Issue Tracking Made Simple</p>
        </div>

        <!-- Sign In Form -->
        <form @submit.prevent="handleLogin" class="login-form">
          <div class="form-group">
            <label for="email-input">Email Address</label>
            <input 
              id="email-input"
              type="email" 
              v-model="email" 
              required 
              class="form-input"
              placeholder="you@example.com"
              autocomplete="email"
            />
          </div>
          
          <div class="form-group">
            <label for="password-input">Password</label>
            <input 
              id="password-input"
              type="password" 
              v-model="password" 
              required 
              class="form-input"
              placeholder="••••••••"
              autocomplete="current-password"
            />
          </div>

          <!-- Error Message -->
          <div v-if="errorMsg" class="error-message" role="alert">
            <span class="error-icon">⚠️</span>
            {{ errorMsg }}
          </div>

          <!-- Submit Button -->
          <button type="submit" :disabled="isLoggingIn" class="btn btn-login">
            <span v-if="isLoggingIn" class="spinner-mini"></span>
            {{ isLoggingIn ? 'Authenticating...' : 'Sign In' }}
          </button>
        </form>

        <!-- Demo Credentials Hint -->
        <div class="demo-hint">
          <p class="hint-text">Demo account: <code>admin@bugsbunny.local</code></p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Root Variables and Theming */
:root {
  --bg-primary: #ffffff;
  --bg-secondary: #f8f9fa;
  --text-primary: #1f2937;
  --text-secondary: #6b7280;
  --text-tertiary: #9ca3af;
  --border-color: #e5e7eb;
  --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.07), 0 2px 4px -1px rgba(0, 0, 0, 0.05);
  --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
  --color-blue: #3b82f6;
  --color-blue-dark: #2563eb;
  --color-red: #ef4444;
  --color-red-light: #fee2e2;
}

[data-theme="dark"] {
  --bg-primary: #1f2937;
  --bg-secondary: #111827;
  --text-primary: #f3f4f6;
  --text-secondary: #d1d5db;
  --text-tertiary: #9ca3af;
  --border-color: #374151;
  --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.3), 0 2px 4px -1px rgba(0, 0, 0, 0.2);
  --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.4), 0 4px 6px -2px rgba(0, 0, 0, 0.3);
  --color-red-light: #7f1d1d;
}

* {
  box-sizing: border-box;
}

body {
  background: var(--bg-primary);
}

/* Login Wrapper - Full Screen */
.login-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  padding: 1rem;
  background: linear-gradient(135deg, var(--bg-secondary) 0%, var(--bg-primary) 100%);
}

/* Login Container */
.login-container {
  width: 100%;
  max-width: 420px;
}

/* Login Card */
.login-card {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 0.875rem;
  padding: 2.5rem 2rem;
  box-shadow: var(--shadow-lg);
}

/* Header */
.login-header {
  text-align: center;
  margin-bottom: 2rem;
}

.app-logo {
  font-size: 2.5rem;
  margin-bottom: 1rem;
  display: block;
}

.login-header h1 {
  margin: 0 0 0.5rem 0;
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

.tagline {
  margin: 0;
  font-size: 0.9375rem;
  color: var(--text-secondary);
  font-weight: 400;
}

/* Form */
.login-form {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
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
  padding: 0.75rem 1rem;
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

/* Error Message */
.error-message {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.875rem 1rem;
  background: var(--color-red-light);
  color: #991b1b;
  border-radius: 0.5rem;
  font-size: 0.875rem;
  border: 1px solid #fca5a5;
}

[data-theme="dark"] .error-message {
  color: #fca5a5;
  border-color: #7f1d1d;
}

.error-icon {
  font-size: 1rem;
  flex-shrink: 0;
}

/* Button */
.btn {
  padding: 0.75rem 1rem;
  border: none;
  border-radius: 0.5rem;
  font-size: 0.9375rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  font-family: inherit;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
}

.btn-login {
  background: var(--color-blue);
  color: white;
  padding: 0.875rem 1rem;
  font-size: 1rem;
}

.btn-login:hover:not(:disabled) {
  background: var(--color-blue-dark);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.btn-login:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* Mini Spinner */
.spinner-mini {
  display: inline-block;
  width: 1rem;
  height: 1rem;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* Demo Hint */
.demo-hint {
  margin-top: 1.5rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--border-color);
  text-align: center;
}

.hint-text {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--text-secondary);
}

.hint-text code {
  display: inline-block;
  background: var(--bg-secondary);
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  font-family: 'Monaco', 'Courier New', monospace;
  color: var(--text-primary);
  font-weight: 500;
  margin: 0 0.25rem;
}

/* Responsive */
@media (max-width: 480px) {
  .login-card {
    padding: 2rem 1.5rem;
  }

  .login-header h1 {
    font-size: 1.5rem;
  }

  .app-logo {
    font-size: 2rem;
    margin-bottom: 0.75rem;
  }
}
</style>
