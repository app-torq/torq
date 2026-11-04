import { createHash } from 'node:crypto'
import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join, relative, sep } from 'node:path'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

/**
 * Fills the service worker's shell list and build hash after a production
 * build (specs/004-installable-pwa, research W1). The hash covers every
 * shell file, so each build yields a byte-different sw.js that browsers
 * install as an update, and an unchanged build an identical one.
 */
function serviceWorkerShell(): Plugin {
  return {
    name: 'torq-sw-shell',
    apply: 'build',
    closeBundle() {
      const dist = join(import.meta.dirname, 'dist')
      const files = (readdirSync(dist, { recursive: true, withFileTypes: true }))
        .filter((e) => e.isFile())
        .map((e) => '/' + relative(dist, join(e.parentPath, e.name)).split(sep).join('/'))
        .filter((p) => p !== '/sw.js' && p !== '/.gitkeep')
        .sort()
      const hash = createHash('sha256')
      for (const f of files) hash.update(f).update(readFileSync(join(dist, f)))
      const swPath = join(dist, 'sw.js')
      const sw = readFileSync(swPath, 'utf8')
      if (!sw.includes('__TORQ_SHELL__') || !sw.includes('__TORQ_BUILD__')) {
        throw new Error('dist/sw.js lacks the __TORQ_SHELL__/__TORQ_BUILD__ placeholders')
      }
      writeFileSync(swPath, sw
        .replace('__TORQ_SHELL__', JSON.stringify(files))
        .replace('__TORQ_BUILD__', JSON.stringify(hash.digest('hex').slice(0, 12))))
    },
  }
}

export default defineConfig({
  plugins: [react(), tailwindcss(), serviceWorkerShell()],
  server: {
    // changeOrigin must stay false: the API's CSRF guard compares the Origin
    // header with Host, and the string shorthand would rewrite Host to :8080.
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: false },
      '/healthz': { target: 'http://localhost:8080', changeOrigin: false },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
