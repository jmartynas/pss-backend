const BASE = ''

let refreshing: Promise<boolean> | null = null

async function tryRefresh(): Promise<boolean> {
  if (refreshing) return refreshing
  refreshing = fetch(BASE + '/auth/refresh', { method: 'POST', credentials: 'include' })
    .then(r => r.ok)
    .catch(() => false)
    .finally(() => { refreshing = null })
  return refreshing
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const opts: RequestInit = {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(options.headers ?? {}) },
    ...options,
  }

  let res = await fetch(BASE + path, opts)

  if (res.status === 401) {
    const ok = await tryRefresh()
    if (ok) {
      res = await fetch(BASE + path, opts)
    } else {
      throw new ApiError(401, 'Sesija pasibaigė')
    }
  }

  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    let message = text.trim()
    try {
      const json = JSON.parse(text)
      if (typeof json?.error === 'string') message = json.error
    } catch {}
    throw new ApiError(res.status, message)
  }

  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export class ApiError extends Error {
  readonly status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'ApiError'
  }
}

const translations: Record<string, string> = {
  'route not found':                                        'Maršrutas nerastas',
  'route has already started':                              'Maršrutas jau pradėtas',
  'route is full':                                          'Maršrutas užpildytas',
  'route creator cannot apply to own route':                'Maršruto kūrėjas negali teikti prašymo',
  'already applied to this route':                          'Prašymas šiam maršrutui jau pateiktas',
  'one or more stops is too far from the route':            'Viena ar daugiau stotelių yra per toli nuo maršruto',
  "one or more stops is past the driver's destination":     'Viena ar daugiau stotelių yra toliau nei vairuotojo paskirties vieta',
  'application not found':                                  'Prašymas nerastas',
  'application is not in pending state':                    'Prašymas jau buvo peržiūrėtas',
  'application cannot be cancelled once accepted or rejected': 'Patvirtinto ar atmesto prašymo atšaukti negalima',
  'application cannot be edited once accepted or rejected': 'Patvirtinto ar atmesto prašymo redaguoti negalima',
  'application is not approved':                            'Prašymas nėra patvirtintas',
  'no pending stop-change request':                         'Nėra laukiančio stotelių keitimo prašymo',
  'no pending stop change request':                         'Nėra laukiančio stotelių keitimo prašymo',
  'not found':                                              'Nerasta',
  'forbidden':                                              'Veiksmas draudžiamas',
  'unauthorized':                                           'Nesate prisijungę',
  'invalid request body':                                   'Neteisingi duomenys',
}

export function errMsg(e: unknown, fallback: string): string {
  if (e instanceof ApiError) {
    if (e.status >= 500) return 'Serverio klaida. Bandykite vėliau.'
    return translations[e.message] ?? fallback
  }
  return fallback
}

export const get = <T>(path: string) => request<T>(path, { method: 'GET' })
export const post = <T>(path: string, body?: unknown) =>
  request<T>(path, { method: 'POST', body: JSON.stringify(body) })
export const patch = <T>(path: string, body?: unknown) =>
  request<T>(path, { method: 'PATCH', body: JSON.stringify(body) })
export const del = <T>(path: string) => request<T>(path, { method: 'DELETE' })
