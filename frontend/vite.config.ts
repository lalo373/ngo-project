import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const BACKEND = 'http://localhost:5001'

const proxyPrefixes = [
  '/login',
  '/logout',
  '/me',
  '/dailyPayout',
  '/weeklyPayouts',
  '/calendar',
  '/admin',
]

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../static',
    emptyOutDir: true,
  },
  server: {
    proxy: Object.fromEntries(
      proxyPrefixes.map((p) => [p, { target: BACKEND, changeOrigin: true }]),
    ),
  },
})
