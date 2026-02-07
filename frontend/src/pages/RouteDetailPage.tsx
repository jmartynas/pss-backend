import { useEffect, useRef, useState } from 'react'
import { useParams, useLocation } from 'react-router-dom'
import type { PlaceResult } from '../components/PlaceInput'
import PlaceInput from '../components/PlaceInput'
import { getRoute, updateRoute } from '../api/routes'
import {
  applyToRoute,
  getRouteApplications,
  getMyApplicationForRoute,
  reviewApplication,
  cancelApplication,
  updateMyApplication,
  requestStopChange,
  reviewStopChange,
  cancelStopChange,
} from '../api/applications'
import type { Route, Application } from '../types'
import ApplicationCard from '../components/ApplicationCard'
import RouteMap, { type StopEntry } from '../components/RouteMap'
import { useAuth } from '../context/AuthContext'
import { ApiError } from '../api/client'

function fmt(iso?: string) {
  if (!iso) return 'Lankstus išvykimas'
  return new Date(iso).toLocaleString('lt-LT', { dateStyle: 'medium', timeStyle: 'short' })
}


export default function RouteDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { user } = useAuth()
  const location = useLocation()
  const searchCtx = location.state as {
    searchFrom?: PlaceResult
    searchTo?: PlaceResult
    searchStops?: StopEntry[]
    openEdit?: boolean
    openStopChange?: boolean
  } | null
  const autoOpenedStopChange = useRef(false)

  // Combined stop list for the map (driver's locked + search overlay unlocked).
  // Initialized once when the route loads so full reordering is preserved.
  const [mapStops, setMapStops] = useState<StopEntry[]>([])
  const overlayStops: StopEntry[] = (() => {
    const stops: StopEntry[] = []
    if (searchCtx?.searchFrom)
      stops.push({ address: `Išvykimas: ${searchCtx.searchFrom.address}`, lat: searchCtx.searchFrom.lat, lng: searchCtx.searchFrom.lng, locked: false })
    for (const s of searchCtx?.searchStops ?? [])
      stops.push({ ...s, locked: false })
    if (searchCtx?.searchTo)
      stops.push({ address: `Paskirties vieta: ${searchCtx.searchTo.address}`, lat: searchCtx.searchTo.lat, lng: searchCtx.searchTo.lng, locked: false })
    return stops
  })()

  const [route, setRoute] = useState<Route | null>(null)
  const [applications, setApplications] = useState<Application[]>([])
  const [myApplication, setMyApplication] = useState<Application | null | undefined>(undefined) // undefined = loading
  const [loadingRoute, setLoadingRoute] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState(false)

  const [applyComment, setApplyComment] = useState('')

  const [expandedAppId, setExpandedAppId] = useState<string | null>(null)
  const [changingStops, setChangingStops] = useState(false)
  const [changeAllStops, setChangeAllStops] = useState<StopEntry[]>([])
  const [changeComment, setChangeComment] = useState('')

  const [editingApplication, setEditingApplication] = useState(false)
  const [editAppAllStops, setEditAppAllStops] = useState<StopEntry[]>([])
  const [editAppComment, setEditAppComment] = useState('')

  const [editingRoute, setEditingRoute] = useState(false)
  const [editForm, setEditForm] = useState<{ description: string; max_passengers?: number; max_deviation?: number; price?: number; leaving_at: string }>({
    description: '', leaving_at: '',
  })
  const [editStart, setEditStart] = useState<PlaceResult | null>(null)
  const [editEnd, setEditEnd] = useState<PlaceResult | null>(null)
  const [editStops, setEditStops] = useState<StopEntry[]>([])

  const openEditWith = (r: Route) => {
    setEditForm({
      description: r.description ?? '',
      max_passengers: r.max_passengers,
      max_deviation: r.max_deviation ?? undefined,
      price: r.price ?? undefined,
      leaving_at: r.leaving_at ? r.leaving_at.slice(0, 16) : '',
    })
    setEditStart({ address: r.start_formatted_address ?? '', lat: r.start_lat, lng: r.start_lng, placeId: r.start_place_id ?? undefined })
    setEditEnd({ address: r.end_formatted_address ?? '', lat: r.end_lat, lng: r.end_lng, placeId: r.end_place_id ?? undefined })
    setEditStops(r.stops.map(s => ({
      address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
      lat: s.lat, lng: s.lng, placeId: s.place_id ?? undefined, locked: false,
    })))
    setEditingRoute(true)
  }

  const openEdit = () => { if (route) openEditWith(route) }

  const handleUpdate = async () => {
    if (!id || !editStart || !editEnd) return
    setActionLoading(true)
    try {
      const updated = await updateRoute(id, {
        description: editForm.description || undefined,
        max_passengers: editForm.max_passengers,
        max_deviation: editForm.max_deviation,
        price: editForm.price,
        leaving_at: editForm.leaving_at ? new Date(editForm.leaving_at).toISOString() : undefined,
        start_lat: editStart.lat,
        start_lng: editStart.lng,
        start_place_id: editStart.placeId,
        start_formatted_address: editStart.address,
        end_lat: editEnd.lat,
        end_lng: editEnd.lng,
        end_place_id: editEnd.placeId,
        end_formatted_address: editEnd.address,
        stops: editStops.map(s => ({ lat: s.lat, lng: s.lng, place_id: s.placeId, formatted_address: s.address })),
      })
      setRoute(updated)
      setMapStops(updated.stops.map(s => ({
        address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
        lat: s.lat, lng: s.lng, locked: true,
      })))
      setEditingRoute(false)
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Failed to update route')
    } finally {
      setActionLoading(false)
    }
  }

