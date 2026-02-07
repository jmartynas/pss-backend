import { useEffect, useRef, useState } from 'react'
import { importLibrary } from '@googlemaps/js-api-loader'
import PlaceInput, { type PlaceResult } from './PlaceInput'
import type { Route } from '../types'

// A stop in the route. locked = driver's existing stop (cannot be removed).
export interface StopEntry extends PlaceResult {
  locked?: boolean
  routeStopId?: string
}

type PlacingMode = 'start' | 'stop' | 'end'

interface Props {
  // ── Creation mode ─────────────────────────────────────────────────────────
  start?: PlaceResult | null
  end?: PlaceResult | null
  onStartChange?: (r: PlaceResult) => void
  onEndChange?: (r: PlaceResult) => void

  // ── Join mode ─────────────────────────────────────────────────────────────
  baseRoute?: Route

  // ── Both modes ───────────────────────────────────────────────────────────
  stops: StopEntry[]
  onStopsChange: (stops: StopEntry[]) => void

  // ── Read-only mode ────────────────────────────────────────────────────────
  readOnly?: boolean
}

const API_KEY = import.meta.env.VITE_GOOGLE_MAPS_API_KEY ?? ''
const MAP_ID = import.meta.env.VITE_GOOGLE_MAPS_MAP_ID ?? 'DEMO_MAP_ID'

/** Creates a styled circle div used as AdvancedMarkerElement content. */
function makeCircleDot(size: number, color: string, label?: string): HTMLElement {
  const el = document.createElement('div')
  Object.assign(el.style, {
    width: `${size}px`,
    height: `${size}px`,
    borderRadius: '50%',
    background: color,
    border: '2px solid #fff',
    boxShadow: '0 1px 3px rgba(0,0,0,0.3)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    color: '#fff',
    fontWeight: 'bold',
    fontSize: '11px',
    fontFamily: 'inherit',
  })
  if (label) el.textContent = label
  return el
}

interface RouteResult {
  path: google.maps.LatLng[]
  bounds: google.maps.LatLngBounds | null
}

async function computeRoute(
  origin: { lat: number; lng: number },
  destination: { lat: number; lng: number },
  waypoints: Array<{ lat: number; lng: number }>,
): Promise<RouteResult | null> {
  const body: Record<string, unknown> = {
    origin: { location: { latLng: { latitude: origin.lat, longitude: origin.lng } } },
    destination: { location: { latLng: { latitude: destination.lat, longitude: destination.lng } } },
    travelMode: 'DRIVE',
    computeAlternativeRoutes: false,
  }
  if (waypoints.length > 0) {
    body.intermediates = waypoints.map(w => ({
      location: { latLng: { latitude: w.lat, longitude: w.lng } },
    }))
  }

  try {
    const res = await fetch('https://routes.googleapis.com/directions/v2:computeRoutes', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Goog-Api-Key': API_KEY,
        'X-Goog-FieldMask': 'routes.polyline.encodedPolyline,routes.viewport',
      },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      console.warn('Routes API error:', res.status, await res.text())
      return null
    }
    const data = await res.json() as {
      routes?: Array<{
        polyline: { encodedPolyline: string }
        viewport?: {
          low: { latitude: number; longitude: number }
          high: { latitude: number; longitude: number }
        }
      }>
    }
    const route = data.routes?.[0]
    if (!route) return null

    // decodePath is available after importLibrary('geometry') has been called.
    const path = google.maps.geometry.encoding.decodePath(route.polyline.encodedPolyline)

    let bounds: google.maps.LatLngBounds | null = null
    if (route.viewport) {
      bounds = new google.maps.LatLngBounds(
        { lat: route.viewport.low.latitude, lng: route.viewport.low.longitude },
        { lat: route.viewport.high.latitude, lng: route.viewport.high.longitude },
      )
    }
    return { path, bounds }
  } catch (err) {
    console.warn('computeRoute error:', err)
    return null
  }
}

