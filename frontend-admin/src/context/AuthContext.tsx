import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'

interface AuthState {
  token: string
  adminId: string
  permissions: number
}

interface AuthContextType {
  auth: AuthState | null
  signIn: (token: string, adminId: string, permissions: number) => void
  signOut: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [auth, setAuth] = useState<AuthState | null>(() => {
    const token = localStorage.getItem('admin_token')
    const adminId = localStorage.getItem('admin_id')
    const perms = localStorage.getItem('admin_permissions')
    if (token && adminId && perms) {
      return { token, adminId, permissions: Number(perms) }
    }
    return null
  })

  useEffect(() => {
    if (auth) {
      localStorage.setItem('admin_token', auth.token)
      localStorage.setItem('admin_id', auth.adminId)
      localStorage.setItem('admin_permissions', String(auth.permissions))
    } else {
      localStorage.removeItem('admin_token')
      localStorage.removeItem('admin_id')
      localStorage.removeItem('admin_permissions')
    }
  }, [auth])

  function signIn(token: string, adminId: string, permissions: number) {
    setAuth({ token, adminId, permissions })
  }

  function signOut() {
    setAuth(null)
  }

  return <AuthContext.Provider value={{ auth, signIn, signOut }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
