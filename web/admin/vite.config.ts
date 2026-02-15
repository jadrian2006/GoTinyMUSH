import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [preact(), tailwindcss()],
  base: '/admin/',
  build: {
    outDir: '../../pkg/admin/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/admin/api': {
        target: 'http://localhost:8443',
        changeOrigin: true,
      },
    },
  },
})