export default function RouteMap({
  start, end, onStartChange, onEndChange,
  baseRoute,
  stops, onStopsChange,
  readOnly = false,
}: Props) {
  const isJoinMode = !!baseRoute

  const [mapReady, setMapReady] = useState(false)

  const containerRef = useRef<HTMLDivElement>(null)
  const mapRef = useRef<google.maps.Map | null>(null)
  const geocoderRef = useRef<google.maps.Geocoder | null>(null)
  const basePolylineRef = useRef<google.maps.Polyline | null>(null)
  const livePolylineRef = useRef<google.maps.Polyline | null>(null)
  const startMarkerRef = useRef<google.maps.marker.AdvancedMarkerElement | null>(null)
  const endMarkerRef = useRef<google.maps.marker.AdvancedMarkerElement | null>(null)
  const stopMarkersRef = useRef<google.maps.marker.AdvancedMarkerElement[]>([])
  const AdvancedMarkerElementRef = useRef<typeof google.maps.marker.AdvancedMarkerElement | null>(null)

  // Stable refs so map-init closure sees latest values without re-running
  const stopsRef = useRef(stops)
  useEffect(() => { stopsRef.current = stops }, [stops])
  const onStopsChangeRef = useRef(onStopsChange)
  useEffect(() => { onStopsChangeRef.current = onStopsChange }, [onStopsChange])
  const onStartChangeRef = useRef(onStartChange)
  useEffect(() => { onStartChangeRef.current = onStartChange }, [onStartChange])
  const onEndChangeRef = useRef(onEndChange)
  useEffect(() => { onEndChangeRef.current = onEndChange }, [onEndChange])
  const baseRouteRef = useRef(baseRoute)
  useEffect(() => { baseRouteRef.current = baseRoute }, [baseRoute])

  // Placing mode — creation only
  const [placing, setPlacing] = useState<PlacingMode>('start')
  const placingRef = useRef(placing)
  useEffect(() => { placingRef.current = placing }, [placing])

  // ── init map once ─────────────────────────────────────────────────────────
  useEffect(() => {
    if (!containerRef.current) return

    Promise.all([
      importLibrary('maps'),
      importLibrary('geocoding'),
      importLibrary('geometry'), // needed for decodePath
      importLibrary('marker'),   // needed for AdvancedMarkerElement
    ]).then(([mapsLib, geocodingLib, , markerLib]) => {
      if (!containerRef.current) return
      const { Map } = mapsLib as google.maps.MapsLibrary
      const { Geocoder } = geocodingLib as google.maps.GeocodingLibrary
      const { AdvancedMarkerElement } = markerLib as google.maps.MarkerLibrary
      AdvancedMarkerElementRef.current = AdvancedMarkerElement

      const map = new Map(containerRef.current, {
        center: { lat: 55.3, lng: 23.9 },
        zoom: 7,
        mapId: MAP_ID,
        streetViewControl: false,
        mapTypeControl: false,
        fullscreenControl: false,
      })
      mapRef.current = map
      geocoderRef.current = new Geocoder()
      setMapReady(true)

      if (isJoinMode) {
        // ── Join mode: fixed gray driver route ────────────────────────────
        const r = baseRouteRef.current!
        computeRoute(
          { lat: r.start_lat, lng: r.start_lng },
          { lat: r.end_lat, lng: r.end_lng },
          r.stops.map(s => ({ lat: s.lat, lng: s.lng })),
        ).then(result => {
          if (!result) return
          basePolylineRef.current = new google.maps.Polyline({
            path: result.path,
            map,
            strokeColor: '#94a3b8',
            strokeWeight: 5,
            strokeOpacity: 0.7,
          })
          if (result.bounds) map.fitBounds(result.bounds)
        })

        // Fixed start/end markers
        new AdvancedMarkerElement({
          map,
          position: { lat: r.start_lat, lng: r.start_lng },
          title: r.start_formatted_address ?? 'Pradžia',
          content: makeCircleDot(18, '#64748b'),
        })
        new AdvancedMarkerElement({
          map,
          position: { lat: r.end_lat, lng: r.end_lng },
          title: r.end_formatted_address ?? 'Pabaiga',
          content: makeCircleDot(18, '#334155'),
        })

        if (readOnly) return
        map.addListener('click', (e: google.maps.MapMouseEvent) => {
          if (!e.latLng || !geocoderRef.current) return
          const lat = e.latLng.lat()
          const lng = e.latLng.lng()
          geocoderRef.current.geocode({ location: { lat, lng } }, (results, status) => {
            const place: StopEntry = {
              address: status === 'OK' && results?.[0] ? results[0].formatted_address ?? '' : `${lat.toFixed(5)}, ${lng.toFixed(5)}`,
              lat, lng,
              placeId: results?.[0]?.place_id,
              locked: false,
            }
            const cur = stopsRef.current
            const lastLockedIdx = [...cur].map((s, i) => s.locked ? i : -1).filter(i => i >= 0).pop()
            if (lastLockedIdx !== undefined) {
              const next = [...cur]
              next.splice(lastLockedIdx, 0, place)
              onStopsChangeRef.current(next)
            } else {
              onStopsChangeRef.current([...cur, place])
            }
          })
        })
      } else {
        // ── Creation mode: live indigo route ─────────────────────────────
        map.addListener('click', (e: google.maps.MapMouseEvent) => {
          if (!e.latLng || !geocoderRef.current) return
          const lat = e.latLng.lat()
          const lng = e.latLng.lng()
          geocoderRef.current.geocode({ location: { lat, lng } }, (results, status) => {
            const result: StopEntry = {
              address: status === 'OK' && results?.[0] ? results[0].formatted_address ?? '' : `${lat.toFixed(5)}, ${lng.toFixed(5)}`,
              lat, lng,
              placeId: results?.[0]?.place_id,
            }
            const mode = placingRef.current
            if (mode === 'start') {
              onStartChangeRef.current?.(result)
              setPlacing('stop')
            } else if (mode === 'stop') {
              onStopsChangeRef.current([...stopsRef.current, result])
            } else {
              onEndChangeRef.current?.(result)
            }
          })
        })
      }
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // ── creation mode: sync start marker ─────────────────────────────────────
  useEffect(() => {
    if (isJoinMode || !mapRef.current || !AdvancedMarkerElementRef.current) return
    if (startMarkerRef.current) { startMarkerRef.current.map = null; startMarkerRef.current = null }
    if (!start) return
    startMarkerRef.current = new AdvancedMarkerElementRef.current({
      map: mapRef.current,
      position: { lat: start.lat, lng: start.lng },
      title: 'Išvykimas',
      content: makeCircleDot(20, '#4f46e5'),
    })
    if (!end && stops.length === 0) mapRef.current.panTo({ lat: start.lat, lng: start.lng })
  }, [start, mapReady]) // eslint-disable-line react-hooks/exhaustive-deps

  // ── creation mode: sync end marker ───────────────────────────────────────
  useEffect(() => {
    if (isJoinMode || !mapRef.current || !AdvancedMarkerElementRef.current) return
    if (endMarkerRef.current) { endMarkerRef.current.map = null; endMarkerRef.current = null }
    if (!end) return
    endMarkerRef.current = new AdvancedMarkerElementRef.current({
      map: mapRef.current,
      position: { lat: end.lat, lng: end.lng },
      title: 'Paskirties vieta',
      content: makeCircleDot(20, '#10b981'),
    })
    if (!start) mapRef.current.panTo({ lat: end.lat, lng: end.lng })
  }, [end, mapReady]) // eslint-disable-line react-hooks/exhaustive-deps

  // ── creation mode: redraw live route ─────────────────────────────────────
  useEffect(() => {
    if (isJoinMode || !mapRef.current) return

    // Clear existing polyline
    if (livePolylineRef.current) {
      livePolylineRef.current.setMap(null)
      livePolylineRef.current = null
    }

    if (!start || !end) return

    computeRoute(
      { lat: start.lat, lng: start.lng },
      { lat: end.lat, lng: end.lng },
      stops.map(s => ({ lat: s.lat, lng: s.lng })),
    ).then(result => {
      if (!result || !mapRef.current) return
      livePolylineRef.current = new google.maps.Polyline({
        path: result.path,
        map: mapRef.current,
        strokeColor: '#4f46e5',
        strokeWeight: 5,
        strokeOpacity: 0.8,
      })
      if (result.bounds) mapRef.current.fitBounds(result.bounds)
    })
  }, [start, end, stops, mapReady]) // eslint-disable-line react-hooks/exhaustive-deps

  // ── sync stop markers (both modes) ───────────────────────────────────────
  useEffect(() => {
    if (!mapRef.current || !AdvancedMarkerElementRef.current) return
    stopMarkersRef.current.forEach(m => { m.map = null })
    stopMarkersRef.current = []

    stops.forEach((s, i) => {
      const isLocked = !!s.locked
      const marker = new AdvancedMarkerElementRef.current!({
        map: mapRef.current!,
        position: { lat: s.lat, lng: s.lng },
        content: makeCircleDot(isLocked ? 22 : 26, isLocked ? '#94a3b8' : '#f59e0b', String(i + 1)),
      })
      stopMarkersRef.current.push(marker)
    })
  }, [stops, mapReady]) // eslint-disable-line react-hooks/exhaustive-deps

  // ── drag-and-drop ─────────────────────────────────────────────────────────
  const dragIndexRef = useRef<number | null>(null)
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null)

  const onDragStart = (i: number) => { dragIndexRef.current = i }
  const onDragOver = (e: React.DragEvent, i: number) => { e.preventDefault(); setDragOverIndex(i) }
  const onDragEnd = () => { dragIndexRef.current = null; setDragOverIndex(null) }
  const onDrop = (target: number) => {
    const from = dragIndexRef.current
    if (from === null || from === target) { onDragEnd(); return }
    const arr = [...stops]
    const [item] = arr.splice(from, 1)
    arr.splice(target, 0, item)
    onStopsChange(arr)
    dragIndexRef.current = null
    setDragOverIndex(null)
    setEditingIndex(null)
  }

  // ── inline editing ────────────────────────────────────────────────────────
  const [editingIndex, setEditingIndex] = useState<number | null>(null)

  const ownedCount = stops.filter(s => !s.locked).length
  const showList = isJoinMode
    ? stops.length > 0
    : !!(start || end || stops.length > 0)

  // Read-only: map + stop list, no interactive controls
  if (readOnly) {
    const readOnlyStart = isJoinMode ? baseRoute : start
    const readOnlyEnd = isJoinMode ? baseRoute : end
    return (
      <div className="space-y-2">
        <div ref={containerRef} className="w-full h-72 rounded-xl overflow-hidden border border-gray-200" />
        {(readOnlyStart || stops.length > 0 || readOnlyEnd) && (
          <div className="space-y-1">
            {readOnlyStart && (
              <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-50 border border-slate-200">
                <span className="w-2.5 h-2.5 rounded-full bg-indigo-500 shrink-0" />
                <span className="text-xs font-semibold text-slate-500 shrink-0 w-14">Pradžia</span>
                <span className="flex-1 text-sm text-slate-600 truncate">
                  {isJoinMode
                    ? (baseRoute!.start_formatted_address ?? `${baseRoute!.start_lat.toFixed(4)}, ${baseRoute!.start_lng.toFixed(4)}`)
                    : (start!.address)}
                </span>
              </div>
            )}
            {stops.map((s, i) => (
              <div key={i} className={`flex items-center gap-2 px-3 py-1.5 rounded-lg border ${
                s.locked ? 'bg-slate-50 border-slate-200' : 'bg-amber-50 border-amber-200'
              }`}>
                <span className={`w-2.5 h-2.5 rounded-full shrink-0 ${s.locked ? 'bg-slate-400' : 'bg-amber-400'}`} />
                <span className={`text-xs font-semibold shrink-0 w-8 ${s.locked ? 'text-slate-400' : 'text-amber-600'}`}>#{i + 1}</span>
                <span className={`flex-1 text-sm truncate ${s.locked ? 'text-slate-600' : 'text-gray-700'}`}>
                  {s.address || `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`}
                </span>
              </div>
            ))}
            {readOnlyEnd && (
              <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-50 border border-slate-200">
                <span className="w-2.5 h-2.5 rounded-full bg-emerald-600 shrink-0" />
                <span className="text-xs font-semibold text-slate-500 shrink-0 w-14">Pabaiga</span>
                <span className="flex-1 text-sm text-slate-600 truncate">
                  {isJoinMode
                    ? (baseRoute!.end_formatted_address ?? `${baseRoute!.end_lat.toFixed(4)}, ${baseRoute!.end_lng.toFixed(4)}`)
                    : (end!.address)}
                </span>
              </div>
            )}
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {/* Creation mode: placing-mode toggle */}
      {!isJoinMode && (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setPlacing('start')}
            className={`flex-1 py-1.5 rounded-lg text-sm font-medium border transition ${
              placing === 'start'
                ? 'bg-indigo-600 text-white border-indigo-600'
                : 'bg-white text-gray-600 border-gray-300 hover:border-indigo-400'
            }`}
          >
            {start ? start.address.split(',')[0] : 'Spustelėkite norėdami nustatyti išvykimą'}
          </button>
          <button
            type="button"
            onClick={() => setPlacing('stop')}
            className={`flex-1 py-1.5 rounded-lg text-sm font-medium border transition ${
              placing === 'stop'
                ? 'bg-amber-500 text-white border-amber-500'
                : 'bg-white text-gray-600 border-gray-300 hover:border-amber-400'
            }`}
          >
            Pridėti stotelę
          </button>
          <button
            type="button"
            onClick={() => setPlacing('end')}
            className={`flex-1 py-1.5 rounded-lg text-sm font-medium border transition ${
              placing === 'end'
                ? 'bg-emerald-600 text-white border-emerald-600'
                : 'bg-white text-gray-600 border-gray-300 hover:border-emerald-400'
            }`}
          >
            {end ? end.address.split(',')[0] : 'Spustelėkite norėdami nustatyti paskirties vietą'}
          </button>
        </div>
      )}

      {/* Join mode: instruction */}
      {isJoinMode && (
        <div className="flex items-center justify-between">
          <p className="text-xs text-gray-500">
            Spustelėkite žemėlapį norėdami pridėti savo sustojimus. Vilkite norėdami pertvarkyti.
          </p>
          {ownedCount > 0 && (
            <span className="text-xs text-amber-600 font-medium">{ownedCount} {ownedCount !== 1 ? 'stotelės pridėtos' : 'stotelė pridėta'}</span>
          )}
        </div>
      )}

      {/* Map */}
      <div ref={containerRef} className="w-full h-80 rounded-xl overflow-hidden border border-gray-200" />

      {/* Stop list */}
      {showList && (
        <div className="space-y-1">
          {/* Start row */}
          {(isJoinMode ? baseRoute : start) && (
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-50 border border-slate-200">
              <span className="w-2.5 h-2.5 rounded-full bg-indigo-500 shrink-0" />
              <span className="text-xs font-semibold text-slate-500 shrink-0 w-14">Pradžia</span>
              <span className="flex-1 text-sm text-slate-600 truncate">
                {isJoinMode
                  ? (baseRoute!.start_formatted_address ?? `${baseRoute!.start_lat.toFixed(4)}, ${baseRoute!.start_lng.toFixed(4)}`)
                  : (start!.address || `${start!.lat.toFixed(4)}, ${start!.lng.toFixed(4)}`)}
              </span>
            </div>
          )}

          {/* Intermediate stops */}
          {stops.map((s, i) => (
            <div key={i}>
              <div
                draggable
                onDragStart={() => onDragStart(i)}
                onDragOver={(e) => onDragOver(e, i)}
                onDrop={() => onDrop(i)}
                onDragEnd={onDragEnd}
                className={`flex items-center gap-2 rounded-lg px-3 py-2 border transition cursor-grab active:cursor-grabbing ${
                  dragOverIndex === i
                    ? `${s.locked ? 'bg-slate-100 border-slate-400' : 'bg-amber-50 border-amber-400'} shadow-sm`
                    : s.locked
                      ? 'bg-slate-50 border-slate-200'
                      : 'bg-amber-50 border-amber-200'
                } ${dragIndexRef.current === i ? 'opacity-40' : ''}`}
              >
                <span className={`shrink-0 select-none text-sm ${s.locked ? 'text-slate-400' : 'text-gray-400'}`}>::</span>
                <span
                  className="w-2.5 h-2.5 rounded-full shrink-0"
                  style={{ backgroundColor: s.locked ? '#94a3b8' : '#f59e0b' }}
                />
                <span className={`text-xs font-semibold shrink-0 w-5 ${s.locked ? 'text-slate-400' : 'text-amber-600'}`}>
                  {i + 1}
                </span>
                <span className={`flex-1 text-sm truncate ${s.locked ? 'text-slate-500' : 'text-gray-700'}`}>
                  {s.address || `${s.lat.toFixed(4)}, ${s.lng.toFixed(4)}`}
                </span>

                {/* Edit — unlocked stops only */}
                {!s.locked && (
                  <button
                    type="button"
                    onClick={() => setEditingIndex(editingIndex === i ? null : i)}
                    className={`shrink-0 w-6 h-6 flex items-center justify-center rounded text-sm transition ${
                      editingIndex === i ? 'bg-indigo-100 text-indigo-600' : 'hover:bg-gray-200 text-gray-400 hover:text-indigo-600'
                    }`}
                    title="Redaguoti adresą"
                  >
                    ✎
                  </button>
                )}

                {/* Remove */}
                {(!isJoinMode || !s.locked) && (
                  <button
                    type="button"
                    onClick={() => {
                      onStopsChange(stops.filter((_, j) => j !== i))
                      if (editingIndex === i) setEditingIndex(null)
                    }}
                    className="text-gray-400 hover:text-red-500 shrink-0 text-sm leading-none px-1"
                    title="Pašalinti stotelę"
                  >
                    x
                  </button>
                )}
              </div>

              {/* Inline address edit */}
              {editingIndex === i && (
                <div className="mt-1 ml-7 p-3 bg-white border border-indigo-200 rounded-lg shadow-sm">
                  <PlaceInput
                    label=""
                    placeholder="Ieškoti naujo adreso…"
                    value={s.address}
                    onSelect={(place) => {
                      const next = [...stops]
                      next[i] = { ...place, locked: s.locked }
                      onStopsChange(next)
                      setEditingIndex(null)
                    }}
                  />
                  <button type="button" onClick={() => setEditingIndex(null)} className="mt-2 text-xs text-gray-500 hover:text-gray-700">
                    Atšaukti
                  </button>
                </div>
              )}
            </div>
          ))}

          {/* End row */}
          {(isJoinMode ? baseRoute : end) && (
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-50 border border-slate-200">
              <span className="w-2.5 h-2.5 rounded-full bg-emerald-600 shrink-0" />
              <span className="text-xs font-semibold text-slate-500 shrink-0 w-14">Pabaiga</span>
              <span className="flex-1 text-sm text-slate-600 truncate">
                {isJoinMode
                  ? (baseRoute!.end_formatted_address ?? `${baseRoute!.end_lat.toFixed(4)}, ${baseRoute!.end_lng.toFixed(4)}`)
                  : (end!.address || `${end!.lat.toFixed(4)}, ${end!.lng.toFixed(4)}`)}
              </span>
            </div>
          )}
        </div>
      )}

      {/* Legend — join mode only */}
      {isJoinMode && (
        <div className="flex gap-4 text-xs text-gray-500 px-1">
          <span className="flex items-center gap-1"><span className="w-3 h-3 rounded-full bg-slate-400 inline-block" /> Vairuotojo stotelės</span>
          <span className="flex items-center gap-1"><span className="w-3 h-3 rounded-full bg-amber-400 inline-block" /> Jūsų stotelės</span>
        </div>
      )}
    </div>
  )
}
