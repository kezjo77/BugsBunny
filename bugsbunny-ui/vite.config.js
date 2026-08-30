import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'
import { fileURLToPath, URL } from 'node:url'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      // '@' maps to the 'src' directory
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  }
})
