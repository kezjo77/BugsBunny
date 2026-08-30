// src/config.js
// Central place for the backend API base URL.
// Falls back to the same backend URL P2 was hardcoding everywhere,
// but allows overriding via VITE_API_URL (e.g. in .env) without touching the backend.
export const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'
