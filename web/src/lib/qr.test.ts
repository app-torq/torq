import { describe, expect, it } from 'vitest'
import { isLocalAddress, qrMatrix, qrPath } from './qr'

describe('qrMatrix', () => {
  const m = qrMatrix('https://torq.example.com/vehicles/42')

  it('is a square QR symbol with a four-module quiet zone', () => {
    const inner = m.length - 8
    // Symbols are 21, 25, 29, … modules wide (version 1, 2, 3, …).
    expect((inner - 21) % 4).toBe(0)
    for (const row of m) expect(row).toHaveLength(m.length)
    expect(m.slice(0, 4).flat().some(Boolean)).toBe(false)
    expect(m.map((row) => row[0]).some(Boolean)).toBe(false)
  })

  it('has the three finder patterns in their corners', () => {
    const n = m.length
    for (const [y, x] of [[4, 4], [4, n - 5], [n - 5, 4]] as const) {
      expect(m[y]?.[x]).toBe(true) // outer ring
      expect(m[y + 1 * (y === 4 ? 1 : -1)]?.[x + (x === 4 ? 1 : -1)]).toBe(false) // white ring
      expect(m[y + 3 * (y === 4 ? 1 : -1)]?.[x + 3 * (x === 4 ? 1 : -1)]).toBe(true) // centre
    }
  })

  it('draws one square per dark module', () => {
    const dark = m.flat().filter(Boolean).length
    expect(qrPath(m).match(/M/g)).toHaveLength(dark)
  })
})

describe('isLocalAddress', () => {
  it('flags addresses a phone cannot reach', () => {
    for (const url of ['http://localhost:8080/vehicles/1', 'http://127.0.0.1:5173/x', 'http://[::1]/x', 'http://torq.localhost/x']) {
      expect(isLocalAddress(url)).toBe(true)
    }
    for (const url of ['https://torq.example.com/vehicles/1', 'http://192.168.1.20:8080/vehicles/1']) {
      expect(isLocalAddress(url)).toBe(false)
    }
  })
})
