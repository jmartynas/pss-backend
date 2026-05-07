import { Link } from 'react-router-dom'
import type { Application } from '../types'

const statusColors: Record<string, string> = {
  pending: 'bg-yellow-100 text-yellow-700',
  approved: 'bg-green-100 text-green-700',
  rejected: 'bg-red-100 text-red-600',
}

interface Props {
  app: Application
  onApprove?: (appId: string) => void
  onReject?: (appId: string) => void
  onCancel?: (appId: string) => void
  onApproveStopChange?: (appId: string) => void
  onRejectStopChange?: (appId: string) => void
  onToggleExpand?: (appId: string) => void
  expanded?: boolean
  loading?: boolean
}

export default function ApplicationCard({
  app,
  onApprove,
  onReject,
  onCancel,
  onApproveStopChange,
  onRejectStopChange,
  onToggleExpand,
  expanded,
  loading,
}: Props) {
  const canExpand = !!onToggleExpand

  return (
    <div className="bg-white rounded-xl border border-gray-200 p-4">
      <div className="flex items-center justify-between gap-4">
        <div>
          <Link to={`/users/${app.user_id}`} className="font-medium text-gray-800 hover:text-indigo-600 hover:underline">{app.user_name}</Link>
          {app.comment && (
            <p className="text-sm text-gray-500 mt-0.5">{app.comment}</p>
          )}
          <div className="text-xs text-gray-400 mt-1">
            {new Date(app.created_at).toLocaleDateString('lt-LT')}
          </div>
        </div>

        <div className="flex items-center gap-2 flex-shrink-0">
          <span
            className={`text-xs px-2 py-0.5 rounded-full font-medium ${statusColors[app.status] ?? 'bg-gray-100 text-gray-600'}`}
          >
            {app.status}
          </span>

          {app.status === 'pending' && onApprove && onReject && (
            <>
              <button
                onClick={() => onApprove(app.id)}
                disabled={loading}
                className="text-sm bg-green-600 text-white px-3 py-1 rounded-md hover:bg-green-700 disabled:opacity-50"
              >
                Patvirtinti
              </button>
              <button
                onClick={() => onReject(app.id)}
                disabled={loading}
                className="text-sm bg-red-600 text-white px-3 py-1 rounded-md hover:bg-red-700 disabled:opacity-50"
              >
                Atmesti
              </button>
            </>
          )}

          {app.status === 'approved' && app.pending_stop_change && (
            <>
              <span className="text-xs text-amber-600 font-medium">Stotelių pakeitimas</span>
              {onApproveStopChange && onRejectStopChange && (
                <>
                  <button
                    onClick={() => onApproveStopChange(app.id)}
                    disabled={loading}
                    className="text-sm bg-green-600 text-white px-3 py-1 rounded-md hover:bg-green-700 disabled:opacity-50"
                  >
                    Patvirtinti
                  </button>
                  <button
                    onClick={() => onRejectStopChange(app.id)}
                    disabled={loading}
                    className="text-sm bg-red-600 text-white px-3 py-1 rounded-md hover:bg-red-700 disabled:opacity-50"
                  >
                    Atmesti
                  </button>
                </>
              )}
            </>
          )}

          {canExpand && (
            <button
              onClick={() => onToggleExpand(app.id)}
              disabled={loading}
              className={`text-sm px-3 py-1 rounded-md border font-medium transition ${
                expanded
                  ? 'bg-gray-100 text-gray-700 border-gray-300'
                  : 'bg-white text-gray-600 border-gray-300 hover:bg-gray-50'
              }`}
            >
              {expanded ? 'Slėpti' : 'Peržiūrėti'}
            </button>
          )}

          {app.status === 'pending' && onCancel && (
            <button
              onClick={() => onCancel(app.id)}
              disabled={loading}
              className="text-sm text-red-500 hover:text-red-700 disabled:opacity-50"
            >
              Atšaukti
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
