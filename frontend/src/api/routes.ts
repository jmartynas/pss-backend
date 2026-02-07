import { get, post, patch, del } from './client'
import type {
  Route,
  CreateRouteInput,
  UpdateRouteInput,
  SearchRouteInput,
} from '../types'

export const getRoute = (id: string) => get<Route>(`/routes/${id}`)

export const searchRoutes = (input: SearchRouteInput) =>
  post<Route[]>('/routes/search', input)

export const createRoute = (input: CreateRouteInput) =>
  post<{ id: string }>('/routes', input)

export const updateRoute = (id: string, input: UpdateRouteInput) =>
  patch<Route>(`/routes/${id}`, input)

export const deleteRoute = (id: string) => del<void>(`/routes/${id}`)

export const getMyRoutes = (filter?: 'active' | 'past') => {
  const q = filter ? `?filter=${filter}` : ''
  return get<Route[]>(`/routes/my${q}`)
}

export const getParticipatedRoutes = (filter?: 'active' | 'past') => {
  const q = filter ? `?filter=${filter}` : ''
  return get<Route[]>(`/routes/participated${q}`)
}
