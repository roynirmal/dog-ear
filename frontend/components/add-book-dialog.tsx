'use client'

import { useEffect, useState } from 'react'
import { Camera, Search } from 'lucide-react'
import { BookPicker } from '@/components/book-picker'
import { FilterChips } from '@/components/filter-chips'
import { useLibrary } from '@/components/library-provider'
import { monthsValid, ReadingMonthFields } from '@/components/reading-months'
import { ShelfScan } from '@/components/shelf-scan'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { shelfStatusOptions, thisMonth, type Book, type ShelfStatus } from '@/lib/data'

export function AddBookDialog({
  open,
  onOpenChange,
  onAdded,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdded: () => void
}) {
  const { addToShelf } = useLibrary()
  const [mode, setMode] = useState<'search' | 'scan'>('search')
  const [book, setBook] = useState<Book | null>(null)
  const [status, setStatus] = useState<ShelfStatus>('want')
  const [busy, setBusy] = useState(false)
  const [started, setStarted] = useState('')
  const [finished, setFinished] = useState('')

  // A book you're reading started this month unless you say otherwise. A book
  // you already finished starts blank: you may not remember when you read it.
  useEffect(() => {
    setStarted(status === 'reading' ? thisMonth() : '')
    setFinished('')
  }, [status])

  useEffect(() => {
    if (open) {
      setBook(null)
      setStatus('want')
      setMode('search')
    }
  }, [open])

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!book || busy || !monthsValid(status, started, finished)) return
    setBusy(true)
    try {
      await addToShelf(
        book,
        status,
        status === 'want' ? undefined : status === 'reading' ? { started } : { started, finished },
      )
      onOpenChange(false)
      onAdded()
    } catch {
      // The provider shows the error.
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={`max-h-[90dvh] overflow-y-auto rounded-2xl p-5 ${mode === 'scan' ? 'sm:max-w-lg' : 'sm:max-w-md'}`}
      >
        <DialogHeader>
          <DialogTitle className="font-serif text-xl">Add to your shelf</DialogTitle>
          <DialogDescription>
            {mode === 'scan'
              ? 'Photograph a bookshelf and add several books at once.'
              : 'Books you own, are reading, or want to read.'}
          </DialogDescription>
        </DialogHeader>
        <div
          className="grid grid-cols-2 gap-1 rounded-full border bg-muted/40 p-1"
          role="tablist"
          aria-label="Add method"
        >
          <button
            type="button"
            role="tab"
            aria-selected={mode === 'search'}
            onClick={() => setMode('search')}
            className={`flex h-10 items-center justify-center gap-2 rounded-full text-sm font-medium transition-colors ${
              mode === 'search' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <Search className="size-4" />
            Search
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mode === 'scan'}
            onClick={() => setMode('scan')}
            className={`flex h-10 items-center justify-center gap-2 rounded-full text-sm font-medium transition-colors ${
              mode === 'scan' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <Camera className="size-4" />
            Scan shelf
          </button>
        </div>
        {mode === 'search' ? (
          <form onSubmit={handleSubmit} className="flex min-w-0 flex-col gap-4">
            <BookPicker id="shelf-book" value={book} onChange={setBook} />
            <FilterChips
              label="Shelf"
              wrap
              value={status}
              onChange={setStatus}
              options={shelfStatusOptions.map((o) =>
                o.value === 'finished' ? { ...o, label: 'Already finished' } : o,
              )}
            />
            <ReadingMonthFields
              id="shelf-months"
              status={status}
              started={started}
              finished={finished}
              hint={status === 'finished' ? 'Leave these blank if you don’t remember.' : undefined}
              onChange={(m) => {
                setStarted(m.started)
                setFinished(m.finished)
              }}
            />
            <Button
              type="submit"
              className="h-12 rounded-full text-base"
              disabled={!book || busy || !monthsValid(status, started, finished)}
            >
              {busy ? 'Adding…' : 'Add to shelf'}
            </Button>
          </form>
        ) : (
          <ShelfScan
            onAdded={() => {
              onOpenChange(false)
              onAdded()
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}
