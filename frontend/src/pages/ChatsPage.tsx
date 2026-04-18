import { useEffect, useRef, useState } from 'react'
import { useAuth } from '../context/AuthContext'
import {
  listPrivateChats, listGroupChats,
  getPrivateMessages, getGroupMessages,
  sendPrivateMessage, sendGroupMessage,
} from '../api/chats'
import type { PrivateChat, GroupChat, ChatMessage } from '../types'

type Tab = 'private' | 'group'
type ActiveChat =
  | { kind: 'private'; id: string; name: string }
  | { kind: 'group'; id: string; name: string }

function fmtTime(iso: string) {
  return new Date(iso).toLocaleTimeString('lt-LT', { hour: '2-digit', minute: '2-digit' })
}

export default function ChatsPage() {
  const { user } = useAuth()
  const [tab, setTab] = useState<Tab>('private')
  const [privateChats, setPrivateChats] = useState<PrivateChat[]>([])
  const [groupChats, setGroupChats] = useState<GroupChat[]>([])
  const [activeChat, setActiveChat] = useState<ActiveChat | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [loadingChats, setLoadingChats] = useState(true)
  const [loadingMsgs, setLoadingMsgs] = useState(false)
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    setLoadingChats(true)
    Promise.all([listPrivateChats(), listGroupChats()])
      .then(([priv, grp]) => {
        setPrivateChats(priv)
        setGroupChats(grp)
      })
      .catch(() => {})
      .finally(() => setLoadingChats(false))
  }, [])

  const loadMessages = async (chat: ActiveChat) => {
    setLoadingMsgs(true)
    try {
      const msgs = chat.kind === 'private'
        ? await getPrivateMessages(chat.id)
        : await getGroupMessages(chat.id)
      setMessages(msgs)
    } catch {
      setMessages([])
    } finally {
      setLoadingMsgs(false)
    }
  }

  const openChat = (chat: ActiveChat) => {
    setActiveChat(chat)
    setMessages([])
    setInput('')
    loadMessages(chat)
  }

  // SSE: stream new messages while a chat is open
  useEffect(() => {
    if (esRef.current) { esRef.current.close(); esRef.current = null }
    if (!activeChat) return
    const url = activeChat.kind === 'private'
      ? `/chats/private/${activeChat.id}/events`
      : `/chats/group/${activeChat.id}/events`
    const es = new EventSource(url, { withCredentials: true })
    esRef.current = es
    es.onmessage = (e) => {
      try {
        const msgs = JSON.parse(e.data) as ChatMessage[]
        setMessages(msgs)
      } catch {}
    }
    return () => { es.close(); esRef.current = null }
  }, [activeChat])

  // Scroll to bottom when messages change
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleSend = async () => {
    if (!activeChat || !input.trim()) return
    setSending(true)
    try {
      if (activeChat.kind === 'private') {
        await sendPrivateMessage(activeChat.id, input.trim())
      } else {
        await sendGroupMessage(activeChat.id, input.trim())
      }
      setInput('')
      await loadMessages(activeChat)
    } catch {
      alert('Nepavyko išsiųsti žinutės')
    } finally {
      setSending(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend() }
  }

  if (!user) return null

  const chatList = tab === 'private' ? privateChats : groupChats

  return (
    <div className="max-w-6xl mx-auto px-4 py-6">
      <h1 className="text-2xl font-bold text-gray-900 mb-4">Pokalbiai</h1>

      <div className="flex gap-4 h-[calc(100vh-180px)] min-h-[500px]">
        {/* ── Left: chat list ── */}
        <div className="w-72 flex-shrink-0 flex flex-col bg-white rounded-2xl border border-gray-200 overflow-hidden">
          {/* Tabs */}
          <div className="flex border-b border-gray-200">
            {(['private', 'group'] as Tab[]).map(t => (
              <button
                key={t}
                onClick={() => { setTab(t); setActiveChat(null); setMessages([]) }}
                className={`flex-1 py-3 text-sm font-medium transition-colors ${
                  tab === t
                    ? 'text-indigo-600 border-b-2 border-indigo-600'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                {t === 'private' ? 'Privatūs' : 'Grupiniai'}
              </button>
            ))}
          </div>

          {/* List */}
          <div className="flex-1 overflow-y-auto">
            {loadingChats ? (
              <p className="text-sm text-gray-400 p-4">Kraunama…</p>
            ) : chatList.length === 0 ? (
              <p className="text-sm text-gray-400 p-4">Nėra pokalbių.</p>
            ) : tab === 'private' ? (
              (privateChats as PrivateChat[]).map(c => {
                const isActive = activeChat?.kind === 'private' && activeChat.id === c.ID
                return (
                  <button
                    key={c.ID}
                    onClick={() => openChat({ kind: 'private', id: c.ID, name: c.OtherName })}
                    className={`w-full text-left px-4 py-3 border-b border-gray-100 hover:bg-gray-50 transition-colors ${isActive ? 'bg-indigo-50' : ''}`}
                  >
                    <div className="font-medium text-sm text-gray-800 truncate">{c.OtherName}</div>
                  </button>
                )
              })
            ) : (
              (groupChats as GroupChat[]).map(c => {
                const isActive = activeChat?.kind === 'group' && activeChat.id === c.RouteID
                return (
                  <button
                    key={c.RouteID}
                    onClick={() => openChat({ kind: 'group', id: c.RouteID, name: c.RouteName })}
                    className={`w-full text-left px-4 py-3 border-b border-gray-100 hover:bg-gray-50 transition-colors ${isActive ? 'bg-indigo-50' : ''}`}
                  >
                    <div className="font-medium text-sm text-gray-800 truncate">{c.RouteName}</div>
                  </button>
                )
              })
            )}
          </div>
        </div>

        {/* ── Right: chat view ── */}
        <div className="flex-1 flex flex-col bg-white rounded-2xl border border-gray-200 overflow-hidden">
          {!activeChat ? (
            <div className="flex-1 flex items-center justify-center text-gray-400 text-sm">
              Pasirinkite pokalbį
            </div>
          ) : (
            <>
              {/* Header */}
              <div className="px-5 py-3 border-b border-gray-200">
                <div className="font-semibold text-gray-900">{activeChat.name}</div>
                <div className="text-xs text-gray-400">{activeChat.kind === 'private' ? 'Privatus pokalbis' : 'Grupinis pokalbis'}</div>
              </div>

              {/* Messages */}
              <div className="flex-1 overflow-y-auto px-5 py-4 space-y-3">
                {loadingMsgs ? (
                  <p className="text-sm text-gray-400 text-center">Kraunama…</p>
                ) : messages.length === 0 ? (
                  <p className="text-sm text-gray-400 text-center">Dar nėra žinučių.</p>
                ) : (
                  messages.map(m => {
                    const isMine = m.SenderUserID === user.id
                    return (
                      <div key={m.ID} className={`flex flex-col ${isMine ? 'items-end' : 'items-start'}`}>
                        {!isMine && (
                          <span className="text-xs text-gray-500 mb-0.5 px-1">{m.SenderName}</span>
                        )}
                        <div className={`max-w-xs lg:max-w-md px-3 py-2 rounded-2xl text-sm break-words ${
                          isMine
                            ? 'bg-indigo-600 text-white rounded-br-sm'
                            : 'bg-gray-100 text-gray-800 rounded-bl-sm'
                        }`}>
                          {m.Message}
                        </div>
                        <span className="text-xs text-gray-400 mt-0.5 px-1">{fmtTime(m.CreatedAt)}</span>
                      </div>
                    )
                  })
                )}
                <div ref={bottomRef} />
              </div>

              {/* Input */}
              <div className="px-4 py-3 border-t border-gray-200 flex gap-2">
                <textarea
                  className="flex-1 border border-gray-300 rounded-xl px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-indigo-400"
                  rows={1}
                  placeholder="Rašyti žinutę… (Enter – siųsti)"
                  value={input}
                  onChange={e => setInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                />
                <button
                  onClick={handleSend}
                  disabled={sending || !input.trim()}
                  className="bg-indigo-600 text-white px-4 py-2 rounded-xl text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
                >
                  Siųsti
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
