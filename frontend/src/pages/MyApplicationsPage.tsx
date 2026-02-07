import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getMyApplications, cancelApplication } from '../api/applications'
import type { Application } from '../types'
import { ApiError } from '../api/client'

type Tab = 'pending' | 'participating' | 'finished' | 'rejected' | 'left'

const TABS: { id: Tab; label: string }[] = [
  { id: 'pending',       label: 'Laukiama' },
  { id: 'participating', label: 'Dalyvaujama' },
  { id: 'finished',      label: 'Kelionė baigta' },
  { id: 'rejected',      label: 'Atmesta' },
  { id: 'left',          label: 'Palikta' },
]

function tabForApp(app: Application): Tab {
  if (app.status === 'pending') return 'pending'
  if (app.status === 'rejected') return 'rejected'
  if (app.status === 'left') return 'left'
  // approved — check if ride has already departed
  if (app.route_leaving_at && new Date(app.route_leaving_at) < new Date()) return 'finished'
  return 'participating'
}

function fmt(iso?: string) {
  if (!iso) return 'Lanksti'
  return new Date(iso).toLocaleString('lt-LT', { dateStyle: 'medium', timeStyle: 'short' })
}

const statusBadge: Record<string, string> = {
  pending:       'bg-amber-100 text-amber-700',
  approved:      'bg-green-100 text-green-700',
  rejected:      'bg-red-100 text-red-600',
  left:          'bg-gray-100 text-gray-500',
}

const statusLabel: Record<string, string> = {
  pending:  'Laukiama',
  approved: 'Patvirtinta',
  rejected: 'Atmesta',
  left:     'Palikta',
}

type TimeFilter = 'upcoming' | 'past'

function isUpcoming(app: Application) {
  return !app.route_leaving_at || new Date(app.route_leaving_at) > new Date()
}

