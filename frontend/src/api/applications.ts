import { get, post, patch, del } from './client'
import type { Application, ApplyInput } from '../types'

export const applyToRoute = (routeId: string, input: ApplyInput) =>
  post<{ id: string }>(`/routes/${routeId}/applications`, input)

export const getRouteApplications = (routeId: string) =>
  get<Application[]>(`/routes/${routeId}/applications`)

export const reviewApplication = (
  routeId: string,
  appId: string,
  status: 'approved' | 'rejected'
) => patch<void>(`/routes/${routeId}/applications/${appId}`, { status })

export const cancelApplication = (routeId: string, appId: string) =>
  del<void>(`/routes/${routeId}/applications/${appId}`)

export const getMyApplicationForRoute = (routeId: string) =>
  get<Application>(`/routes/${routeId}/applications/my`)

export const updateMyApplication = (
  routeId: string,
  appId: string,
  stops: Array<{ position: number; lat: number; lng: number; place_id?: string; formatted_address?: string; route_stop_id?: string }>,
  comment?: string
) => patch<void>(`/routes/${routeId}/applications/${appId}/stops`, { stops, comment: comment ?? null })

export const getMyApplications = () => get<Application[]>('/applications/my')

export const requestStopChange = (
  routeId: string,
  appId: string,
  stops: Array<{ position: number; lat: number; lng: number; place_id?: string; formatted_address?: string; route_stop_id?: string }>,
  comment?: string
) => post<void>(`/routes/${routeId}/applications/${appId}/stop-change`, { stops, comment: comment || undefined })

export const reviewStopChange = (
  routeId: string,
  appId: string,
  approve: boolean
) => patch<void>(`/routes/${routeId}/applications/${appId}/stop-change`, { approve })

export const cancelStopChange = (routeId: string, appId: string) =>
  del<void>(`/routes/${routeId}/applications/${appId}/stop-change`)