const isCreator = user && route && user.id === route.creator_id

  useEffect(() => {
    if (!id) return
    setLoadingRoute(true)
    getRoute(id)
      .then(r => {
        setRoute(r)
        const driverStops = r.stops.map(s => ({
          address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
          lat: s.lat,
          lng: s.lng,
          locked: true,
          routeStopId: s.id,
        }))
        setMapStops([...driverStops, ...overlayStops])
        if (searchCtx?.openEdit) openEditWith(r)
      })
      .catch(() => setError('Route not found'))
      .finally(() => setLoadingRoute(false))
  }, [id]) // eslint-disable-line react-hooks/exhaustive-deps

  // Fetch current user's own application for this route
  useEffect(() => {
    if (!id || !user) { setMyApplication(null); return }
    getMyApplicationForRoute(id)
      .then(app => setMyApplication(app))
      .catch(() => setMyApplication(null)) // 404 → no application
  }, [id, user])

  const openStopChangePanel = (app: typeof myApplication) => {
    if (!app || app.status !== 'approved' || !route) return
    if (app.pending_stop_change && app.stops.length > 0) {
      // Proposed order stored in full. Determine locked by cross-referencing route.stops:
      // new stops (no route_stop_id) and own existing stops are unlocked; others are locked.
      setChangeAllStops(app.stops.map(s => {
        const routeStop = s.route_stop_id ? route.stops.find(rs => rs.id === s.route_stop_id) : undefined
        const isOwn = !s.route_stop_id || routeStop?.participant_id === app.id
        return {
          address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
          lat: s.lat, lng: s.lng, placeId: s.place_id,
          locked: !isOwn,
          routeStopId: s.route_stop_id,
        }
      }))
    } else {
      // Fresh: all route stops shown in natural order. All carry routeStopId.
      // Context stops (not own) are locked so they can't be removed/address-edited.
      setChangeAllStops(route.stops.map(s => ({
        address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
        lat: s.lat, lng: s.lng, placeId: s.place_id,
        locked: s.participant_id !== app.id,
        routeStopId: s.id,
      })))
    }
    setChangingStops(true)
  }

  // Auto-open stop-change panel when navigated here with openStopChange: true
  useEffect(() => {
    if (!searchCtx?.openStopChange) return
    if (autoOpenedStopChange.current) return
    if (!route || myApplication === undefined) return
    if (myApplication?.status !== 'approved') return
    autoOpenedStopChange.current = true
    openStopChangePanel(myApplication)
  }, [route, myApplication]) // eslint-disable-line react-hooks/exhaustive-deps

  // Fetch all applications (creator or approved participant)
  const canViewApplications = isCreator || myApplication?.status === 'approved'
  useEffect(() => {
    if (!id || !canViewApplications) return
    getRouteApplications(id)
      .then(setApplications)
      .catch(() => {})
  }, [id, canViewApplications])

  const handleApply = async () => {
    if (!id) return
    setActionLoading(true)
    try {
      await applyToRoute(id, {
        comment: applyComment || undefined,
        stops: mapStops.map((s, i) => ({
          position: i,
          lat: s.lat,
          lng: s.lng,
          place_id: s.placeId,
          formatted_address: s.address,
          route_stop_id: s.routeStopId,
        })),
      })
      const app = await getMyApplicationForRoute(id)
      setMyApplication(app)
      setApplyComment('')
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Failed to apply')
    } finally {
      setActionLoading(false)
    }
  }

