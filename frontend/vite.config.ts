import {defineConfig, type Plugin} from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// When a fixed prefix is baked in (VITE_BASE_PATH, e.g. for serving dist/ from
// nginx without the Go backend), index.html has to advertise it too — the app
// reads the base path from these tags, not from import.meta.env. The backend
// rewrites the very same tags at request time for the default relative build.
function basePathTags(basePath: string): Plugin {
  const normalized = basePath.replace(/\/+$/, '')
  return {
    name: 'garage-ui-base-path-tags',
    transformIndexHtml(html) {
      return html
        .replace(/<base[^>]*\shref\s*=\s*"[^"]*"[^>]*>/i, `<base href="${normalized}/">`)
        .replace(
          /<meta[^>]*\sname\s*=\s*"garage-ui-base-path"[^>]*>/i,
          `<meta name="garage-ui-base-path" content="${normalized}">`,
        )
    },
  }
}

const envBasePath = process.env.VITE_BASE_PATH || ''

// https://vite.dev/config/
export default defineConfig({
  plugins: envBasePath ? [react(), basePathTags(envBasePath)] : [react()],
  // Relative asset URLs keep the build deployment-agnostic: the backend
  // injects <base href="/prefix/"> into index.html at request time, so the
  // same image serves any subpath (issue #107). Set VITE_BASE_PATH to bake a
  // fixed prefix in instead (e.g. when serving the dist/ folder from nginx).
  base: envBasePath || './',
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/auth': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
  },
})
