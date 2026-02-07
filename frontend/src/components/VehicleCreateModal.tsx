import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { createVehicle } from '../api/vehicles'
import { ApiError } from '../api/client'
import type { Vehicle } from '../types'

interface Props {
  onCreated: (vehicle: Vehicle) => void
  onClose: () => void
}

export default function VehicleCreateModal({ onCreated, onClose }: Props) {
  const [form, setForm] = useState({ make: '', model: '', plate_number: '', seats: '4' })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const firstInputRef = useRef<HTMLInputElement>(null)

  // Focus first field and lock body scroll when mounted
  useEffect(() => {
    firstInputRef.current?.focus()
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = '' }
  }, [])

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm(f => ({ ...f, [k]: e.target.value }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setError(null)
    setLoading(true)
    try {
      const { id } = await createVehicle({
        make: form.make || undefined,
        model: form.model,
        plate_number: form.plate_number,
        seats: parseInt(form.seats, 10),
      })
      const vehicle: Vehicle = {
        id,
        user_id: '',
        make: form.make,
        model: form.model,
        plate_number: form.plate_number,
        seats: parseInt(form.seats, 10),
        created_at: new Date().toISOString(),
      }
      onCreated(vehicle)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Nepavyko išsaugoti transporto priemonės')
    } finally {
      setLoading(false)
    }
  }

  return createPortal(
    /* Backdrop */
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
    >
      {/* Panel */}
      <div className="w-full max-w-md bg-white rounded-2xl shadow-xl p-6 space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">Pridėti transporto priemonę</h2>
          <button
            type="button"
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 text-xl leading-none"
            aria-label="Close"
          >
            x
          </button>
        </div>

        {error && (
          <div className="bg-red-50 text-red-600 rounded-lg px-4 py-2 text-sm">{error}</div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Markė</label>
            <input
              ref={firstInputRef}
              type="text"
              placeholder="pvz. Toyota"
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
              value={form.make}
              onChange={set('make')}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Modelis <span className="text-red-500">*</span>
            </label>
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
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Valstybinis numeris <span className="text-red-500">*</span>
            </label>
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
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Keleivių sėdynės <span className="text-red-500">*</span>
            </label>
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

          <div className="flex gap-3 pt-1">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 py-2.5 rounded-xl border border-gray-300 text-sm font-medium text-gray-600 hover:bg-gray-50 transition"
            >
              Atšaukti
            </button>
            <button
              type="submit"
              disabled={loading}
              className="flex-1 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-semibold hover:bg-indigo-700 disabled:opacity-40 transition"
            >
              {loading ? 'Išsaugoma…' : 'Išsaugoti transporto priemonę'}
            </button>
          </div>
        </form>
      </div>
    </div>,
    document.body,
  )
}
