import PocketBase from 'pocketbase'
import type { Book, LibraryState } from '@/lib/data'

// In production PocketBase serves this app, so the API is same-origin. During
// `next dev` the backend runs on its own port (see NEXT_PUBLIC_PB_URL).
const baseUrl =
  process.env.NEXT_PUBLIC_PB_URL || (typeof window === 'undefined' ? 'http://127.0.0.1:8091' : window.location.origin)

export const pb = new PocketBase(baseUrl)
pb.autoCancellation(false)

export async function signInWithGoogle() {
  await pb.collection('users').authWithOAuth2({ provider: 'google' })
}

export function signOut() {
  pb.authStore.clear()
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = pb.authStore.token
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', Authorization: token, ...init.headers },
  })
  if (response.status === 401 && pb.authStore.token === token) pb.authStore.clear()
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.message || 'Something went wrong. Please try again.')
  return data as T
}

export function api(action: string, body: Record<string, unknown> = {}) {
  return request<LibraryState>(`/api/dogear/${action}`, { method: 'POST', body: JSON.stringify(body) })
}

export function searchBooks(query: string, signal?: AbortSignal) {
  return request<Book[]>(`/api/dogear/search?q=${encodeURIComponent(query)}`, { signal })
}

export type FoundPerson = {
  id: string
  name: string
  relation: 'self' | 'friend' | 'sent' | 'incoming' | 'none'
}

/** Looks up a Dog-ear account by exact email. Throws "No one with that email…" when there is none. */
export function findPerson(email: string) {
  return request<FoundPerson>(`/api/dogear/people?email=${encodeURIComponent(email)}`)
}

export function fetchConfig() {
  return request<{ google: boolean }>('/api/dogear/config')
}
