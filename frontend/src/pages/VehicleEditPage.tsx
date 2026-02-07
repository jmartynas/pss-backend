import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { listMyVehicles, updateVehicle } from '../api/vehicles'
import { ApiError } from '../api/client'

export default function VehicleEditPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [fetching, setFetching] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [form, setForm] = useState({
    make: '',
    model: '',
    plate_number: '',
    seats: '4',
  })

  useEffect(() => {
    listMyVehicles()
      .then((vehicles) => {
        const v = vehicles.find((v) => v.id === id)
        if (!v) { navigate('/profile'); return }
        setForm({
          make: v.make ?? '',
          model: v.model,
          plate_number: v.plate_number,
          seats: String(v.seats),
        })
      })
      .catch(() => navigate('/profile'))
      .finally(() => setFetching(false))
  }, [id, navigate])

  const set =
    (k: keyof typeof form) =>
    (e: React.ChangeEvent<HTMLInputElement>) =>
      setForm((f) => ({ ...f, [k]: e.target.value }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await updateVehicle(id!, {
        make: form.make || undefined,
        model: form.model,
        plate_number: form.plate_number,
        seats: parseInt(form.seats, 10),
      })
      navigate('/profile')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Nepavyko išsaugoti transporto priemonės')
    } finally {
      setLoading(false)
    }
  }

  if (fetching) return null

  return (
    <div className="max-w-md mx-auto px-4 py-8">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Redaguoti transporto priemonę</h1>

      {error && (
        <div className="bg-red-50 text-red-600 rounded-lg p-4 mb-6 text-sm">{error}</div>
      )}

      <form onSubmit={handleSubmit} className="bg-white rounded-2xl border border-gray-200 p-6 space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Markė</label>
          <input
            type="text"
            placeholder="pvz. Toyota"
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
            value={form.make}
            onChange={set('make')}
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Modelis <span className="text-red-500">*</span></label>
          <input
            required
            type="text"
            placeholder="pvz. Corolla"
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
            value={form.model}
            onChange={set('model')}
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Valstybinis numeris <span className="text-red-500">*</span></label>
          <input
            required
            type="text"
            placeholder="pvz. ABC 123"
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
            value={form.plate_number}
            onChange={set('plate_number')}
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Keleivių sėdynės <span className="text-red-500">*</span></label>
          <input
            required
            type="number"
            min="1"
            max="16"
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
            value={form.seats}
            onChange={set('seats')}
          />
        </div>

        <div className="flex gap-3 pt-2">
          <button
            type="button"
            onClick={() => navigate('/profile')}
            className="flex-1 py-2.5 rounded-xl border border-gray-300 text-sm font-medium text-gray-600 hover:bg-gray-50 transition"
          >
            Atšaukti
          </button>
          <button
            type="submit"
            disabled={loading}
            className="flex-1 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-semibold hover:bg-indigo-700 disabled:opacity-40 transition"
          >
            {loading ? 'Išsaugoma…' : 'Išsaugoti'}
          </button>
        </div>
      </form>
    </div>
  )
}
