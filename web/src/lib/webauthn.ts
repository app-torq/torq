/**
 * WebAuthn in the browser. The server sends options as JSON (binary values in
 * base64url); the browser API wants ArrayBuffers and returns them. Newer
 * browsers convert on their own (parse…FromJSON, toJSON); this covers the rest
 * without a dependency.
 */

type Json = Record<string, unknown>

export function fromB64url(s: string): ArrayBuffer {
  const b64 = s.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (s.length % 4)) % 4)
  const bin = atob(b64)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out.buffer
}

export function toB64url(buf: ArrayBuffer | ArrayBufferView): string {
  const bytes = buf instanceof ArrayBuffer ? new Uint8Array(buf) : new Uint8Array(buf.buffer, buf.byteOffset, buf.byteLength)
  let bin = ''
  for (const b of bytes) bin += String.fromCharCode(b)
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function withIds(list: unknown): unknown {
  return Array.isArray(list) ? list.map((c: Json) => ({ ...c, id: fromB64url(c.id as string) })) : list
}

/** Creation options from the server for navigator.credentials.create. */
export function creationOptions(json: Json): PublicKeyCredentialCreationOptions {
  const user = json.user as Json
  return {
    ...json,
    challenge: fromB64url(json.challenge as string),
    user: { ...user, id: fromB64url(user.id as string) },
    excludeCredentials: withIds(json.excludeCredentials),
  } as unknown as PublicKeyCredentialCreationOptions
}

/** Request options from the server for navigator.credentials.get. */
export function requestOptions(json: Json): PublicKeyCredentialRequestOptions {
  return {
    ...json,
    challenge: fromB64url(json.challenge as string),
    allowCredentials: withIds(json.allowCredentials),
  } as unknown as PublicKeyCredentialRequestOptions
}

/** A credential as the JSON the server expects. */
export function credentialJSON(cred: PublicKeyCredential): Json {
  // Registration and sign-in return different response types.
  const r = cred.response as Partial<AuthenticatorAttestationResponse> & Partial<AuthenticatorAssertionResponse> & AuthenticatorResponse
  const response: Json = { clientDataJSON: toB64url(r.clientDataJSON) }
  if (r.attestationObject) {
    response.attestationObject = toB64url(r.attestationObject)
    if (r.getTransports) response.transports = r.getTransports()
  }
  if (r.authenticatorData && r.signature) {
    response.authenticatorData = toB64url(r.authenticatorData)
    response.signature = toB64url(r.signature)
    if (r.userHandle) response.userHandle = toB64url(r.userHandle)
  }
  return { id: cred.id, rawId: toB64url(cred.rawId), type: cred.type, response, clientExtensionResults: cred.getClientExtensionResults() }
}

/** Whether this browser can use passkeys at all. */
export function passkeysSupported(): boolean {
  return typeof window !== 'undefined' && typeof window.PublicKeyCredential === 'function' && !!navigator.credentials
}

/** Creates a passkey from the server's {publicKey} options. */
export async function createPasskey(options: { publicKey: Json }): Promise<Json> {
  const cred = await navigator.credentials.create({ publicKey: creationOptions(options.publicKey) })
  if (!cred) throw new Error('no credential')
  return credentialJSON(cred as PublicKeyCredential)
}

/** Signs the server's {publicKey} request options with a passkey. */
export async function getPasskey(options: { publicKey: Json }): Promise<Json> {
  const cred = await navigator.credentials.get({ publicKey: requestOptions(options.publicKey) })
  if (!cred) throw new Error('no credential')
  return credentialJSON(cred as PublicKeyCredential)
}
