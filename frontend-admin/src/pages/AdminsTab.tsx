import { useState, useEffect, useCallback, type FormEvent } from 'react'
import { api, type Admin, PERM_USERS, PERM_ROUTES, PERM_ADMINS } from '../api'

const PERMS = [
  { bit: PERM_USERS, label: 'Valdyti naudotojus' },
  { bit: PERM_ROUTES, label: 'Valdyti maršrutus' },
  { bit: PERM_ADMINS, label: 'Valdyti administratorius' },
]

export default function AdminsTab({ currentAdminId }: { currentAdminId: string }) {
  const [admins, setAdmins] = useState<Admin[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState<string | null>(null)
  const [pending, setPending] = useState<Record<string, number>>({})

  const [showCreate, setShowCreate] = useState(false)
  const [newEmail, setNewEmail] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newPerms, setNewPerms] = useState(0)
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await api.admins.list()
      setAdmins(data)
      const map: Record<string, number> = {}
      data.forEach((a) => { map[a.id] = a.permissions })
      setPending(map)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Nepavyko įkelti administratorių')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  function togglePerm(adminId: string, bit: number) {
    setPending((prev) => ({ ...prev, [adminId]: (prev[adminId] ?? 0) ^ bit }))
  }

  async function deleteAdmin(adminId: string) {
    if (!confirm('Ištrinti šį administratorių?')) return
    try {
      await api.admins.delete(adminId)
      await load()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Veiksmas nepavyko')
    }
  }

  async function savePerms(adminId: string) {
    setSaving(adminId)
    try {
      await api.admins.setPermissions(adminId, pending[adminId] ?? 0)
      await load()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Nepavyko išsaugoti teisių')
    } finally {
      setSaving(null)
    }
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault()
    setCreateError('')
    setCreating(true)
    try {
      await api.admins.create(newEmail, newPassword, newPerms)
      setNewEmail('')
      setNewPassword('')
      setNewPerms(0)
      setShowCreate(false)
      await load()
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'Nepavyko sukurti administratoriaus')
    } finally {
      setCreating(false)
    }
  }

  if (loading) return <div className="text-gray-400 text-sm">Kraunama…</div>
  if (error) return <div className="text-red-600 text-sm">{error}</div>

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button
          onClick={() => { setShowCreate((v) => !v); setCreateError('') }}
          className="text-sm font-medium px-4 py-2 rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors"
        >
          {showCreate ? 'Atšaukti' : 'Naujas administratorius'}
        </button>
      </div>

      {showCreate && (
        <div className="bg-white rounded-xl shadow p-5">
          <h2 className="text-sm font-semibold text-gray-700 mb-4">Sukurti administratorių</h2>
          <form onSubmit={handleCreate} className="space-y-3">
            <div className="flex gap-3">
              <div className="flex-1">
                <label className="block text-xs text-gray-500 mb-1">El. paštas</label>
                <input
                  type="email"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  required
                  className="w-full border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div className="flex-1">
                <label className="block text-xs text-gray-500 mb-1">Slaptažodis</label>
                <input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  className="w-full border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-2">Teisės</label>
              <div className="flex gap-4">
                {PERMS.map(({ bit, label }) => (
                  <label key={bit} className="flex items-center gap-1.5 text-xs cursor-pointer">
                    <input
                      type="checkbox"
                      checked={!!(newPerms & bit)}
                      onChange={() => setNewPerms((p) => p ^ bit)}
                      className="accent-blue-600"
                    />
                    {label}
                  </label>
                ))}
              </div>
            </div>
            {createError && <p className="text-xs text-red-600">{createError}</p>}
            <div className="flex justify-end">
              <button
                type="submit"
                disabled={creating}
                className="text-sm font-medium px-4 py-1.5 rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-50"
              >
                {creating ? 'Kuriama…' : 'Sukurti'}
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="bg-white rounded-xl shadow overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
            <tr>
              <th className="px-4 py-3 text-left">El. paštas</th>
              <th className="px-4 py-3 text-left">Sukurta</th>
              <th className="px-4 py-3 text-left">Teisės</th>
              <th className="px-4 py-3 text-right">Veiksmai</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {admins.map((a) => {
              const isSelf = a.id === currentAdminId
              const currentPerms = pending[a.id] ?? a.permissions
              const dirty = currentPerms !== a.permissions

              return (
                <tr key={a.id} className={isSelf ? 'bg-gray-50 opacity-60' : 'hover:bg-gray-50'}>
                  <td className="px-4 py-3 font-medium text-gray-800">
                    {a.email}
                  </td>
                  <td className="px-4 py-3 text-gray-500">{new Date(a.created_at).toLocaleDateString('lt-LT')}</td>
                  <td className="px-4 py-3">
                    <div className="flex gap-4">
                      {PERMS.map(({ bit, label }) => (
                        <label
                          key={bit}
                          className={`flex items-center gap-1.5 text-xs ${isSelf ? 'cursor-not-allowed' : 'cursor-pointer'}`}
                        >
                          <input
                            type="checkbox"
                            checked={!!(currentPerms & bit)}
                            onChange={() => !isSelf && togglePerm(a.id, bit)}
                            disabled={isSelf}
                            className="accent-blue-600"
                          />
                          {label}
                        </label>
                      ))}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-right">
                    {!isSelf && (
                      <div className="flex gap-2 justify-end">
                        <button
                          onClick={() => savePerms(a.id)}
                          disabled={saving === a.id || !dirty}
                          className="text-xs font-medium px-3 py-1 rounded-lg bg-blue-100 text-blue-700 hover:bg-blue-200 transition-colors disabled:opacity-40"
                        >
                          {saving === a.id ? '…' : 'Išsaugoti'}
                        </button>
                        <button
                          onClick={() => deleteAdmin(a.id)}
                          className="text-xs font-medium px-3 py-1 rounded-lg bg-red-100 text-red-700 hover:bg-red-200 transition-colors"
                        >
                          Ištrinti
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
        {admins.length === 0 && (
          <div className="text-center text-gray-400 py-8 text-sm">Administratorių nerasta.</div>
        )}
      </div>
    </div>
  )
}
