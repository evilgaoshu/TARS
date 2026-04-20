import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.ico', 'apple-touch-icon.png', 'mask-icon.svg'],
      manifest: {
        name: 'TARS Web Console',
        short_name: 'TARS',
        description: 'TARS Incident Recovery Gateway Console',
        theme_color: '#101820',
        icons: [
          { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' }
        ]
      }
    })
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    target: 'esnext',
    cssCodeSplit: true,
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            // Group framework core
            if (id.includes('react-dom') || id.includes('react-router')) return 'vendor-framework';
            // Group UI/Icons
            if (id.includes('lucide-react')) return 'vendor-ui';
            // Specific heavy libraries that are lazy-loaded
            if (id.includes('swagger-ui-react')) return 'vendor-swagger';
            if (id.includes('@orama')) return 'vendor-search';
            if (id.includes('react-markdown')) return 'vendor-markdown';
          }
        }
      },
      onwarn(warning, warn) {
        if (warning.code === 'CIRCULAR_DEPENDENCY') {
          if (warning.message.includes('src/')) {
            throw new Error(`CRITICAL: Circular dependency detected in project source: ${warning.message}`);
          }
        }
        warn(warning);
      },
    },
  },
})
