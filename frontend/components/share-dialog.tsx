'use client'

import { useEffect, useState } from 'react'
import { Star } from 'lucide-react'
import { BookPicker } from '@/components/book-picker'
import { FilterChips } from '@/components/filter-chips'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { Book, ReadingStatus } from '@/lib/data'
import { cn } from '@/lib/utils'

export function ShareDialog({
  open,
  initialBook,
  onOpenChange,
  onShared,
}: {
  open: boolean
  initialBook?: Book
  onOpenChange: (open: boolean) => void
  onShared: () => void
}) {
  const { addPost, shelf } = useLibrary()
  const [book, setBook] = useState<Book | null>(null)
  const [busy, setBusy] = useState(false)
  const [status, setStatus] = useState<ReadingStatus>('reading')
  const [progress, setProgress] = useState(25)
  const [rating, setRating] = useState(0)
  const [thoughts, setThoughts] = useState('')
  const [canLend, setCanLend] = useState(true)

  // Start from the shelf copy's lending setting when the book is already there.
  useEffect(() => {
    if (!book) return
    const copy = shelf.find((b) =>
      book.olKey
        ? b.olKey === book.olKey
        : b.title.toLowerCase() === book.title.toLowerCase() && b.author.toLowerCase() === book.author.toLowerCase(),
    )
    setCanLend(copy ? copy.lendable : true)
    // Only when the chosen book changes, not on background refreshes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [book])

  useEffect(() => {
    if (open) reset(initialBook)
  }, [open, initialBook])

  function reset(start?: Book) {
    setBook(start ?? null)
    setStatus('reading')
    setProgress(25)
    setRating(0)
    setThoughts('')
    setCanLend(true)
  }

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!book || busy) return
    setBusy(true)
    try {
      await addPost({
        book,
        status,
        progress: status === 'reading' ? progress : undefined,
        rating: status === 'finished' && rating > 0 ? rating : undefined,
        thoughts: thoughts.trim(),
        canLend,
      })
      onOpenChange(false)
      onShared()
    } catch {
      // The provider shows the error; keep the form so nothing is lost.
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto rounded-2xl p-5 sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-serif text-xl">Share a book</DialogTitle>
          <DialogDescription>Let your friends know what you&apos;re reading.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex min-w-0 flex-col gap-4">
          <FilterChips
            label="Reading status"
            value={status}
            onChange={setStatus}
            options={[
              { value: 'reading', label: 'Reading now' },
              { value: 'finished', label: 'Finished it' },
            ]}
          />
          <BookPicker id="share-book" value={book} onChange={setBook} />

          {status === 'reading' ? (
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="share-progress">How far along?</Label>
                <span className="text-sm font-medium tabular-nums">{progress}%</span>
              </div>
              <input
                id="share-progress"
                type="range"
                min={0}
                max={100}
                step={5}
                value={progress}
                onChange={(e) => setProgress(Number(e.target.value))}
                className="h-11 w-full accent-primary"
              />
            </div>
          ) : (
            <fieldset className="flex flex-col gap-2">
              <legend className="mb-2 text-sm font-medium">Your rating</legend>
              <div className="flex gap-1">
                {Array.from({ length: 5 }, (_, i) => {
                  const value = i + 1
                  return (
                    <button
                      key={value}
                      type="button"
                      aria-label={`${value} star${value > 1 ? 's' : ''}`}
                      aria-pressed={rating === value}
                      onClick={() => setRating(rating === value ? 0 : value)}
                      className="flex size-11 items-center justify-center rounded-lg outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                    >
                      <Star
                        aria-hidden="true"
                        className={cn('size-7', value <= rating ? 'fill-accent text-accent' : 'text-border')}
                      />
                    </button>
                  )
                })}
              </div>
            </fieldset>
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor="share-thoughts">Your thoughts</Label>
            <Textarea
              id="share-thoughts"
              value={thoughts}
              onChange={(e) => setThoughts(e.target.value)}
              placeholder="What’s sticking with you?"
              rows={4}
              className="text-base"
            />
          </div>

          <label className="flex min-h-11 items-center gap-3 text-sm">
            <input
              type="checkbox"
              checked={canLend}
              onChange={(e) => setCanLend(e.target.checked)}
              className="size-5 accent-primary"
            />
            Friends can borrow my copy
          </label>

          <Button type="submit" className="h-12 rounded-full text-base" disabled={!book || busy}>
            {busy ? 'Sharing…' : 'Share with friends'}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
