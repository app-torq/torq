// Renders the app icons from public/favicon.svg with Playwright's Chromium,
// so no image library is needed (specs/004-installable-pwa research W9).
// The PNGs are committed; run `npm run icons` after changing the logo.
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { chromium } from '@playwright/test'

const publicDir = join(import.meta.dirname, '..', 'public')
const favicon = readFileSync(join(publicDir, 'favicon.svg'), 'utf8')
// The bare mark without the squircle frame, for icons the system masks itself
// (Android's maskable shapes, iOS's rounded corners).
const mark = favicon.replace(/<rect[^>]*\/>/g, '')

// scale is the share of the icon the SVG fills; the maskable icon keeps the
// mark inside the safe zone so no icon shape cuts it.
const icons = [
  { file: 'icons/icon-192.png', size: 192, scale: 1, svg: favicon },
  { file: 'icons/icon-512.png', size: 512, scale: 1, svg: favicon },
  { file: 'icons/icon-maskable-512.png', size: 512, scale: 0.85, svg: mark },
  { file: 'apple-touch-icon.png', size: 180, scale: 0.85, svg: mark },
]

const browser = await chromium.launch()
try {
  for (const { file, size, scale, svg } of icons) {
    const page = await browser.newPage({ viewport: { width: size, height: size } })
    const inner = Math.round(size * scale)
    await page.setContent(
      `<body style="margin:0;width:${size}px;height:${size}px;background:#09090b;display:grid;place-items:center">` +
        `<div style="width:${inner}px;height:${inner}px">${svg}</div></body>`,
    )
    await page.screenshot({ path: join(publicDir, file) })
    await page.close()
    console.log(`${file} ${size}×${size}`)
  }
} finally {
  await browser.close()
}
