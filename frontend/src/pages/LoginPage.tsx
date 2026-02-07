import { useSearchParams } from 'react-router-dom'
import { loginUrl } from '../api/auth'

const providers = [
  { id: 'google', label: 'Tęsti su Google', color: 'bg-white border-gray-300 text-gray-700 hover:bg-gray-50' },
  { id: 'github', label: 'Tęsti su GitHub', color: 'bg-gray-900 text-white hover:bg-gray-800' },
  { id: 'microsoft', label: 'Tęsti su Microsoft', color: 'bg-[#2f7ed8] text-white hover:bg-[#2369c0]' },
] as const

const errorMessages: Record<string, string> = {
  blocked: 'Jūsų paskyra buvo išjungta. Kreipkitės į palaikymą.',
}

export default function LoginPage() {
  const [params] = useSearchParams()
  const error = params.get('error')

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center px-4">
      <div className="bg-white rounded-2xl border border-gray-200 shadow-sm p-10 w-full max-w-sm">
        <h1 className="text-2xl font-bold text-gray-900 mb-2 text-center">
          Prisijungti prie Pakeleivių paieškos sistemos
        </h1>
        <p className="text-sm text-gray-500 text-center mb-8">
          Raskite ar pasiūlykite keliones žmonėms einantiems jūsų keliu
        </p>

        {error && (
          <div className="bg-red-50 text-red-600 text-sm rounded-lg px-4 py-3 mb-6">
            {errorMessages[error] ?? 'Kažkas nutiko. Bandykite dar kartą.'}
          </div>
        )}

        <div className="flex flex-col gap-3">
          {providers.map((p) => (
            <a
              key={p.id}
              href={loginUrl(p.id)}
              className={`w-full flex items-center justify-center py-2.5 px-4 rounded-lg border font-medium text-sm transition ${p.color}`}
            >
              {p.label}
            </a>
          ))}
        </div>
      </div>
    </div>
  )
}
