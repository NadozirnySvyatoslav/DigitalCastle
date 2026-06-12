import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// У режимі dev (vite на :5173) проксуємо API та медіа на Go-бекенд :8080.
export default defineConfig({
  plugins: [react()],
  base: './',
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/media': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
  },
})