export default function MyApplicationsPage() {
  const [apps, setApps] = useState<Application[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState(false)
  const [timeFilter, setTimeFilter] = useState<TimeFilter>('upcoming')
  const [activeTab, setActiveTab] = useState<Tab>('pending')

  useEffect(() => {
    getMyApplications()
      .then(setApps)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  const handleCancel = async (appId: string) => {
    const app = apps.find((a) => a.id === appId)
    if (!app) return
    if (!confirm('Atšaukti šią paraišką?')) return
    setActionLoading(true)
    try {
      await cancelApplication(app.route_id, appId)
      // After cancelling an approved app it becomes 'left'; pending/rejected are removed
      if (app.status === 'approved') {
        setApps(prev => prev.map(a => a.id === appId ? { ...a, status: 'left' as const } : a))
      } else {
        setApps(prev => prev.filter(a => a.id !== appId))
      }
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Failed to cancel')
    } finally {
      setActionLoading(false)
    }
  }

  const timeFiltered = apps.filter(app =>
    timeFilter === 'upcoming' ? isUpcoming(app) : !isUpcoming(app)
  )

  const tabCounts = timeFiltered.reduce<Record<Tab, number>>((acc, app) => {
    acc[tabForApp(app)]++
    return acc
  }, { pending: 0, participating: 0, finished: 0, rejected: 0, left: 0 })

  const visible = timeFiltered.filter(app => tabForApp(app) === activeTab)

  return (
    <div className="max-w-3xl mx-auto px-4 py-8">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Mano kelionės</h1>

      {error && (
        <div className="bg-red-50 text-red-600 rounded-lg p-4 mb-6 text-sm">{error}</div>
      )}

      {/* Time filter */}
      <div className="flex gap-2 mb-4">
        {(['upcoming', 'past'] as TimeFilter[]).map(f => (
          <button
            key={f}
            onClick={() => setTimeFilter(f)}
            className={`px-4 py-1.5 rounded-full text-sm font-medium transition ${
              timeFilter === f
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {f === 'upcoming' ? 'Artėjančios' : 'Praėjusios'}
          </button>
        ))}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 border-b border-gray-200">
        {TABS.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-4 py-2 text-sm font-medium rounded-t-lg transition ${
              activeTab === tab.id
                ? 'text-indigo-600 border-b-2 border-indigo-600 -mb-px'
                : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            {tab.label}
            {tabCounts[tab.id] > 0 && (
              <span className={`ml-1.5 text-xs px-1.5 py-0.5 rounded-full font-semibold ${
                activeTab === tab.id ? 'bg-indigo-100 text-indigo-600' : 'bg-gray-100 text-gray-500'
              }`}>
                {tabCounts[tab.id]}
              </span>
            )}
          </button>
        ))}
      </div>

      {loading ? (
        <p className="text-gray-400 text-sm">Kraunama…</p>
      ) : visible.length === 0 ? (
        <div className="text-center py-16 text-gray-400">
          <p className="text-sm">Čia kelionių nėra.</p>
          {activeTab === 'pending' && (
            <Link to="/" className="text-indigo-600 text-sm mt-2 inline-block">
              Rasti kelionę →
            </Link>
          )}
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {visible.map(app => (
            <div key={app.id} className="rounded-xl border border-gray-200 overflow-hidden">
              {/* Card body */}
              <div className="bg-white px-5 py-4 flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  {(app.route_start_address || app.route_end_address) && (
                    <div className="flex items-center gap-1.5 text-sm font-medium text-gray-800 mb-1 flex-wrap">
                      <span className="truncate">{app.route_start_address ?? '—'}</span>
                      <span className="text-gray-400">→</span>
                      <span className="truncate">{app.route_end_address ?? '—'}</span>
                    </div>
                  )}
                  <div className="text-xs text-gray-400">
                    {fmt(app.route_leaving_at)} · Pateikta {new Date(app.created_at).toLocaleDateString('lt-LT')}
                  </div>
                  {app.comment && (
                    <p className="text-sm text-gray-500 mt-1 italic">"{app.comment}"</p>
                  )}
                </div>
                <span className={`text-xs px-2 py-0.5 rounded-full font-medium flex-shrink-0 ${statusBadge[app.status] ?? 'bg-gray-100 text-gray-600'}`}>
                  {statusLabel[app.status] ?? app.status}
                </span>
              </div>

              {/* Button footer */}
              <div className="flex items-center gap-2 px-5 py-2 border-t border-gray-100 bg-gray-50 flex-wrap">
                {app.status === 'approved' && !(app.route_leaving_at && new Date(app.route_leaving_at) <= new Date()) && (
                  app.pending_stop_change ? (
                    <span className="text-xs bg-amber-100 text-amber-700 px-2.5 py-1 rounded font-medium">
                      Stotelių pakeitimas laukia patvirtinimo
                    </span>
                  ) : (
                    <Link
                      to={`/routes/${app.route_id}`}
                      state={{ openStopChange: true }}
                      className="text-xs bg-indigo-600 text-white px-3 py-1.5 rounded hover:bg-indigo-700 font-medium"
                    >
                      Kurti prašymą keisti kelionės informaciją
                    </Link>
                  )
                )}
                <Link
                  to={`/routes/${app.route_id}`}
                  className="text-xs bg-white border border-gray-200 px-3 py-1.5 rounded hover:bg-gray-50 font-medium text-gray-700"
                >
                  Peržiūrėti
                </Link>
                {(app.status === 'pending' || app.status === 'approved') && !(app.route_leaving_at && new Date(app.route_leaving_at) <= new Date()) && (
                  <button
                    onClick={() => handleCancel(app.id)}
                    disabled={actionLoading}
                    className="text-xs bg-red-50 text-red-600 border border-red-200 px-3 py-1.5 rounded hover:bg-red-100 font-medium disabled:opacity-50"
                  >
                    {app.status === 'approved' ? 'Išeiti' : 'Atšaukti'}
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
