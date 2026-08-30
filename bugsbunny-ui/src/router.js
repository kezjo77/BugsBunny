// src/router.js
import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from './views/DashboardView.vue'
import IssueView from './views/IssueView.vue'
import QueueView from './views/QueueView.vue'
import LoginView from './views/LoginView.vue'
import AdminView from './views/AdminView.vue'
import ProfileView from './views/ProfileView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: LoginView },
    { path: '/', component: DashboardView },
    { path: '/queue', component: QueueView },
    { path: '/admin', component: AdminView, meta: { requiresAuth: true } },
    { path: '/issue/:id', component: IssueView }, // The :id makes this dynamic!
    { path: '/profile', name: 'Profile', component: ProfileView, meta: { requiresAuth: true } }
  ]
})

// The Global Guard
router.beforeEach((to, from, next) => {
  const isAuthenticated = !!localStorage.getItem('bunny_token')

  if (to.meta.requiresAuth && !isAuthenticated) {
    next('/login')
  } else if (to.path === '/login' && isAuthenticated) {
    next('/queue')
  } else {
    next()
  }
})

export default router
