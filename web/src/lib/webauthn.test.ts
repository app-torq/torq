import { describe, expect, it } from 'vitest'
import { creationOptions, fromB64url, requestOptions, toB64url } from './webauthn'

describe('webauthn', () => {
  it('round-trips base64url', () => {
    const bytes = new Uint8Array([0, 1, 250, 251, 252, 253, 254, 255])
    const s = toB64url(bytes)
    expect(s).toBe('AAH6-_z9_v8')
    expect(new Uint8Array(fromB64url(s))).toEqual(bytes)
  })

  it('turns the binary option fields into buffers', () => {
    const c = creationOptions({ challenge: 'AAE', rp: { id: 'x' }, user: { id: 'Ag', name: 'anna' }, excludeCredentials: [{ id: 'Aw', type: 'public-key' }] })
    expect(new Uint8Array(c.challenge as ArrayBuffer)).toEqual(new Uint8Array([0, 1]))
    expect(new Uint8Array(c.user.id as ArrayBuffer)).toEqual(new Uint8Array([2]))
    expect(new Uint8Array(c.excludeCredentials?.[0]?.id as ArrayBuffer)).toEqual(new Uint8Array([3]))
    const r = requestOptions({ challenge: 'BA', allowCredentials: [{ id: 'BQ', type: 'public-key' }] })
    expect(new Uint8Array(r.allowCredentials?.[0]?.id as ArrayBuffer)).toEqual(new Uint8Array([5]))
    expect(requestOptions({ challenge: 'BA' }).allowCredentials).toBeUndefined()
  })
})
