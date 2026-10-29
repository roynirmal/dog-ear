'use client'

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { api, pb } from '@/lib/api'
import type { Book, LibraryState, LoanDirection, Person, ReadingStatus, ShelfStatus } from '@/lib/data'

type NewPost = {
  book: Book
  status: ReadingStatus
  progress?: number
  rating?: number
  thoughts: string
  canLend: boolean
}

type NewLoan = {
  direction: LoanDirection
  /** A friend's user ID, or empty with `contact` set for someone outside the app. */
  friend?: string
  contact?: string
  book: Book
  /** YYYY-MM-DD */
  due?: string
}

type NoteInput = {
  id?: string
  bookId?: string
  text: string
  tags: string[]
}

type LibraryContextValue = LibraryState & {
  person: (id: string) => Person
  refresh: () => Promise<void>
  addPost: (post: NewPost) => Promise<void>
  deletePost: (postId: string) => Promise<void>
  toggleLike: (postId: string) => Promise<void>
  addComment: (postId: string, text: string) => Promise<void>
  requestBorrow: (postId: string) => Promise<void>
  respondToRequest: (requestId: string, accept: boolean) => Promise<void>
  addLoan: (loan: NewLoan) => Promise<void>
  markReturned: (loanId: string) => Promise<void>
  undoReturned: (loanId: string) => Promise<void>
  nudge: (loanId: string) => Promise<void>
  deleteLoan: (loanId: string) => Promise<void>
  addToShelf: (book: Book, status: ShelfStatus) => Promise<void>
  setShelfStatus: (bookId: string, status: ShelfStatus) => Promise<void>
  removeFromShelf: (bookId: string) => Promise<void>
  saveNote: (note: NoteInput) => Promise<void>
  deleteNote: (noteId: string) => Promise<void>
  befriend: (code: string) => Promise<void>
  unfriend: (friendId: string) => Promise<void>
}

const LibraryContext = createContext<LibraryContextValue | null>(null)

const unknownPerson = (id: string): Person => ({ id, name: 'Former friend' })

export function LibraryProvider({
  initial,
  onError,
  children,
}: {
  initial: LibraryState
  onError: (message: string) => void
  children: React.ReactNode
}) {
  const [state, setState] = useState(initial)
  const latest = useRef(0)

  // Every action returns the whole library; apply only the newest response so a
  // slow refresh never overwrites a later change.
  const run = useCallback(
    async (action: string, body: Record<string, unknown> = {}) => {
      const ticket = ++latest.current
      try {
        const next = await api(action, body)
        if (ticket === latest.current) setState(next)
      } catch (error) {
        if (pb.authStore.isValid) onError(error instanceof Error ? error.message : String(error))
        throw error
      }
    },
    [onError],
  )

  const refresh = useCallback(() => run('state').catch(() => undefined), [run])

  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState === 'visible') refresh()
    }
    document.addEventListener('visibilitychange', onVisible)
    const timer = window.setInterval(onVisible, 60_000)
    return () => {
      document.removeEventListener('visibilitychange', onVisible)
      window.clearInterval(timer)
    }
  }, [refresh])

  const value = useMemo<LibraryContextValue>(() => {
    const people = new Map(state.people.map((p) => [p.id, p]))
    return {
      ...state,
      person: (id) => people.get(id) ?? unknownPerson(id),
      refresh,
      addPost: (post) => run('post', post),
      deletePost: (id) => run('post-delete', { id }),
      toggleLike: (id) => run('like', { id }),
      addComment: (id, text) => run('comment', { id, text }),
      requestBorrow: (id) => run('request', { id }),
      respondToRequest: (id, accept) => run('respond', { id, accept }),
      addLoan: (loan) => run('loan', loan),
      markReturned: (id) => run('return', { id }),
      undoReturned: (id) => run('unreturn', { id }),
      nudge: (id) => run('nudge', { id }),
      deleteLoan: (id) => run('loan-delete', { id }),
      addToShelf: (book, status) => run('shelf-add', { book, status }),
      setShelfStatus: (id, status) => run('shelf-update', { id, status }),
      removeFromShelf: (id) => run('shelf-remove', { id }),
      saveNote: (note) => run('note-save', note),
      deleteNote: (id) => run('note-delete', { id }),
      befriend: (code) => run('befriend', { code }),
      unfriend: (id) => run('unfriend', { id }),
    }
  }, [state, run, refresh])

  return <LibraryContext.Provider value={value}>{children}</LibraryContext.Provider>
}

export function useLibrary() {
  const context = useContext(LibraryContext)
  if (!context) throw new Error('useLibrary must be used within LibraryProvider')
  return context
}
