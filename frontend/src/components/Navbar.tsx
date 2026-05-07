import { Link, NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { logout } from '../api/auth'

export default function Navbar() {
  const { user, refetch } = useAuth()
  const navigate = useNavigate()

  const handleLogout = async () => {
    await logout().catch(() => {})
    await refetch()
    navigate('/')
  }

  return (
    <nav className="bg-white border-b border-gray-200 sticky top-0 z-50">
      <div className="max-w-6xl mx-auto px-4 flex items-center justify-between h-14 gap-4 overflow-x-auto">
        <Link to="/" className="text-xl font-bold text-indigo-600 shrink-0">
          <span className="hidden lg:inline">Pakeleivių paieškos sistema</span>
          <span className="lg:hidden">PSS</span>
        </Link>

        <div className="flex items-center gap-2 lg:gap-4 text-sm shrink-0">
          <NavLink
            to="/"
            end
            className={({ isActive }) =>
              isActive ? 'text-indigo-600 font-medium' : 'text-gray-600 hover:text-gray-900'
            }
          >
            Rasti kelionę
          </NavLink>

          {user ? (
            <>
              <NavLink
                to="/routes/new"
                className={({ isActive }) =>
                  isActive ? 'text-indigo-600 font-medium' : 'text-gray-600 hover:text-gray-900'
                }
              >
                Pasiūlyti kelionę
              </NavLink>
              <NavLink
                to="/my-routes"
                className={({ isActive }) =>
                  isActive ? 'text-indigo-600 font-medium' : 'text-gray-600 hover:text-gray-900'
                }
              >
                Mano maršrutai
              </NavLink>
              <NavLink
                to="/my-applications"
                className={({ isActive }) =>
                  isActive ? 'text-indigo-600 font-medium' : 'text-gray-600 hover:text-gray-900'
                }
              >
                Mano kelionės
              </NavLink>
              <NavLink
                to="/chats"
                className={({ isActive }) =>
                  isActive ? 'text-indigo-600 font-medium' : 'text-gray-600 hover:text-gray-900'
                }
              >
                Pokalbiai
              </NavLink>
              <NavLink
                to="/profile"
                className={({ isActive }) =>
                  isActive
                    ? 'bg-indigo-50 text-indigo-600 font-medium px-3 py-1.5 rounded-lg'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200 px-3 py-1.5 rounded-lg'
                }
              >
                {user.name || user.email}
              </NavLink>
              <button
                onClick={handleLogout}
                className="text-red-500 hover:text-white hover:bg-red-500 border border-red-200 hover:border-red-500 px-3 py-1.5 rounded-lg transition-colors"
              >
                Atsijungti
              </button>
            </>
          ) : (
            <Link
              to="/login"
              className="bg-indigo-600 text-white px-4 py-1.5 rounded-md hover:bg-indigo-700"
            >
              Prisijungti
            </Link>
          )}
        </div>
      </div>
    </nav>
  )
}
