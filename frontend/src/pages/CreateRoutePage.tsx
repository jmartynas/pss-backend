import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createRoute } from '../api/routes'
import { listMyVehicles } from '../api/vehicles'
import { ApiError } from '../api/client'
import PlaceInput, { type PlaceResult } from '../components/PlaceInput'
import RouteMap, { type StopEntry } from '../components/RouteMap'
import VehicleCreateModal from '../components/VehicleCreateModal'
import type { Vehicle } from '../types'

export default function CreateRoutePage() {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [start, setStart] = useState<PlaceResult | null>(null)
  const [end, setEnd] = useState<PlaceResult | null>(null)
  const [stops, setStops] = useState<StopEntry[]>([])

  // Vehicle selection
  const [vehicles, setVehicles] = useState<Vehicle[]>([])
  const [selectedVehicleId, setSelectedVehicleId] = useState<string>('')
  const [showVehicleModal, setShowVehicleModal] = useState(false)
  const [seatOverride, setSeatOverride] = useState<string>('')

  useEffect(() => {
    listMyVehicles().then(setVehicles).catch(() => {})
  }, [])

  const selectedVehicle = vehicles.find(v => v.id === selectedVehicleId) ?? null
  const effectiveSeats = seatOverride !== ''
    ? seatOverride
    : selectedVehicle
      ? String(selectedVehicle.seats)
      : '3'

  const [details, setDetails] = useState({
    description: '',
    max_deviation: '10',
    price: '',
    leaving_at: '',
  })

  const set =
    (k: keyof typeof details) =>
    (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      setDetails((d) => ({ ...d, [k]: e.target.value }))

  const handleVehicleChange = (id: string) => {
    setSelectedVehicleId(id)
    setSeatOverride('')
  }

  const handleVehicleCreated = (vehicle: Vehicle) => {
    setVehicles(vs => [...vs, vehicle])
    setSelectedVehicleId(vehicle.id)
    setSeatOverride('')
    setShowVehicleModal(false)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!start || !end) {
      setError('Pasirinkite išvykimo ir paskirties vietą.')
      return
    }
    if (!details.leaving_at) {
      setError('Pasirinkite išvykimo laiką.')
      return
    }
    if (new Date(details.leaving_at) <= new Date()) {
      setError('Išvykimo laikas turi būti ateityje.')
      return
    }
    setError(null)
    setLoading(true)
    try {
      const { id } = await createRoute({
        vehicle_id: selectedVehicleId || undefined,
        description: details.description || undefined,
        start_lat: start.lat,
        start_lng: start.lng,
        start_place_id: start.placeId,
        start_formatted_address: start.address,
        end_lat: end.lat,
        end_lng: end.lng,
        end_place_id: end.placeId,
        end_formatted_address: end.address,
        max_passengers: parseInt(effectiveSeats, 10),
        max_deviation: parseFloat(details.max_deviation),
        price: details.price ? parseFloat(details.price) : undefined,
        leaving_at: details.leaving_at
          ? new Date(details.leaving_at).toISOString()
          : undefined,
        stops: stops.map(s => ({
          lat: s.lat,
          lng: s.lng,
          place_id: s.placeId,
          formatted_address: s.address,
        })),
      })
      navigate(`/routes/${id}`)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to create route')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto px-4 py-8">
      {showVehicleModal && (
        <VehicleCreateModal
          onCreated={handleVehicleCreated}
          onClose={() => setShowVehicleModal(false)}
        />
      )}

      <h1 className="text-2xl font-bold text-gray-900 mb-6">Pasiūlyti kelionę</h1>

      {error && (
        <div className="bg-red-50 text-red-600 rounded-lg p-4 mb-6 text-sm">{error}</div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">

        {/* Route */}
        <div className="bg-white rounded-2xl border border-gray-200 p-6 space-y-4">
          <h2 className="font-semibold text-gray-800">Maršrutas</h2>

          <PlaceInput
            label="Išvykimas"
            placeholder="Ieškoti išvykimo adreso…"
            value={start?.address}
            onSelect={setStart}
          />
          <PlaceInput
            label="Paskirties vieta"
            placeholder="Ieškoti paskirties adreso…"
            value={end?.address}
            onSelect={setEnd}
          />

          <RouteMap
            start={start}
            end={end}
            onStartChange={setStart}
            onEndChange={setEnd}
            stops={stops}
            onStopsChange={setStops}
          />
        </div>

        {/* Schedule */}
        <div className="bg-white rounded-2xl border border-gray-200 p-6 space-y-4">
          <h2 className="font-semibold text-gray-800">Grafikas</h2>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Išvykimo data ir laikas <span className="text-red-500">*</span>
            </label>
            <input
              type="datetime-local"
              required
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
              min={new Date(Date.now() - new Date().getTimezoneOffset() * 60000).toISOString().slice(0, 16)}
              value={details.leaving_at}
              onChange={set('leaving_at')}
            />
          </div>
        </div>

        {/* Details */}
        <div className="bg-white rounded-2xl border border-gray-200 p-6 space-y-4">
          <h2 className="font-semibold text-gray-800">Detalės</h2>

          {/* Vehicle picker */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="block text-sm font-medium text-gray-700">Transporto priemonė</label>
              <button
                type="button"
                onClick={() => setShowVehicleModal(true)}
                className="text-xs text-indigo-600 hover:underline"
              >
                + Pridėti transporto priemonę
              </button>
            </div>
            {vehicles.length === 0 ? (
              <p className="text-sm text-gray-400 italic">
                Dar nėra transporto priemonių.{' '}
                <button
                  type="button"
                  onClick={() => setShowVehicleModal(true)}
                  className="text-indigo-600 hover:underline"
                >
                  Pridėkite
                </button>
                {' '}norint automatiškai užpildyti vietų skaičių.
              </p>
            ) : (
              <select
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400 bg-white"
                value={selectedVehicleId}
                onChange={e => handleVehicleChange(e.target.value)}
              >
                <option value="">— Transporto priemonė nepasirinkta —</option>
                {vehicles.map(v => (
                  <option key={v.id} value={v.id}>
                    {v.make ? `${v.make} ` : ''}{v.model} · {v.plate_number} · {v.seats} vietos
                  </option>
                ))}
              </select>
            )}
          </div>

          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Vietos
                {selectedVehicle && seatOverride === '' && (
                  <span className="ml-1 text-xs text-gray-400">(iš transporto priemonės)</span>
                )}
              </label>
              <input
                required
                type="number"
                min="1"
                max="16"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                value={effectiveSeats}
                onChange={e => setSeatOverride(e.target.value)}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Maks. nukrypimas (km)</label>
              <input
                required
                type="number"
                min="0"
                step="0.1"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                value={details.max_deviation}
                onChange={set('max_deviation')}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Kaina / vietai (€)</label>
              <input
                type="number"
                min="0"
                step="0.01"
                placeholder="Nemokama"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                value={details.price}
                onChange={set('price')}
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Aprašymas</label>
            <textarea
              rows={3}
              placeholder="Papildoma informacija keleiviams — bagažas, stotelės, pageidavimai…"
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-400"
              value={details.description}
              onChange={set('description')}
            />
          </div>
        </div>

        <button
          type="submit"
          disabled={loading || !start || !end}
          className="w-full bg-indigo-600 text-white py-3 rounded-xl font-semibold hover:bg-indigo-700 disabled:opacity-40 disabled:cursor-not-allowed transition"
        >
          {loading ? 'Skelbiama…' : 'Paskelbti kelionę'}
        </button>
      </form>
    </div>
  )
}
