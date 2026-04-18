import { useState, type FormEvent } from 'react'
import { useAuth } from '../context/AuthContext'
import { PERM_USERS, PERM_ROUTES, PERM_ADMINS, changePassword } from '../api'
import UsersTab from './UsersTab'
import RoutesTab from './RoutesTab'
import AdminsTab from './AdminsTab'

type Tab = 'users' | 'routes' | 'admins'

export default function DashboardPage() {
  const { auth, signOut } = useAuth()
  const perms = auth?.permissions ?? 0

  const availableTabs: Tab[] = []
  if (perms & PERM_USERS) availableTabs.push('users')
  if (perms & PERM_ROUTES) availableTabs.push('routes')
  if (perms & PERM_ADMINS) availableTabs.push('admins')

  const [tab, setTab] = useState<Tab>(availableTabs[0] ?? 'users')

  const [showPwForm, setShowPwForm] = useState(false)
  const [currentPw, setCurrentPw] = useState('')
  const [newPw, setNewPw] = useState('')
  const [pwError, setPwError] = useState('')
  const [pwSaving, setPwSaving] = useState(false)

  const tabLabel: Record<Tab, string> = {
    users: 'Naudotojai',
    routes: 'Maršrutai',
    admins: 'Administratoriai',
  }

  async function handleChangePw(e: FormEvent) {
    e.preventDefault()
    setPwError('')
    setPwSaving(true)
    try {
      await changePassword(currentPw, newPw)
      setCurrentPw('')
      setNewPw('')
      setShowPwForm(false)
    } catch (err) {
      setPwError(err instanceof Error ? err.message : 'Nepavyko pakeisti slaptažodžio')
    } finally {
      setPwSaving(false)
    }
  }

  return (
    <div className="min-h-screen bg-gray-100">
      <header className="bg-white border-b border-gray-200 px-6 py-3 flex items-center justify-between">
        <span className="font-bold text-gray-800 text-lg">PSS Administravimas</span>
        <div className="flex items-center gap-4">
          <button
            onClick={() => { setShowPwForm((v) => !v); setPwError('') }}
            className="text-sm text-gray-500 hover:text-gray-800 transition-colors"
          >
            Keisti slaptažodį
          </button>
          <button
            onClick={signOut}
            className="text-sm text-gray-500 hover:text-gray-800 transition-colors"
          >
            Atsijungti
          </button>
        </div>
      </header>

      {showPwForm && (
        <div className="bg-white border-b border-gray-200 px-6 py-4">
          <form onSubmit={handleChangePw} className="flex flex-wrap items-end gap-3 max-w-xl">
            <div>
              <label className="block text-xs text-gray-500 mb-1">Dabartinis slaptažodis</label>
              <input
                type="password"
                value={currentPw}
                onChange={(e) => setCurrentPw(e.target.value)}
                required
                className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Naujas slaptažodis</label>
              <input
                type="password"
                value={newPw}
                onChange={(e) => setNewPw(e.target.value)}
                required
                className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            {pwError && <p className="text-xs text-red-600 w-full">{pwError}</p>}
            <button
              type="submit"
              disabled={pwSaving}
              className="text-sm font-medium px-4 py-1.5 rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-50"
            >
              {pwSaving ? 'Saugoma…' : 'Išsaugoti'}
            </button>
          </form>
        </div>
      )}

      <div className="max-w-6xl mx-auto p-6">
        {availableTabs.length === 0 ? (
          <div className="text-gray-500 text-center mt-16">Nepriskirtos jokios teisės.</div>
        ) : (
          <>
            <div className="flex gap-1 mb-6 border-b border-gray-200">
              {availableTabs.map((t) => (
                <button
                  key={t}
                  onClick={() => setTab(t)}
                  className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                    tab === t
                      ? 'border-blue-600 text-blue-600'
                      : 'border-transparent text-gray-500 hover:text-gray-800'
                  }`}
                >
                  {tabLabel[t]}
                </button>
              ))}
            </div>

            {tab === 'users' && <UsersTab />}
            {tab === 'routes' && <RoutesTab />}
            {tab === 'admins' && <AdminsTab currentAdminId={auth?.adminId ?? ''} />}
          </>
        )}
      </div>
    </div>
  )
}
