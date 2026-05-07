import { useEffect, useRef, useState, useCallback } from 'react'
import { setOptions, importLibrary } from '@googlemaps/js-api-loader'

setOptions({
  key: import.meta.env.VITE_GOOGLE_MAPS_API_KEY ?? '',
  v: 'weekly',
  libraries: ['places'],
  language: 'lt',
})

interface SuggestionItem {
  main: string
  secondary: string | null
  prediction: google.maps.places.PlacePrediction
}

interface Props {
  placeholder?: string
  value: string
  onChange: (address: string) => void
}

export default function PlaceInput({ placeholder, value, onChange }: Props) {
  const [inputValue, setInputValue] = useState(value)
  const [suggestions, setSuggestions] = useState<SuggestionItem[]>([])
  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)

  const onChangeRef = useRef(onChange)
  useEffect(() => { onChangeRef.current = onChange }, [onChange])

  const placesRef = useRef<google.maps.PlacesLibrary | null>(null)
  const sessionTokenRef = useRef<google.maps.places.AutocompleteSessionToken | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    importLibrary('places').then(lib => {
      const places = lib as google.maps.PlacesLibrary
      placesRef.current = places
      sessionTokenRef.current = new places.AutocompleteSessionToken()
    })
  }, [])

  useEffect(() => {
    setInputValue(value)
    if (!value) { setSuggestions([]); setOpen(false) }
  }, [value])

  const fetchSuggestions = useCallback((input: string) => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    if (!input.trim() || input.length < 2) { setSuggestions([]); setOpen(false); return }
    debounceRef.current = setTimeout(async () => {
      const places = placesRef.current
      if (!places) return
      try {
        const { suggestions: raw } = await places.AutocompleteSuggestion.fetchAutocompleteSuggestions({
          input,
          sessionToken: sessionTokenRef.current ?? undefined,
        })
        const items: SuggestionItem[] = raw
          .filter(s => s.placePrediction)
          .map(s => ({
            main: s.placePrediction!.mainText?.text ?? s.placePrediction!.text.text,
            secondary: s.placePrediction!.secondaryText?.text ?? null,
            prediction: s.placePrediction!,
          }))
        setSuggestions(items)
        setOpen(items.length > 0)
        setActiveIndex(-1)
      } catch {
        setSuggestions([]); setOpen(false)
      }
    }, 200)
  }, [])

  async function handleSelect(item: SuggestionItem) {
    const places = placesRef.current!
    setSuggestions([]); setOpen(false)
    const place = item.prediction.toPlace()
    try {
      await place.fetchFields({ fields: ['formattedAddress', 'displayName'] })
      const address = place.formattedAddress ?? place.displayName ?? item.main
      setInputValue(address)
      onChangeRef.current(address)
      sessionTokenRef.current = new places.AutocompleteSessionToken()
    } catch { /* ignore */ }
  }

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  return (
    <div ref={containerRef} className="relative">
      <input
        type="text"
        value={inputValue}
        placeholder={placeholder}
        className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 w-full"
        onChange={e => { setInputValue(e.target.value); onChangeRef.current(e.target.value); fetchSuggestions(e.target.value) }}
        onKeyDown={e => {
          if (!open || !suggestions.length) return
          if (e.key === 'ArrowDown') { e.preventDefault(); setActiveIndex(i => Math.min(i + 1, suggestions.length - 1)) }
          else if (e.key === 'ArrowUp') { e.preventDefault(); setActiveIndex(i => Math.max(i - 1, 0)) }
          else if (e.key === 'Enter' && activeIndex >= 0) { e.preventDefault(); handleSelect(suggestions[activeIndex]) }
          else if (e.key === 'Escape') setOpen(false)
        }}
        onFocus={() => { if (suggestions.length > 0) setOpen(true) }}
      />
      {open && suggestions.length > 0 && (
        <ul className="absolute z-50 left-0 right-0 mt-1 bg-white border border-gray-200 rounded-lg shadow-lg max-h-56 overflow-y-auto">
          {suggestions.map((s, i) => (
            <li
              key={i}
              onMouseDown={e => { e.preventDefault(); handleSelect(s) }}
              className={`px-3 py-2 text-sm cursor-pointer ${i === activeIndex ? 'bg-blue-50' : 'hover:bg-gray-50'}`}
            >
              <div className="font-medium text-gray-800 truncate">{s.main}</div>
              {s.secondary && <div className="text-xs text-gray-400 truncate">{s.secondary}</div>}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
