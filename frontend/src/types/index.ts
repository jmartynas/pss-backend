export interface Stop {
  id: string
  position: number
  lat: number
  lng: number
  place_id?: string
  formatted_address?: string
  participant_id?: string
}

export interface Participant {
  user_id: string
  name: string
}

export interface Route {
  id: string
  creator_id: string
  creator_name: string
  description?: string
  start_lat: number
  start_lng: number
  start_place_id?: string
  start_formatted_address?: string
  end_lat: number
  end_lng: number
  end_place_id?: string
  end_formatted_address?: string
  max_passengers: number
  max_deviation: number
  available_passengers: number
  price?: number
  leaving_at?: string
  stops: Stop[]
  participants: Participant[]
  creator_rating?: number
  creator_review_count: number
}

export interface ApplicationStop {
  id: string
  position: number
  lat: number
  lng: number
  place_id?: string
  formatted_address?: string
  route_stop_id?: string
}

export interface Application {
  id: string
  user_id: string
  user_name: string
  route_id: string
  status: 'pending' | 'approved' | 'rejected' | 'left'
  comment?: string
  created_at: string
  stops: ApplicationStop[]
  pending_stop_change: boolean
  route_leaving_at?: string
  route_start_address?: string
  route_end_address?: string
}

export interface User {
  id: string
  email: string
  name: string
  provider: string
  status: string
  created_at: string
  updated_at: string
}

export interface Review {
  id: string
  author_id: string
  author_name: string
  rating: number
  comment: string
  created_at: string
}

export interface UserProfile {
  id: string
  email: string
  name: string
  status: string
  created_at: string
  reviews: Review[]
}

export interface SearchRouteInput {
  start_lat: number
  start_lng: number
  end_lat: number
  end_lng: number
  stops: Array<{ lat: number; lng: number }>
}

export interface Vehicle {
  id: string
  user_id: string
  make: string
  model: string
  plate_number: string
  seats: number
  created_at: string
}

export interface CreateRouteInput {
  vehicle_id?: string
  description?: string
  start_lat: number
  start_lng: number
  start_place_id?: string
  start_formatted_address?: string
  end_lat: number
  end_lng: number
  end_place_id?: string
  end_formatted_address?: string
  max_passengers: number
  max_deviation: number
  price?: number
  leaving_at?: string
  stops: Array<{
    lat: number
    lng: number
    place_id?: string
    formatted_address?: string
  }>
}

export interface UpdateRouteInput {
  description?: string
  max_passengers?: number
  max_deviation?: number
  price?: number
  leaving_at?: string
  start_lat?: number
  start_lng?: number
  start_place_id?: string
  start_formatted_address?: string
  end_lat?: number
  end_lng?: number
  end_place_id?: string
  end_formatted_address?: string
  stops?: Array<{ lat: number; lng: number; place_id?: string; formatted_address?: string }>
}

export interface ApplyInput {
  comment?: string
  stops: Array<{
    position: number
    lat: number
    lng: number
    place_id?: string
    formatted_address?: string
    route_stop_id?: string
  }>
}
