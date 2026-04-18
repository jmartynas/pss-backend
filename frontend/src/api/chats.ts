import { get, post } from './client'
import type { PrivateChat, GroupChat, ChatMessage } from '../types'

export const listPrivateChats = () => get<PrivateChat[]>('/chats/private')
export const listGroupChats = () => get<GroupChat[]>('/chats/group')

export const getPrivateMessages = (chatId: string) =>
  get<ChatMessage[]>(`/chats/private/${chatId}/messages`)

export const getGroupMessages = (routeId: string) =>
  get<ChatMessage[]>(`/chats/group/${routeId}/messages`)

export const sendPrivateMessage = (chatId: string, message: string) =>
  post<{ id: string }>(`/chats/private/${chatId}/messages`, { message })

export const sendGroupMessage = (routeId: string, message: string) =>
  post<{ id: string }>(`/chats/group/${routeId}/messages`, { message })
