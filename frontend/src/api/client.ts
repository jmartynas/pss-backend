const BASE = ''

let refreshing: Promise<boolean> | null = null

async function tryRefresh(): Promise<boolean> {
  if (refreshing) return refreshing
  refreshing = fetch(BASE + '/auth/refresh', { method: 'POST', credentials: 'include' })
    .then(r => r.ok)
    .catch(() => false)
    .finally(() => { refreshing = null })
  return refreshing
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const opts: RequestInit = {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(options.headers ?? {}) },
    ...options,
  }

  let res = await fetch(BASE + path, opts)

  if (res.status === 401) {
    const ok = await tryRefresh()
    if (ok) {
      res = await fetch(BASE + path, opts)
    } else {
      throw new ApiError(401, 'Session expired')
    }
  }

  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new ApiError(res.status, text.trim())
  }

  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export class ApiError extends Error {
  readonly status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'ApiError'
  }
}

export const get = <T>(path: string) => request<T>(path, { method: 'GET' })
export const post = <T>(path: string, body?: unknown) =>
  request<T>(path, { method: 'POST', body: JSON.stringify(body) })
export const patch = <T>(path: string, body?: unknown) =>
  request<T>(path, { method: 'PATCH', body: JSON.stringify(body) })
export const del = <T>(path: string) => request<T>(path, { method: 'DELETE' })
