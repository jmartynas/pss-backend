import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import PlaceInput, { type PlaceResult } from '../components/PlaceInput'

export default function HomePage() {
  const navigate = useNavigate()
  const [from, setFrom] = useState<PlaceResult | null>(null)
  const [to, setTo] = useState<PlaceResult | null>(null)

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    if (!from || !to) return
    const params = new URLSearchParams({
      start_lat: String(from.lat),
      start_lng: String(from.lng),
      end_lat: String(to.lat),
      end_lng: String(to.lng),
      from: from.address,
      to: to.address,
    })
    navigate(`/search?${params}`)
  }

  const ready = from !== null && to !== null

  return (
    <div className="min-h-screen bg-gradient-to-b from-indigo-50 to-white">
      {/* Hero */}
      <div className="max-w-3xl mx-auto px-4 pt-24 pb-16 text-center">
        <h1 className="text-5xl font-extrabold text-gray-900 mb-4 leading-tight">
          Pasidalink kelione,<br />
          <span className="text-indigo-600">dalinkis išlaidomis</span>
        </h1>
        <p className="text-lg text-gray-500 mb-12">
          Rask kelionę ar pasiūlyk vietas savo maršrute — be tarpininkų, tik bendruomenė.
        </p>

        {/* Search card */}
        <form
          onSubmit={handleSearch}
          className="bg-white rounded-2xl shadow-md border border-gray-200 p-6 text-left"
        >
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <PlaceInput
              label="Išvykimo vieta"
              placeholder="Miestas ar adresas"
              onSelect={setFrom}
            />
            <PlaceInput
              label="Paskirties vieta"
              placeholder="Miestas ar adresas"
              onSelect={setTo}
            />
          </div>

          {/* Coordinate confirmation pills */}
          {(from || to) && (
            <div className="flex flex-wrap gap-2 mt-3">
              {from && (
                <span className="text-xs bg-indigo-50 text-indigo-600 px-2 py-1 rounded-full">
                  {from.address}
                </span>
              )}
              {to && (
                <span className="text-xs bg-indigo-50 text-indigo-600 px-2 py-1 rounded-full">
                  {to.address}
                </span>
              )}
            </div>
          )}

          <button
            type="submit"
            disabled={!ready}
            className="mt-5 w-full bg-indigo-600 text-white py-3 rounded-xl font-semibold hover:bg-indigo-700 disabled:opacity-40 disabled:cursor-not-allowed transition"
          >
            Ieškoti kelionių
          </button>
        </form>
      </div>

      {/* Feature grid */}
      <div className="max-w-4xl mx-auto px-4 pb-24 grid grid-cols-1 md:grid-cols-3 gap-6 text-center">
        {[
          { title: 'Pasiūlyti kelionę', desc: 'Važiuoji kažkur? Paimk keleivius ir pasidalink degalų kaina.' },
          { title: 'Rasti kelionę', desc: 'Ieškok maršrutų pagal vietą. Sistema suranda vairuotojus tavo kryptimi.' },
          { title: 'Lengvas užsakymas', desc: 'Pateik paraišką prisijungti prie kelionės. Vairuotojas patvirtina ir esate pasiruošę.' },
        ].map((f) => (
          <div key={f.title} className="bg-white rounded-2xl border border-gray-200 p-6">
            <h3 className="font-semibold text-gray-900 mb-1">{f.title}</h3>
            <p className="text-sm text-gray-500">{f.desc}</p>
          </div>
        ))}
      </div>
    </div>
  )
}
