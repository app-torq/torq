// Thin fetch wrapper for the torq API (contracts/rest-api.md).

export interface ApiErrorBody {
  code: string
  fields?: Record<string, string>
  params?: Record<string, unknown>
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>
  readonly params: Record<string, unknown>

  constructor(status: number, body: ApiErrorBody) {
    super(body.code)
    this.status = status
    this.code = body.code
    this.fields = body.fields ?? {}
    this.params = body.params ?? {}
  }
}

export const API_BASE = '/api/v1'
export const UNAUTHENTICATED_EVENT = 'torq:unauthenticated'

type Query = Record<string, string | number | boolean | null | undefined | readonly (string | number)[]>

export function withQuery(path: string, query?: Query): string {
  if (!query) return path
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue
    if (Array.isArray(value)) {
      for (const v of value) params.append(key, String(v))
    } else {
      params.set(key, String(value))
    }
  }
  const qs = params.toString()
  return qs ? `${path}?${qs}` : path
}

async function parseError(res: Response): Promise<ApiError> {
  let body: ApiErrorBody = { code: res.status >= 500 ? 'internal' : 'request.failed' }
  try {
    const json = (await res.json()) as { error?: ApiErrorBody }
    if (json.error?.code) body = json.error
  } catch {
    // Non-JSON error body; keep the generic code.
  }
  if (res.status === 401) window.dispatchEvent(new Event(UNAUTHENTICATED_EVENT))
  return new ApiError(res.status, body)
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (method !== 'GET') headers['X-Torq-Request'] = '1'
  const init: RequestInit = { method, headers, credentials: 'same-origin' }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  let res: Response
  try {
    res = await fetch(API_BASE + path, init)
  } catch {
    throw new ApiError(0, { code: 'network.unreachable' })
  }
  if (!res.ok) throw await parseError(res)
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

/** Uploads a single file with progress reporting (XMLHttpRequest supports upload progress). */
function upload<T>(
  method: 'POST' | 'PUT',
  path: string,
  file: File,
  fields: Record<string, string> = {},
  onProgress?: (fraction: number) => void,
): Promise<T> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    // Text fields first: the server streams the file part and reads fields that precede it.
    for (const [k, v] of Object.entries(fields)) form.append(k, v)
    form.append('file', file)
    const xhr = new XMLHttpRequest()
    xhr.open(method, API_BASE + path)
    xhr.setRequestHeader('X-Torq-Request', '1')
    xhr.setRequestHeader('Accept', 'application/json')
    xhr.withCredentials = true
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) onProgress(e.loaded / e.total)
    }
    xhr.onerror = () => { reject(new ApiError(0, { code: 'network.unreachable' })) }
    xhr.onload = () => {
      let json: unknown
      try {
        json = xhr.responseText ? JSON.parse(xhr.responseText) : undefined
      } catch {
        json = undefined
      }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(json as T)
        return
      }
      const errBody = (json as { error?: ApiErrorBody } | undefined)?.error ?? { code: 'request.failed' }
      if (xhr.status === 401) window.dispatchEvent(new Event(UNAUTHENTICATED_EVENT))
      reject(new ApiError(xhr.status, errBody))
    }
    xhr.send(form)
  })
}

export const api = {
  get: <T>(path: string, query?: Query) => request<T>('GET', withQuery(path, query)),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body ?? {}),
  patch: <T>(path: string, body?: unknown) => request<T>('PATCH', path, body ?? {}),
  del: <T = void>(path: string, body?: unknown) => request<T>('DELETE', path, body),
  upload,
}

export function isApiError(err: unknown, code?: string): err is ApiError {
  return err instanceof ApiError && (code === undefined || err.code === code)
}
