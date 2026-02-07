import { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { updateMe, disableMyAccount, getUserProfile } from '../api/auth'
import { listMyVehicles, deleteVehicle } from '../api/vehicles'
import VehicleCreateModal from '../components/VehicleCreateModal'
import type { Review, Vehicle } from '../types'

function StarRating({ rating }: { rating: number }) {
  return (
    <span className="text-sm text-gray-700">
      {rating}/5
    </span>
  )
}

export default function ProfilePage() {
  const { user, refetch } = useAuth()
  const navigate = useNavigate()
  const [name, setName] = useState(user?.name ?? '')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [reviews, setReviews] = useState<Review[]>([])
  const [vehicles, setVehicles] = useState<Vehicle[]>([])
  const [showVehicleModal, setShowVehicleModal] = useState(false)
  const [disabling, setDisabling] = useState(false)
  const [confirmDisable, setConfirmDisable] = useState(false)

  useEffect(() => {
    if (!user) return
    getUserProfile(user.id).then((p) => setReviews(p.reviews)).catch(() => {})
    listMyVehicles().then(setVehicles).catch(() => {})
  }, [user])

  if (!user) return null

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setSaveError(null)
    setSaved(false)
    try {
      await updateMe(name)
      await refetch()
      setSaved(true)
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  const handleDisable = async () => {
    setDisabling(true)
    try {
      await disableMyAccount()
      await refetch()
      navigate('/login')
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to disable account')
      setDisabling(false)
    }
  }

  const handleVehicleCreated = (vehicle: Vehicle) => {
    setVehicles(vs => [...vs, vehicle])
    setShowVehicleModal(false)
  }

  const handleDeleteVehicle = async (id: string) => {
    try {
      await deleteVehicle(id)
      setVehicles(vs => vs.filter(v => v.id !== id))
    } catch {
      alert('Failed to delete vehicle')
    }
  }

  const avgRating = reviews.length
    ? (reviews.reduce((s, r) => s + r.rating, 0) / reviews.length).toFixed(1)
    : null

  return (
    <div className="max-w-2xl mx-auto px-4 py-8 space-y-6">
      {showVehicleModal && (
        <VehicleCreateModal
          onCreated={handleVehicleCreated}
          onClose={() => setShowVehicleModal(false)}
        />
      )}

      <h1 className="text-2xl font-bold text-gray-900">Mano profilis</h1>

      {/* Info card */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6 space-y-4">
        <div>
          <div className="text-xs text-gray-400 mb-0.5">El. paštas</div>
          <div className="text-sm text-gray-700">{user.email}</div>
        </div>
        <div>
          <div className="text-xs text-gray-400 mb-0.5">Teikėjas</div>
          <div className="text-sm text-gray-700 capitalize">{user.provider}</div>
        </div>

        <form onSubmit={handleSave} className="space-y-3">
          <div>
            <label className="block text-xs text-gray-400 mb-1">Rodomas vardas</label>
            <input
              className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Jūsų vardas"
            />
          </div>
          {saveError && <p className="text-red-600 text-sm">{saveError}</p>}
          {saved && <p className="text-green-600 text-sm">Išsaugota!</p>}
          <button
            type="submit"
            disabled={saving}
            className="bg-indigo-600 text-white px-5 py-2 rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
          >
            {saving ? 'Išsaugoma…' : 'Išsaugoti'}
          </button>
        </form>
      </div>

      {/* Vehicles */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6 space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="font-semibold text-gray-900">Transporto priemonės</h2>
          <button
            onClick={() => setShowVehicleModal(true)}
            className="bg-indigo-600 text-white px-4 py-1.5 rounded-lg text-sm font-medium hover:bg-indigo-700 transition"
          >
            + Pridėti transporto priemonę
          </button>
        </div>

        {vehicles.length === 0 ? (
          <p className="text-sm text-gray-400">Dar nėra transporto priemonių. Pridėkite, kad galėtumėte pasiūlyti keliones.</p>
        ) : (
          <ul className="space-y-2">
            {vehicles.map(v => (
              <li
                key={v.id}
                className="flex items-center justify-between border border-gray-100 rounded-xl px-4 py-3"
              >
                <div>
                  <span className="text-sm font-medium text-gray-800">
                    {v.make ? `${v.make} ` : ''}{v.model}
                  </span>
                  <span className="ml-2 text-xs text-gray-400">{v.plate_number} · {v.seats} vietos</span>
                </div>
                <div className="flex items-center gap-2">
                  <Link
                    to={`/vehicles/${v.id}/edit`}
                    className="text-xs font-medium px-3 py-1.5 rounded-lg border border-gray-200 bg-white text-gray-700 hover:bg-gray-50 transition"
                  >
                    Redaguoti
                  </Link>
                  <button
                    onClick={() => handleDeleteVehicle(v.id)}
                    className="text-xs font-medium px-3 py-1.5 rounded-lg border border-red-200 bg-red-50 text-red-600 hover:bg-red-100 transition"
                  >
                    Pašalinti
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* Reviews */}
      <div className="bg-white rounded-2xl border border-gray-200 p-6">
        <div className="flex items-center gap-3 mb-4">
          <h2 className="font-semibold text-gray-900">Atsiliepimai</h2>
          {avgRating && (
            <span className="text-sm text-gray-500">
              <StarRating rating={Math.round(Number(avgRating))} /> {avgRating} · {reviews.length} {reviews.length !== 1 ? 'atsiliepimai' : 'atsiliepimas'}
            </span>
          )}
        </div>
        {reviews.length === 0 ? (
          <p className="text-sm text-gray-400">Dar nėra atsiliepimų.</p>
        ) : (
          <ul className="space-y-4">
            {reviews.map((rv) => (
              <li key={rv.id} className="border-b border-gray-100 pb-4 last:border-0 last:pb-0">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm font-medium text-gray-800">{rv.author_name}</span>
                  <StarRating rating={rv.rating} />
                </div>
                {rv.comment && <p className="text-sm text-gray-600">{rv.comment}</p>}
                <p className="text-xs text-gray-400 mt-1">{new Date(rv.created_at).toLocaleDateString('lt-LT')}</p>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* Danger zone */}
      <div className="bg-white rounded-2xl border border-red-200 p-6">
        <h2 className="font-semibold text-red-700 mb-1">Pavojinga zona</h2>
        <p className="text-sm text-gray-500 mb-4">Išjungus paskyrą prieiga bus užblokuota. Šio veiksmo negalima atšaukti.</p>
        {!confirmDisable ? (
          <button
            onClick={() => setConfirmDisable(true)}
            className="bg-red-50 text-red-600 border border-red-300 px-4 py-2 rounded-lg text-sm font-medium hover:bg-red-100 transition"
          >
            Išjungti mano paskyrą
          </button>
        ) : (
          <div className="flex gap-3">
            <button
              onClick={handleDisable}
              disabled={disabling}
              className="bg-red-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-red-700 disabled:opacity-50 transition"
            >
              {disabling ? 'Išjungiama…' : 'Taip, išjungti'}
            </button>
            <button
              onClick={() => setConfirmDisable(false)}
              className="text-gray-600 px-4 py-2 rounded-lg text-sm font-medium hover:bg-gray-100 transition"
            >
              Atšaukti
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
