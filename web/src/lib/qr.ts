// QR codes for printed labels (#38), encoded in the browser so they work
// offline. The value is any URL: a vehicle page now, a public read-only
// link later.
import { encode } from 'uqr'

/** Modules of white border around the code, as the QR standard asks for. */
const QUIET_ZONE = 4

/** The QR code of value as rows of modules (true = dark), border included. */
export function qrMatrix(value: string): boolean[][] {
  // Error correction M (15 %) survives a label that got a bit dirty in the engine bay.
  return encode(value, { ecc: 'M', border: QUIET_ZONE }).data
}

/** One SVG path drawing every dark module of m as a 1×1 square. */
export function qrPath(m: boolean[][]): string {
  let d = ''
  m.forEach((row, y) => {
    row.forEach((dark, x) => {
      if (dark) d += `M${String(x)} ${String(y)}h1v1h-1z`
    })
  })
  return d
}

/** A PNG of value with modulePx pixels per module, for downloading. */
export function qrPng(value: string, modulePx = 16): Promise<Blob> {
  const m = qrMatrix(value)
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = m.length * modulePx
  const ctx = canvas.getContext('2d')
  if (!ctx) return Promise.reject(new Error('canvas unavailable'))
  ctx.fillStyle = '#fff'
  ctx.fillRect(0, 0, canvas.width, canvas.height)
  ctx.fillStyle = '#000'
  m.forEach((row, y) => {
    row.forEach((dark, x) => {
      if (dark) ctx.fillRect(x * modulePx, y * modulePx, modulePx, modulePx)
    })
  })
  return new Promise((resolve, reject) => {
    canvas.toBlob((b) => { if (b) resolve(b); else reject(new Error('PNG encoding failed')) }, 'image/png')
  })
}

/** Whether a phone that scans a code for url can't reach it (an address of this computer only). */
export function isLocalAddress(url: string): boolean {
  const host = new URL(url).hostname
  return host === 'localhost' || host === '::1' || host === '[::1]' || host.startsWith('127.') || host.endsWith('.localhost')
}
