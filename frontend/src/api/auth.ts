import { get, post, patch } from './client'
import type { User, UserProfile } from '../types'

export const getMe = () => get<User>('/users/me')

export const updateMe = (name: string) =>
  patch<User>('/users/me', { name })

export const getUserProfile = (id: string) =>
  get<UserProfile>(`/users/${id}`)

export const disableMyAccount = () =>
  post<void>('/users/me/disable')

export const refreshSession = () => post<void>('/auth/refresh')

export const logout = () => get<void>('/auth/logout')

export const loginUrl = (provider: 'google' | 'github' | 'microsoft') =>
  `/auth/${provider}/login`
