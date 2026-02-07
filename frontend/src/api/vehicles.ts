import { get, post, patch, del } from './client'
import type { Vehicle } from '../types'

export const listMyVehicles = (): Promise<Vehicle[]> =>
  get<Vehicle[]>('/vehicles/my')

export const createVehicle = (data: {
  make?: string
  model: string
  plate_number: string
  seats: number
}): Promise<{ id: string }> =>
  post<{ id: string }>('/vehicles', data)

export const updateVehicle = (id: string, data: {
  make?: string
  model: string
  plate_number: string
  seats: number
}): Promise<void> =>
  patch<void>(`/vehicles/${id}`, data)

export const deleteVehicle = (id: string): Promise<void> =>
  del<void>(`/vehicles/${id}`)
