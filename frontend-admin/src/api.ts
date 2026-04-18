const BASE = '/api'

function token() {
  return localStorage.getItem('admin_token') ?? ''
}

function authHeaders() {
  return { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: authHeaders(),
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
  return res.json() as Promise<T>
}

export interface LoginResponse {
  token: string
  permissions: number
  admin_id: string
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  const res = await fetch(BASE + '/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
  return res.json() as Promise<LoginResponse>
}

export interface AdminUser {
  id: string
  email: string
  name: string
  status: string
  provider: string
  created_at: string
}

export interface AdminRoute {
  id: string
  from: string
  to: string
  leaving_at: string | null
  created_at: string
  creator: string
  deleted: boolean
}

export interface Admin {
  id: string
  email: string
  permissions: number
  created_at: string
}

export const PERM_USERS = 1
export const PERM_ROUTES = 2
export const PERM_ADMINS = 4

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  await request<unknown>('PATCH', '/me/password', { current_password: currentPassword, new_password: newPassword })
}

export const api = {
  users: {
    list: () => request<AdminUser[]>('GET', '/users'),
    block: (id: string) => request<{ status: string }>('POST', `/users/${id}/block`),
    unblock: (id: string) => request<{ status: string }>('POST', `/users/${id}/unblock`),
  },
  routes: {
    list: () => request<AdminRoute[]>('GET', '/routes'),
    delete: (id: string) => request<{ status: string }>('DELETE', `/routes/${id}`),
  },
  admins: {
    list: () => request<Admin[]>('GET', '/admins'),
    create: (email: string, password: string, permissions: number) =>
      request<{ id: string }>('POST', '/admins', { email, password, permissions }),
    setPermissions: (id: string, permissions: number) =>
      request<{ permissions: number }>('PATCH', `/admins/${id}/permissions`, { permissions }),
    delete: (id: string) => request<{ status: string }>('DELETE', `/admins/${id}`),
  },
}
