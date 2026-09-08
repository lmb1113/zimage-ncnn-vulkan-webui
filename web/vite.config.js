import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:19777',
      '/media': 'http://127.0.0.1:19777',
      '/uploads': 'http://127.0.0.1:19777',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
