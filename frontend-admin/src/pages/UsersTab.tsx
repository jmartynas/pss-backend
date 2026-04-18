import { useState, useEffect, useCallback } from 'react'
import { api, type AdminUser } from '../api'

const STATUS_LABEL: Record<string, string> = {
  active: 'aktyvus',
  blocked: 'užblokuotas',
  inactive: 'neaktyvus',
}

export default function UsersTab() {
  const [users, setUsers] = useState<AdminUser[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState<string | null>(null)

  const [filterEmail, setFilterEmail] = useState('')
  const [filterName, setFilterName] = useState('')
  const [filterProvider, setFilterProvider] = useState('')
  const [filterStatus, setFilterStatus] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setUsers(await api.users.list())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Nepavyko įkelti naudotojų')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  async function toggleBlock(user: AdminUser) {
    setBusy(user.id)
    try {
      if (user.status === 'blocked') {
        await api.users.unblock(user.id)
      } else {
        await api.users.block(user.id)
      }
      await load()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Veiksmas nepavyko')
    } finally {
      setBusy(null)
    }
  }

  const filtered = users.filter((u) => {
    if (filterEmail && !u.email.toLowerCase().includes(filterEmail.toLowerCase())) return false
    if (filterName && !u.name.toLowerCase().includes(filterName.toLowerCase())) return false
    if (filterProvider && u.provider !== filterProvider) return false
    if (filterStatus && u.status !== filterStatus) return false
    return true
  })

  if (loading) return <div className="text-gray-400 text-sm">Kraunama…</div>
  if (error) return <div className="text-red-600 text-sm">{error}</div>

  return (
    <div className="space-y-4">
      <div className="bg-white rounded-xl shadow p-4 flex flex-wrap gap-3">
        <input
          type="text"
          placeholder="Filtruoti pagal el. paštą"
          value={filterEmail}
          onChange={(e) => setFilterEmail(e.target.value)}
          className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 min-w-48"
        />
        <input
          type="text"
          placeholder="Filtruoti pagal vardą"
          value={filterName}
          onChange={(e) => setFilterName(e.target.value)}
          className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 min-w-48"
        />
        <select
          value={filterProvider}
          onChange={(e) => setFilterProvider(e.target.value)}
          className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option value="">Visi tiekėjai</option>
          <option value="google">Google</option>
          <option value="github">GitHub</option>
          <option value="microsoft">Microsoft</option>
        </select>
        <select
          value={filterStatus}
          onChange={(e) => setFilterStatus(e.target.value)}
          className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option value="">Visos būsenos</option>
          <option value="active">Aktyvus</option>
          <option value="blocked">Užblokuotas</option>
          <option value="inactive">Neaktyvus</option>
        </select>
        {(filterEmail || filterName || filterProvider || filterStatus) && (
          <button
            onClick={() => { setFilterEmail(''); setFilterName(''); setFilterProvider(''); setFilterStatus('') }}
            className="text-sm text-gray-400 hover:text-gray-700 transition-colors"
          >
            Išvalyti
          </button>
        )}
      </div>

      <div className="bg-white rounded-xl shadow overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
            <tr>
              <th className="px-4 py-3 text-left">El. paštas</th>
              <th className="px-4 py-3 text-left">Vardas</th>
              <th className="px-4 py-3 text-left">Tiekėjas</th>
              <th className="px-4 py-3 text-left">Būsena</th>
              <th className="px-4 py-3 text-left">Sukurta</th>
              <th className="px-4 py-3 text-right">Veiksmai</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {filtered.map((u) => (
              <tr key={u.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 font-medium text-gray-800">{u.email}</td>
                <td className="px-4 py-3 text-gray-600">{u.name || '—'}</td>
                <td className="px-4 py-3 text-gray-500">{u.provider}</td>
                <td className="px-4 py-3">
                  <span
                    className={`inline-block px-2 py-0.5 rounded-full text-xs font-medium ${
                      u.status === 'active'
                        ? 'bg-green-100 text-green-700'
                        : u.status === 'blocked'
                        ? 'bg-red-100 text-red-700'
                        : 'bg-gray-100 text-gray-600'
                    }`}
                  >
                    {STATUS_LABEL[u.status] ?? u.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-500">{new Date(u.created_at).toLocaleDateString('lt-LT')}</td>
                <td className="px-4 py-3 text-right">
                  {u.status !== 'inactive' && (
                    <button
                      onClick={() => toggleBlock(u)}
                      disabled={busy === u.id}
                      className={`text-xs font-medium px-3 py-1 rounded-lg transition-colors disabled:opacity-50 ${
                        u.status === 'blocked'
                          ? 'bg-green-100 text-green-700 hover:bg-green-200'
                          : 'bg-red-100 text-red-700 hover:bg-red-200'
                      }`}
                    >
                      {busy === u.id ? '…' : u.status === 'blocked' ? 'Atblokuoti' : 'Blokuoti'}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {filtered.length === 0 && (
          <div className="text-center text-gray-400 py-8 text-sm">
            {users.length === 0 ? 'Naudotojų nerasta.' : 'Nėra atitinkančių filtrą naudotojų.'}
          </div>
        )}
      </div>
    </div>
  )
}
