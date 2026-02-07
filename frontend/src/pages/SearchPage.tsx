import { useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { searchRoutes } from '../api/routes'
import type { Route } from '../types'
import RouteCard from '../components/RouteCard'
import PlaceInput, { type PlaceResult } from '../components/PlaceInput'
import RouteMap, { type StopEntry } from '../components/RouteMap'
import { useAuth } from '../context/AuthContext'

type SearchState = 'idle' | 'loading' | 'done' | 'error'

export default function SearchPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()

  // Pre-populate from URL params (when navigating from HomePage)
  const initialFrom = params.get('from') ?? ''
  const initialTo = params.get('to') ?? ''
  const initialStartLat = parseFloat(params.get('start_lat') ?? 'NaN')
  const initialStartLng = parseFloat(params.get('start_lng') ?? 'NaN')
  const initialEndLat = parseFloat(params.get('end_lat') ?? 'NaN')
  const initialEndLng = parseFloat(params.get('end_lng') ?? 'NaN')
  const hasInitialCoords =
    !isNaN(initialStartLat) && !isNaN(initialStartLng) &&
    !isNaN(initialEndLat) && !isNaN(initialEndLng)

  const initialStops: StopEntry[] = (() => {
    try { return JSON.parse(params.get('stops') ?? '[]') } catch { return [] }
  })()

  const [from, setFrom] = useState<PlaceResult | null>(
    hasInitialCoords ? { address: initialFrom, lat: initialStartLat, lng: initialStartLng } : null
  )
  const [to, setTo] = useState<PlaceResult | null>(
    hasInitialCoords ? { address: initialTo, lat: initialEndLat, lng: initialEndLng } : null
  )

  const [routes, setRoutes] = useState<Route[]>([])
  const [state, setState] = useState<SearchState>(hasInitialCoords ? 'loading' : 'idle')
  const [error, setError] = useState<string | null>(null)
  const [minRating, setMinRating] = useState<number>(0) // 0 = any
  const [searchStops, setSearchStops] = useState<StopEntry[]>(initialStops)

  // Run search once on mount if URL params are present, or whenever coords change via form submit
  const searchedRef = useRef(false)
  useEffect(() => {
    if (!hasInitialCoords || searchedRef.current) return
    searchedRef.current = true
    runSearch(initialStartLat, initialStartLng, initialEndLat, initialEndLng)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function runSearch(startLat: number, startLng: number, endLat: number, endLng: number, stops: StopEntry[] = searchStops) {
    setState('loading')
    setError(null)
    searchRoutes({
      start_lat: startLat,
      start_lng: startLng,
      end_lat: endLat,
      end_lng: endLng,
      stops: stops.map(s => ({ lat: s.lat, lng: s.lng })),
    })
      .then(r => {
        const filtered = user
          ? r.filter(rt => !rt.participants.some(p => p.user_id === user.id))
          : r
        setRoutes(filtered)
        setState('done')
      })
      .catch((e: Error) => { setError(e.message); setState('error') })
  }

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    if (!from || !to) return

    // Update URL so results are shareable / back-navigable
    const p = new URLSearchParams({
      start_lat: String(from.lat),
      start_lng: String(from.lng),
      end_lat: String(to.lat),
      end_lng: String(to.lng),
      from: from.address,
      to: to.address,
    })
    if (searchStops.length > 0) {
      p.set('stops', JSON.stringify(searchStops.map(s => ({ lat: s.lat, lng: s.lng, address: s.address }))))
    }
    navigate(`/?${p}`, { replace: true })
    runSearch(from.lat, from.lng, to.lat, to.lng)
  }

  const ready = from !== null && to !== null

  // Effective rating for filtering: drivers with <5 reviews count as 5 stars
  const filteredRoutes = minRating === 0 ? routes : routes.filter(r => {
    const effective = r.creator_review_count >= 5 && r.creator_rating != null
      ? r.creator_rating
      : 5
    return effective >= minRating
  })

  return (
    <div className="max-w-3xl mx-auto px-4 py-8">
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Rasti kelionę</h2>

      {/* Map — always visible; lets user set from/to and add stops */}
      <div className="bg-white rounded-2xl border border-gray-200 p-5 mb-6 shadow-sm">
        <p className="text-xs text-gray-400 mb-3">
          Naudokite žemėlapį nustatyti išvykimą, paskirties vietą ir sustojimus.
        </p>
        <RouteMap
          start={from}
          end={to}
          onStartChange={setFrom}
          onEndChange={setTo}
          stops={searchStops}
          onStopsChange={setSearchStops}
        />
      </div>

      {/* Search form */}
      <form
        onSubmit={handleSearch}
        className="bg-white rounded-2xl border border-gray-200 p-5 mb-6 shadow-sm"
      >
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <PlaceInput
            label="Išvykimo vieta"
            placeholder="Miestas ar adresas"
            value={from?.address}
            onSelect={setFrom}
          />
          <PlaceInput
            label="Paskirties vieta"
            placeholder="Miestas ar adresas"
            value={to?.address}
            onSelect={setTo}
          />
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Min. vairuotojo įvertinimas</label>
            <select
              value={minRating}
              onChange={e => setMinRating(Number(e.target.value))}
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
            >
              <option value={0}>Bet koks</option>
              <option value={2}>2+</option>
              <option value={3}>3+</option>
              <option value={4}>4+</option>
            </select>
          </div>
        </div>

        <button
          type="submit"
          disabled={!ready || state === 'loading'}
          className="mt-4 w-full bg-indigo-600 text-white py-2.5 rounded-xl font-semibold hover:bg-indigo-700 disabled:opacity-40 disabled:cursor-not-allowed transition"
        >
          {state === 'loading' ? 'Ieškoma…' : 'Ieškoti kelionių'}
        </button>
      </form>

      {/* Results */}
      {state === 'idle' && (
        <div className="text-center py-8 text-gray-400">
          <div className="text-4xl mb-3"></div>
          <p className="text-sm">Įveskite išvykimo ir paskirties vietą, kad rastumėte keliones.</p>
        </div>
      )}

      {state === 'error' && (
        <div className="bg-red-50 text-red-600 rounded-lg p-4 text-sm">{error}</div>
      )}

      {state === 'loading' && (
        <div className="text-center py-16 text-gray-400 text-sm">Ieškoma kelionių…</div>
      )}

      {state === 'done' && (
        <>
          <p className="text-sm text-gray-500 mb-4">
            {filteredRoutes.length === 0
              ? 'Nėra kelionių atitinkančių filtrus.'
              : `Rasta ${filteredRoutes.length} ${filteredRoutes.length !== 1 ? 'kelionės' : 'kelionė'}`}
          </p>

          {filteredRoutes.length === 0 ? (
            <div className="text-center py-12 text-gray-400">
              <div className="text-4xl mb-3"></div>
              <p className="text-sm">
                {routes.length === 0
                  ? 'Kelionių nerasta. Pabandykite pakeisti vietas arba grįžkite vėliau.'
                  : 'Nėra kelionių atitinkančių pasirinktą įvertinimą. Pabandykite sumažinti filtrą.'}
              </p>
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              {filteredRoutes.map((r) => (
                <RouteCard
                  key={r.id}
                  route={r}
                  showBadge
                  linkState={{ searchFrom: from, searchTo: to, searchStops }}
                />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}
