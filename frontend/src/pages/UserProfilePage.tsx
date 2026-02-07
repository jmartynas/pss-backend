import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getUserProfile } from '../api/auth'
import type { UserProfile } from '../types'

function StarRating({ rating }: { rating: number }) {
  return (
    <span className="text-sm text-gray-700">
      {rating}/5
    </span>
  )
}

export default function UserProfilePage() {
  const { id } = useParams<{ id: string }>()
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    getUserProfile(id)
      .then(setProfile)
      .catch(() => setError('User not found'))
      .finally(() => setLoading(false))
  }, [id])

  if (loading) return (
    <div className="flex items-center justify-center min-h-screen text-gray-400">Kraunama…</div>
  )

  if (error || !profile) return (
    <div className="max-w-lg mx-auto px-4 py-16 text-center text-gray-500">{error ?? 'Vartotojas nerastas'}</div>
  )

  const avgRating = profile.reviews.length
    ? (profile.reviews.reduce((s, r) => s + r.rating, 0) / profile.reviews.length).toFixed(1)
    : null

  return (
    <div className="max-w-2xl mx-auto px-4 py-8 space-y-6">
      {/* Header */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6">
        <h1 className="text-2xl font-bold text-gray-900 mb-1">
          {profile.name || 'Anonymous'}
        </h1>
        <p className="text-sm text-gray-500">{profile.email}</p>
        {profile.status === 'blocked' && (
          <span className="inline-block mt-2 text-xs bg-red-50 text-red-600 px-2 py-0.5 rounded-full">
            Paskyra išjungta
          </span>
        )}
        {avgRating && (
          <div className="mt-3 flex items-center gap-2">
            <StarRating rating={Math.round(Number(avgRating))} />
            <span className="text-sm text-gray-500">
              {avgRating} · {profile.reviews.length} {profile.reviews.length !== 1 ? 'atsiliepimai' : 'atsiliepimas'}
            </span>
          </div>
        )}
      </div>

      {/* Reviews */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6">
        <h2 className="font-semibold text-gray-900 mb-4">Atsiliepimai</h2>
        {profile.reviews.length === 0 ? (
          <p className="text-sm text-gray-400">Dar nėra atsiliepimų.</p>
        ) : (
          <ul className="space-y-4">
            {profile.reviews.map((rv) => (
              <li key={rv.id} className="border-b border-gray-100 pb-4 last:border-0 last:pb-0">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm font-medium text-gray-800">{rv.author_name}</span>
                  <StarRating rating={rv.rating} />
                </div>
                {rv.comment && <p className="text-sm text-gray-600">{rv.comment}</p>}
                <p className="text-xs text-gray-400 mt-1">{new Date(rv.created_at).toLocaleDateString('lt-LT')}</p>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
