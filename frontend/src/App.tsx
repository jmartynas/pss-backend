import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider, useAuth } from './context/AuthContext'
import Navbar from './components/Navbar'
import LoginPage from './pages/LoginPage'
import SearchPage from './pages/SearchPage'
import RouteDetailPage from './pages/RouteDetailPage'
import CreateRoutePage from './pages/CreateRoutePage'
import MyRoutesPage from './pages/MyRoutesPage'
import MyApplicationsPage from './pages/MyApplicationsPage'
import ProfilePage from './pages/ProfilePage'
import UserProfilePage from './pages/UserProfilePage'
import VehicleCreatePage from './pages/VehicleCreatePage'
import VehicleEditPage from './pages/VehicleEditPage'
import ChatsPage from './pages/ChatsPage'
import type { ReactNode } from 'react'

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) return <div className="flex items-center justify-center min-h-screen text-gray-400">Loading…</div>
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}

function AppRoutes() {
  return (
    <div className="min-h-screen bg-gray-50">
      <Navbar />
      <Routes>
        <Route path="/" element={<SearchPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/routes/:id" element={<RouteDetailPage />} />
        <Route
          path="/routes/new"
          element={<RequireAuth><CreateRoutePage /></RequireAuth>}
        />
        <Route
          path="/my-routes"
          element={<RequireAuth><MyRoutesPage /></RequireAuth>}
        />
        <Route
          path="/my-applications"
          element={<RequireAuth><MyApplicationsPage /></RequireAuth>}
        />
        <Route
          path="/profile"
          element={<RequireAuth><ProfilePage /></RequireAuth>}
        />
        <Route path="/users/:id" element={<UserProfilePage />} />
        <Route
          path="/vehicles/new"
          element={<RequireAuth><VehicleCreatePage /></RequireAuth>}
        />
        <Route
          path="/vehicles/:id/edit"
          element={<RequireAuth><VehicleEditPage /></RequireAuth>}
        />
        <Route
          path="/chats"
          element={<RequireAuth><ChatsPage /></RequireAuth>}
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  )
}
