import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/movies': 'http://localhost:8080',
      '/sessions': 'http://localhost:8080',
      '/bookings': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/catalog': 'http://localhost:8080',
      '/payments': 'http://localhost:8080',
      '/tickets': 'http://localhost:8080',
      '/demo': 'http://localhost:8080',
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
})
