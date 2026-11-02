'use client'

import { createContext, useContext, useEffect, useState } from 'react'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import type { ShelfBook } from '@/lib/data'

const ProgressContext = createContext<(shelfBookId: string) => void>(() => undefined)

/** Opens "Update progress" for one of your shelf books you're reading. */
export function useUpdateProgress() {
  return useContext(ProgressContext)
}

export function ProgressProvider({
  onFinished,
  children,
}: {
  /** "Finished it?" hands the book to the share dialog. */
  onFinished: (book: ShelfBook) => void
  children: React.ReactNode
}) {
  const { shelf } = useLibrary()
  const [bookId, setBookId] = useState<string | null>(null)
  const book = shelf.find((b) => b.id === bookId && b.status === 'reading') ?? null
  return (
    <ProgressContext.Provider value={setBookId}>
      {children}
      <ProgressDialog
        book={book}
        onClose={() => setBookId(null)}
        onFinished={(b) => {
          setBookId(null)
          onFinished(b)
        }}
      />
    </ProgressContext.Provider>
  )
}

function ProgressDialog({
  book,
  onClose,
  onFinished,
}: {
  book: ShelfBook | null
  onClose: () => void
  onFinished: (book: ShelfBook) => void
}) {
  const { updateProgress } = useLibrary()
  const [progress, setProgress] = useState(0)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (book) setProgress(book.progress)
    // Only when a different book opens, not on background refreshes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [book?.id])

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!book || busy) return
    setBusy(true)
    try {
      await updateProgress(book.id, progress)
      onClose()
    } catch {
      // The provider shows the error.
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={book !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="rounded-2xl p-5 sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-serif text-xl">Update progress</DialogTitle>
          <DialogDescription>{book?.title}</DialogDescription>
        </DialogHeader>
        {book ? (
          <form onSubmit={handleSubmit} className="flex min-w-0 flex-col gap-4">
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="update-progress">How far along?</Label>
                <span className="text-sm font-medium tabular-nums">{progress}%</span>
              </div>
              <input
                id="update-progress"
                type="range"
                min={0}
                max={100}
                step={5}
                value={progress}
                onChange={(e) => setProgress(Number(e.target.value))}
                className="h-11 w-full accent-primary"
              />
              <p className="text-xs text-muted-foreground">
                Your post about this book shows the new progress. Nothing new is posted.
              </p>
            </div>
            <Button type="submit" className="h-12 rounded-full text-base" disabled={busy || progress === book.progress}>
              {busy ? 'Saving…' : 'Save progress'}
            </Button>
            <button
              type="button"
              onClick={() => onFinished(book)}
              className="self-center text-sm font-medium text-primary underline-offset-4 hover:underline"
            >
              Finished it? Share that instead
            </button>
          </form>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
