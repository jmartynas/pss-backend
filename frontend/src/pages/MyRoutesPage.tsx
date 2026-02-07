import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getMyRoutes, deleteRoute } from '../api/routes'
import type { Route } from '../types'
import RouteCard from '../components/RouteCard'
import { ApiError } from '../api/client'

type Filter = '' | 'active' | 'past'

export default function MyRoutesPage() {
  const [routes, setRoutes] = useState<Route[]>([])
  const [filter, setFilter] = useState<Filter>('active')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setLoading(true)
    setError(null)
    getMyRoutes(filter || undefined)
      .then(setRoutes)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [filter])

  const handleDelete = async (id: string) => {
    if (!confirm('Ištrinti šį maršrutą?')) return
    try {
      await deleteRoute(id)
      setRoutes((r) => r.filter((x) => x.id !== id))
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Failed to delete')
    }
  }

  return (
    <div className="max-w-3xl mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Mano maršrutai</h1>
        <Link
          to="/routes/new"
          className="bg-indigo-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-indigo-700"
        >
          + Pasiūlyti kelionę
        </Link>
      </div>

      {/* Filter tabs */}
      <div className="flex gap-2 mb-5">
        {(['active', 'past', ''] as Filter[]).map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={`px-4 py-1.5 rounded-full text-sm font-medium transition ${
              filter === f
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {f === 'active' ? 'Artėjančios' : f === 'past' ? 'Praėjusios' : 'Visos'}
          </button>
        ))}
      </div>

      {error && (
        <div className="bg-red-50 text-red-600 rounded-lg p-4 mb-6 text-sm">{error}</div>
      )}

      {loading ? (
        <p className="text-gray-400 text-sm">Kraunama…</p>
      ) : routes.length === 0 ? (
        <div className="text-center py-16 text-gray-400">
          <div className="text-4xl mb-3"></div>
          <p>Dar nėra maršrutų.</p>
          <Link to="/routes/new" className="text-indigo-600 text-sm mt-2 inline-block">
            Pasiūlykite pirmąją kelionę →
          </Link>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {routes.map((r) => (
            <div key={r.id} className="rounded-xl border border-gray-200 overflow-hidden">
              <RouteCard route={r} />
              <div className="flex gap-2 px-5 py-2 border-t border-gray-100 bg-gray-50">
                {!(r.leaving_at && new Date(r.leaving_at) <= new Date()) && (
                  <Link
                    to={`/routes/${r.id}`}
                    state={{ openEdit: true }}
                    className="text-xs bg-indigo-600 text-white px-3 py-1.5 rounded hover:bg-indigo-700 font-medium"
                  >
                    Redaguoti
                  </Link>
                )}
                <Link
                  to={`/routes/${r.id}`}
                  className="text-xs bg-white border border-gray-200 px-3 py-1.5 rounded hover:bg-gray-50 font-medium text-gray-700"
                >
                  {filter === 'past' || (filter !== 'active' && r.leaving_at && new Date(r.leaving_at) <= new Date()) ? 'Peržiūrėti' : 'Tvarkyti'}
                </Link>
                {!(r.leaving_at && new Date(r.leaving_at) <= new Date()) && (
                  <button
                    onClick={() => handleDelete(r.id)}
                    className="text-xs bg-red-50 text-red-600 border border-red-200 px-3 py-1.5 rounded hover:bg-red-100 font-medium"
                  >
                    Ištrinti
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