const handleCancelMyApplication = async () => {
    if (!id || !myApplication) return
    if (!confirm('Cancel your request?')) return
    setActionLoading(true)
    try {
      await cancelApplication(id, myApplication.id)
      setMyApplication(null)
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Failed to cancel')
    } finally {
      setActionLoading(false)
    }
  }

  const openApplicationEdit = (app: NonNullable<typeof myApplication>) => {
    if (app.stops.length > 0) {
      // Full proposed order is stored — use it directly.
      // Stops with a route_stop_id are context stops (locked); own new stops are unlocked.
      setEditAppAllStops(app.stops.map(s => ({
        address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
        lat: s.lat, lng: s.lng, placeId: s.place_id,
        locked: !!s.route_stop_id,
        routeStopId: s.route_stop_id,
      })))
    } else {
      // No stops stored yet — show all route stops locked so user can add own and reorder.
      setEditAppAllStops((route?.stops ?? []).map(s => ({
        address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
        lat: s.lat, lng: s.lng, placeId: s.place_id,
        locked: true,
        routeStopId: s.id,
      })))
    }
    setEditAppComment(app.comment ?? '')
    setEditingApplication(true)
  }

  const handleSaveApplication = async () => {
    if (!id || !myApplication) return
    setActionLoading(true)
    try {
      await updateMyApplication(
        id,
        myApplication.id,
        editAppAllStops.map((s, i) => ({ position: i, lat: s.lat, lng: s.lng, place_id: s.placeId, formatted_address: s.address, route_stop_id: s.routeStopId })),
        editAppComment || undefined,
      )
      const updated = await getMyApplicationForRoute(id)
      setMyApplication(updated)
      setEditingApplication(false)
      setEditAppAllStops([])
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Nepavyko išsaugoti pakeitimų')
    } finally {
      setActionLoading(false)
    }
  }

  const handleReview = async (appId: string, status: 'approved' | 'rejected') => {
    if (!id) return
    setActionLoading(true)
    try {
      await reviewApplication(id, appId, status)
      setApplications(prev => prev.map(a => a.id === appId ? { ...a, status } : a))
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Failed')
    } finally {
      setActionLoading(false)
    }
  }

  const handleRequestStopChange = async () => {
    if (!id || !myApplication) return
    setActionLoading(true)
    try {
      await requestStopChange(
        id,
        myApplication.id,
        changeAllStops.map((s, i) => ({
          position: i,
          lat: s.lat, lng: s.lng,
          place_id: s.placeId,
          formatted_address: s.address,
          route_stop_id: s.routeStopId,
        })),
        changeComment || undefined,
      )
      const updated = await getMyApplicationForRoute(id)
      setMyApplication(updated)
      setChangingStops(false)
      setChangeAllStops([])
      setChangeComment('')
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Nepavyko pateikti prašymo')
    } finally {
      setActionLoading(false)
    }
  }

  const handleCancelStopChange = async () => {
    if (!id || !myApplication) return
    if (!confirm('Atšaukti stotelių pakeitimo prašymą?')) return
    setActionLoading(true)
    try {
      await cancelStopChange(id, myApplication.id)
      setMyApplication(prev => prev ? { ...prev, pending_stop_change: false } : prev)
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Nepavyko atšaukti')
    } finally {
      setActionLoading(false)
    }
  }

  const handleReviewStopChange = async (appId: string, approve: boolean) => {
    if (!id) return
    setActionLoading(true)
    try {
      await reviewStopChange(id, appId, approve)
      setExpandedAppId(null)
      setApplications(prev => prev.map(a =>
        a.id === appId ? { ...a, pending_stop_change: false, stops: [] } : a
      ))
      // Reload route to reflect updated stops on the map
      const updated = await import('../api/routes').then(m => m.getRoute(id))
      setRoute(updated)
      setMapStops(updated.stops.map(s => ({
        address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
        lat: s.lat, lng: s.lng, locked: true,
      })))
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Nepavyko')
    } finally {
      setActionLoading(false)
    }
  }


  if (loadingRoute) {
    return <div className="max-w-3xl mx-auto px-4 py-16 text-center text-gray-400">Kraunama…</div>
  }

  if (error || !route) {
    return <div className="max-w-3xl mx-auto px-4 py-16 text-center text-red-500">{error ?? 'Maršrutas nerastas'}</div>
  }

  const from = route.start_formatted_address ?? `${route.start_lat.toFixed(4)}, ${route.start_lng.toFixed(4)}`
  const to = route.end_formatted_address ?? `${route.end_lat.toFixed(4)}, ${route.end_lng.toFixed(4)}`

  const hasStarted = !!route.leaving_at && new Date(route.leaving_at) <= new Date()

  // Show join button only when: logged in, not creator, seats available, no existing application, and not started
  const canApply = user && !isCreator && route.available_passengers > 0 && myApplication === null && !hasStarted

  return (
    <div className="max-w-3xl mx-auto px-4 py-8">
      {/* Route card */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6 mb-6">
        {hasStarted && (
          <div className="mb-4 flex items-center gap-2 text-sm font-medium text-gray-500 bg-gray-50 border border-gray-200 rounded-lg px-3 py-2">
            <span>Kelionė baigta</span>
          </div>
        )}
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1">
            <div className="flex items-center gap-2 text-lg font-bold text-gray-900 flex-wrap">
              <span>{from}</span>
              <span className="text-gray-400 font-normal">→</span>
              <span>{to}</span>
            </div>
            <div className="text-sm text-gray-500 mt-1">
              {fmt(route.leaving_at)} · Vairuotojas: {route.creator_name}
            </div>
            {route.description && (
              <p className="text-sm text-gray-600 mt-2">{route.description}</p>
            )}
          </div>

          <div className="text-right flex-shrink-0 flex flex-col items-end gap-2">
            {route.price != null ? (
              <div className="text-2xl font-bold text-indigo-600">€{route.price.toFixed(2)}</div>
            ) : (
              <div className="text-sm text-gray-400">Nemokama</div>
            )}
            <div className="text-sm text-gray-500">
              {route.available_passengers}/{route.max_passengers} laisvos vietos
            </div>
            {isCreator && !editingRoute && !hasStarted && (
              <button
                onClick={openEdit}
                className="text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 px-3 py-1.5 rounded-lg"
              >
                Redaguoti maršrutą
              </button>
            )}
          </div>
        </div>

        {/* ── Inline edit form (creator only) ──────────────────────────── */}
        {isCreator && editingRoute && (
          <div className="mt-4 pt-4 border-t border-gray-100">
            <h3 className="font-semibold text-gray-900 mb-4">Redaguoti maršrutą</h3>

            {/* ── Route ── */}
            <div className="space-y-3 mb-4">
              <PlaceInput
                label="Išvykimas"
                placeholder="Ieškoti išvykimo adreso…"
                value={editStart?.address}
                onSelect={setEditStart}
              />
              <PlaceInput
                label="Paskirties vieta"
                placeholder="Ieškoti paskirties adreso…"
                value={editEnd?.address}
                onSelect={setEditEnd}
              />
              <RouteMap
                start={editStart}
                end={editEnd}
                onStartChange={setEditStart}
                onEndChange={setEditEnd}
                stops={editStops}
                onStopsChange={setEditStops}
              />
            </div>

            {/* ── Details ── */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div className="sm:col-span-2">
                <label className="block text-xs font-medium text-gray-500 mb-1">Išvykimo laikas</label>
                <input
                  type="datetime-local"
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                  value={editForm.leaving_at ?? ''}
                  onChange={e => setEditForm(f => ({ ...f, leaving_at: e.target.value }))}
                />
              </div>
              <div className="sm:col-span-2">
                <label className="block text-xs font-medium text-gray-500 mb-1">Aprašymas</label>
                <textarea
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-400"
                  rows={2}
                  value={editForm.description ?? ''}
                  onChange={e => setEditForm(f => ({ ...f, description: e.target.value }))}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1">Maks. keleiviai</label>
                <input
                  type="number"
                  min={1}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                  value={editForm.max_passengers ?? ''}
                  onChange={e => setEditForm(f => ({ ...f, max_passengers: e.target.value ? Number(e.target.value) : undefined }))}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1">Kaina (€)</label>
                <input
                  type="number"
                  min={0}
                  step={0.01}
                  placeholder="Nemokama"
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                  value={editForm.price ?? ''}
                  onChange={e => setEditForm(f => ({ ...f, price: e.target.value ? Number(e.target.value) : undefined }))}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1">Maks. nukrypimas (km)</label>
                <input
                  type="number"
                  min={0}
                  step={1}
                  placeholder="Nėra"
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                  value={editForm.max_deviation ?? ''}
                  onChange={e => setEditForm(f => ({ ...f, max_deviation: e.target.value ? Number(e.target.value) : undefined }))}
                />
              </div>
            </div>

            <div className="flex gap-2 mt-4">
              <button
                onClick={handleUpdate}
                disabled={actionLoading}
                className="bg-indigo-600 text-white px-5 py-2 rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
              >
                {actionLoading ? 'Išsaugoma…' : 'Išsaugoti pakeitimus'}
              </button>
              <button
                onClick={() => setEditingRoute(false)}
                disabled={actionLoading}
                className="px-5 py-2 rounded-lg text-sm font-medium text-gray-600 hover:text-gray-800 disabled:opacity-50"
              >
                Atšaukti
              </button>
            </div>
          </div>
        )}

        {/* Participants */}
        {route.participants.length > 0 && (
          <div className="mt-4 pt-4 border-t border-gray-100">
            <div className="text-xs font-medium text-gray-500 mb-2">Keleiviai</div>
            <div className="flex flex-wrap gap-2">
              {route.participants.map((p) => (
                <span key={p.user_id} className="text-sm bg-indigo-50 text-indigo-700 px-2 py-0.5 rounded-full">
                  {p.name}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Route map — hidden while editing or changing stops (those panels have their own map) */}
        {!editingRoute && !changingStops && !editingApplication && <div className="mt-4 pt-4 border-t border-gray-100">
          <RouteMap
            baseRoute={route}
            stops={mapStops}
            onStopsChange={setMapStops}
            readOnly={overlayStops.length === 0}
          />
        </div>}

        {/* ── Inline apply form ─────────────────────────────────────────── */}
        {canApply && (
          <div className="mt-4 pt-4 border-t border-gray-100">
            <h3 className="font-semibold text-gray-900 mb-3">Prašymas prisijungti</h3>
            <textarea
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-400"
              rows={2}
              placeholder="Žinutė vairuotojui (neprivaloma)"
              value={applyComment}
              onChange={(e) => setApplyComment(e.target.value)}
            />
            <div className="mt-3">
              <button
                onClick={handleApply}
                disabled={actionLoading}
                className="bg-indigo-600 text-white px-5 py-2 rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
              >
                {actionLoading ? 'Siunčiama…' : 'Siųsti prašymą'}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* ── Existing application status (passenger view) ──────────────────── */}
      {user && !isCreator && myApplication && myApplication.status !== 'approved' && (
        <div className="bg-white rounded-2xl border border-gray-200 p-6 mb-6">
          <div className="flex items-center justify-between mb-3">
            <h3 className="font-semibold text-gray-900">Jūsų prašymas</h3>
            <div className="flex items-center gap-2">
              {myApplication.status === 'pending' && !hasStarted && !editingApplication && (
                <button
                  onClick={() => openApplicationEdit(myApplication)}
                  className="text-sm bg-indigo-600 text-white px-3 py-1.5 rounded-lg hover:bg-indigo-700"
                >
                  Redaguoti
                </button>
              )}
              <span className={`text-xs font-semibold px-2.5 py-1 rounded-full ${
                myApplication.status === 'rejected' ? 'bg-red-100 text-red-600' : 'bg-amber-100 text-amber-700'
              }`}>
                {myApplication.status === 'rejected' ? 'Atmesta' : 'Laukiama'}
              </span>
            </div>
          </div>

          {!editingApplication && myApplication.comment && (
            <p className="text-sm text-gray-500 mb-3 italic">"{myApplication.comment}"</p>
          )}

          {/* Pending edit form */}
          {myApplication.status === 'pending' && editingApplication && (
            <>
              <RouteMap
                baseRoute={route}
                stops={editAppAllStops}
                onStopsChange={setEditAppAllStops}
              />
              <textarea
                className="mt-3 w-full border border-gray-300 rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-400"
                rows={2}
                placeholder="Žinutė vairuotojui (neprivaloma)"
                value={editAppComment}
                onChange={e => setEditAppComment(e.target.value)}
              />
              <div className="flex gap-2 mt-3">
                <button
                  onClick={handleSaveApplication}
                  disabled={actionLoading}
                  className="bg-indigo-600 text-white px-5 py-2 rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
                >
                  {actionLoading ? 'Išsaugoma…' : 'Išsaugoti pakeitimus'}
                </button>
                <button
                  onClick={() => setEditingApplication(false)}
                  disabled={actionLoading}
                  className="px-5 py-2 rounded-lg text-sm font-medium text-gray-600 hover:text-gray-800 disabled:opacity-50"
                >
                  Atšaukti
                </button>
              </div>
            </>
          )}

          {/* Pending: allow cancel (only before ride starts) */}
          {myApplication.status === 'pending' && !hasStarted && !editingApplication && (
            <div className="flex gap-2">
              <button
                onClick={handleCancelMyApplication}
                disabled={actionLoading}
                className="text-red-500 hover:text-red-700 text-sm px-3 disabled:opacity-50"
              >
                Atšaukti prašymą
              </button>
            </div>
          )}

          {/* Rejected: just show the status, no actions */}
          {myApplication.status === 'rejected' && (
            <p className="text-sm text-gray-400">Vairuotojas atmetė jūsų prašymą.</p>
          )}
        </div>
      )}

      {/* ── Stop change (approved passenger) ─────────────────────────────── */}
      {user && !isCreator && myApplication?.status === 'approved' && !hasStarted && (
        <div className="bg-white rounded-2xl border border-gray-200 p-6 mb-6">
          <div className="flex items-center justify-between mb-3">
            <h3 className="font-semibold text-gray-900">Stotelių pakeitimas</h3>
            {myApplication.pending_stop_change ? (
              <div className="flex items-center gap-2">
                <span className="text-xs bg-amber-100 text-amber-700 px-2.5 py-1 rounded-full font-semibold">
                  Laukia patvirtinimo
                </span>
                {!changingStops && (
                  <button
                    onClick={() => openStopChangePanel(myApplication)}
                    disabled={actionLoading}
                    className="text-sm bg-indigo-600 text-white px-3 py-1.5 rounded-lg hover:bg-indigo-700 disabled:opacity-50"
                  >
                    Redaguoti
                  </button>
                )}
                <button
                  onClick={handleCancelStopChange}
                  disabled={actionLoading}
                  className="text-sm text-red-500 hover:text-red-700 disabled:opacity-50"
                >
                  Atšaukti
                </button>
              </div>
            ) : (
              !changingStops && (
                <button
                  onClick={() => openStopChangePanel(myApplication)}
                  className="text-sm bg-indigo-600 text-white px-3 py-1.5 rounded-lg hover:bg-indigo-700"
                >
                  Keisti stotelę
                </button>
              )
            )}
          </div>

          {changingStops && (
            <>
              <RouteMap
                baseRoute={route}
                stops={changeAllStops}
                onStopsChange={setChangeAllStops}
              />
              <textarea
                className="mt-3 w-full border border-gray-300 rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-400"
                rows={2}
                placeholder="Komentaras vairuotojui (neprivaloma)"
                value={changeComment}
                onChange={e => setChangeComment(e.target.value)}
              />
              <div className="flex gap-2 mt-3">
                <button
                  onClick={handleRequestStopChange}
                  disabled={actionLoading}
                  className="bg-indigo-600 text-white px-5 py-2 rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
                >
                  {actionLoading ? 'Siunčiama…' : 'Pateikti prašymą'}
                </button>
                <button
                  onClick={() => { setChangingStops(false); setChangeAllStops([]); setChangeComment('') }}
                  disabled={actionLoading}
                  className="px-5 py-2 rounded-lg text-sm font-medium text-gray-600 hover:text-gray-800 disabled:opacity-50"
                >
                  Atšaukti
                </button>
              </div>
            </>
          )}
        </div>
      )}

      {/* ── Applications (creator + approved participants) ────────────────── */}
      {canViewApplications && (
        <div>
          <h3 className="font-semibold text-gray-900 mb-3">
            Paraiškos ({applications.length})
          </h3>
          {applications.length === 0 ? (
            <p className="text-sm text-gray-400">Dar nėra paraiškų.</p>
          ) : (
            <div className="flex flex-col gap-3">
              {applications.map((app) => (
                <div key={app.id}>
                  <ApplicationCard
                    app={app}
                    loading={actionLoading}
                    onApprove={isCreator && !hasStarted && app.status === 'pending' ? (appId) => handleReview(appId, 'approved') : undefined}
                    onReject={isCreator && !hasStarted && app.status === 'pending' ? (appId) => handleReview(appId, 'rejected') : undefined}
                    onApproveStopChange={isCreator && !hasStarted ? (appId) => handleReviewStopChange(appId, true) : undefined}
                    onRejectStopChange={isCreator && !hasStarted ? (appId) => handleReviewStopChange(appId, false) : undefined}
                    onToggleExpand={(appId) => setExpandedAppId(prev => prev === appId ? null : appId)}
                    expanded={expandedAppId === app.id}
                  />
                  {expandedAppId === app.id && (
                    <div className={`mt-2 bg-white rounded-xl border p-4 ${app.pending_stop_change ? 'border-amber-200' : 'border-gray-200'}`}>
                      <p className={`text-xs font-medium mb-3 ${app.pending_stop_change ? 'text-amber-700' : 'text-gray-500'}`}>
                        {app.pending_stop_change ? 'Siūlomi nauji sustojimų taškai' : 'Siūloma maršruto tvarka'}
                      </p>
                      {app.comment && (
                        <p className="text-sm text-gray-600 italic mb-3">"{app.comment}"</p>
                      )}
                      <RouteMap
                        baseRoute={route}
                        stops={
                          app.stops.length > 0
                            // Full proposed order stored — locked = context stop (has route_stop_id)
                            ? app.stops.map(s => ({
                                address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
                                lat: s.lat, lng: s.lng, placeId: s.place_id,
                                locked: !!s.route_stop_id,
                              }))
                            // Approved application with no pending change: show route with own stops highlighted
                            : route.stops.map(s => ({
                                address: s.formatted_address ?? `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`,
                                lat: s.lat, lng: s.lng, placeId: s.place_id,
                                locked: s.participant_id !== app.id,
                              }))
                        }
                        onStopsChange={() => {}}
                        readOnly
                      />
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
