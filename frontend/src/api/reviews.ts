import { get, post } from './client'
import type { CreateReviewInput } from '../types'

export const createReview = (routeId: string, input: CreateReviewInput) =>
  post<{ id: string }>(`/routes/${routeId}/reviews`, input)

export const getMyReviewsForRoute = (routeId: string) =>
  get<{ target_user_id: string }[]>(`/routes/${routeId}/reviews/my`)
