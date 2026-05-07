import { Link } from 'react-router-dom'
import type { Route } from '../types'

function formatDate(iso?: string) {
  if (!iso) return 'Lanksti'
  return new Date(iso).toLocaleString('lt-LT', {
    dateStyle: 'medium',
    timeStyle: 'short',
  })
}

interface RouteCardProps {
  route: Route
  showBadge?: boolean
  linkState?: unknown
}

export default function RouteCard({ route, showBadge, linkState }: RouteCardProps) {
  const from =
    route.start_formatted_address ??
    `${route.start_lat.toFixed(4)}, ${route.start_lng.toFixed(4)}`
  const to =
    route.end_formatted_address ??
    `${route.end_lat.toFixed(4)}, ${route.end_lng.toFixed(4)}`

  return (
    <Link
      to={`/routes/${route.id}`}
      state={linkState}
      className="block bg-white rounded-xl border border-gray-200 p-5 hover:border-indigo-400 hover:shadow-sm transition"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 text-sm text-gray-500 mb-1">
            <span className="font-medium text-gray-800 truncate">{from}</span>
            <span className="text-gray-400">→</span>
            <span className="font-medium text-gray-800 truncate">{to}</span>
          </div>

          <div className="text-sm text-gray-500 mt-1 flex items-center gap-1.5 flex-wrap">
            <span>{formatDate(route.leaving_at)}</span>
            <span className="text-gray-400">·</span>
            <Link to={`/users/${route.creator_id}`} className="text-gray-700 hover:text-indigo-600 hover:underline" onClick={e => e.stopPropagation()}>{route.creator_name}</Link>
            {route.creator_rating != null && (
              <span className="flex items-center gap-0.5 text-gray-600 text-xs font-medium">
                <span>{route.creator_rating.toFixed(1)}/5</span>
                <span className="text-gray-400 font-normal">({route.creator_review_count})</span>
              </span>
            )}
          </div>

          {route.description && (
            <p className="text-sm text-gray-500 mt-1 line-clamp-1">
              {route.description}
            </p>
          )}
        </div>

        <div className="text-right flex-shrink-0">
          {route.price != null ? (
            <div className="text-lg font-semibold text-indigo-600">
              €{route.price.toFixed(2)}
            </div>
          ) : (
            <div className="text-sm text-gray-400">Nemokama</div>
          )}
          <div className="text-xs text-gray-400 mt-0.5">
            {route.available_passengers}/{route.max_passengers} laisvos vietos
          </div>
          {showBadge && route.available_passengers === 0 && (
            <span className="inline-block mt-1 text-xs bg-red-100 text-red-600 px-2 py-0.5 rounded-full">
              Pilna
            </span>
          )}
        </div>
      </div>
    </Link>
  )
}
