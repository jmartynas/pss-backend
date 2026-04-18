import { useState, useEffect, useCallback } from 'react'
import { api, type AdminRoute } from '../api'
import PlaceInput from '../components/PlaceInput'

export default function RoutesTab() {
  const [routes, setRoutes] = useState<AdminRoute[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState<string | null>(null)

  const [filterFrom, setFilterFrom] = useState('')
  const [filterTo, setFilterTo] = useState('')
  const [filterCreator, setFilterCreator] = useState('')
  const [filterDateFrom, setFilterDateFrom] = useState('')
  const [filterDateTo, setFilterDateTo] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setRoutes(await api.routes.list())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Nepavyko įkelti maršrutų')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  async function deleteRoute(id: string) {
    if (!confirm('Ištrinti šį maršrutą?')) return
    setBusy(id)
    try {
      await api.routes.delete(id)
      await load()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Veiksmas nepavyko')
    } finally {
      setBusy(null)
    }
  }

  const filtered = routes.filter((rt) => {
    if (filterFrom && !rt.from.toLowerCase().includes(filterFrom.toLowerCase())) return false
    if (filterTo && !rt.to.toLowerCase().includes(filterTo.toLowerCase())) return false
    if (filterCreator && !rt.creator.toLowerCase().includes(filterCreator.toLowerCase())) return false
    if (filterDateFrom && rt.leaving_at && rt.leaving_at < filterDateFrom) return false
    if (filterDateTo && rt.leaving_at && rt.leaving_at > filterDateTo + 'T23:59:59') return false
    return true
  })

  const hasFilter = filterFrom || filterTo || filterCreator || filterDateFrom || filterDateTo

  function clearFilters() {
    setFilterFrom(''); setFilterTo(''); setFilterCreator(''); setFilterDateFrom(''); setFilterDateTo('')
  }

  if (loading) return <div className="text-gray-400 text-sm">Kraunama…</div>
  if (error) return <div className="text-red-600 text-sm">{error}</div>

  return (
    <div className="space-y-4">
      <div className="bg-white rounded-xl shadow p-4 flex flex-wrap gap-3 items-end">
        <div>
          <label className="block text-xs text-gray-500 mb-1">Iš</label>
          <PlaceInput
            placeholder="Ieškoti pradžios"
            value={filterFrom}
            onChange={setFilterFrom}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">Į</label>
          <PlaceInput
            placeholder="Ieškoti pabaigos"
            value={filterTo}
            onChange={setFilterTo}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">Kūrėjas</label>
          <input
            type="text"
            placeholder="Ieškoti kūrėjo"
            value={filterCreator}
            onChange={(e) => setFilterCreator(e.target.value)}
            className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 min-w-44"
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">Išvyksta nuo</label>
          <input
            type="date"
            value={filterDateFrom}
            onChange={(e) => setFilterDateFrom(e.target.value)}
            className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">Išvyksta iki</label>
          <input
            type="date"
            value={filterDateTo}
            onChange={(e) => setFilterDateTo(e.target.value)}
            className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
        {hasFilter && (
          <button
            onClick={clearFilters}
            className="text-sm text-gray-400 hover:text-gray-700 transition-colors pb-0.5"
          >
            Išvalyti
          </button>
        )}
      </div>

      <div className="bg-white rounded-xl shadow overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
            <tr>
              <th className="px-4 py-3 text-left">Iš</th>
              <th className="px-4 py-3 text-left">Į</th>
              <th className="px-4 py-3 text-left">Kūrėjas</th>
              <th className="px-4 py-3 text-left">Išvyksta</th>
              <th className="px-4 py-3 text-right">Veiksmai</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {filtered.map((rt) => (
              <tr key={rt.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 text-gray-800 max-w-48 truncate">{rt.from || '—'}</td>
                <td className="px-4 py-3 text-gray-800 max-w-48 truncate">{rt.to || '—'}</td>
                <td className="px-4 py-3 text-gray-600">{rt.creator}</td>
                <td className="px-4 py-3 text-gray-500">
                  {rt.leaving_at ? new Date(rt.leaving_at).toLocaleString('lt-LT') : '—'}
                </td>
                <td className="px-4 py-3 text-right">
                  <button
                    onClick={() => deleteRoute(rt.id)}
                    disabled={busy === rt.id}
                    className="text-xs font-medium px-3 py-1 rounded-lg bg-red-100 text-red-700 hover:bg-red-200 transition-colors disabled:opacity-50"
                  >
                    {busy === rt.id ? '…' : 'Ištrinti'}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {filtered.length === 0 && (
          <div className="text-center text-gray-400 py-8 text-sm">
            {routes.length === 0 ? 'Maršrutų nerasta.' : 'Nėra atitinkančių filtrą maršrutų.'}
          </div>
        )}
      </div>
    </div>
  )
}
