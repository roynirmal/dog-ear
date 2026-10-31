'use client'

import { useEffect, useState } from 'react'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { thisMonth, type ShelfBook, type ShelfStatus } from '@/lib/data'

/**
 * Start and finish month inputs. A book being read only has a start; a finished
 * one has both; "want to read" has neither.
 */
export function ReadingMonthFields({
  id,
  status,
  started,
  finished,
  hint,
  onChange,
}: {
  id: string
  status: ShelfStatus
  started: string
  finished: string
  /** Shown under the fields, e.g. that blank is fine. */
  hint?: string
  onChange: (months: { started: string; finished: string }) => void
}) {
  if (status === 'want') return null
  const max = thisMonth()
  const invalid = !!started && !!finished && finished < started
  return (
    <div className="flex flex-col gap-2">
      <div className="grid grid-cols-2 gap-3">
        <div className="flex min-w-0 flex-col gap-2">
          <Label htmlFor={`${id}-started`}>
            Started{' '}
            {status === 'finished' ? <span className="font-normal text-muted-foreground">(optional)</span> : null}
          </Label>
          <Input
            id={`${id}-started`}
            type="month"
            max={max}
            value={started}
            onChange={(e) => onChange({ started: e.target.value, finished })}
            className="h-11 text-base"
          />
        </div>
        {status === 'finished' ? (
          <div className="flex min-w-0 flex-col gap-2">
            <Label htmlFor={`${id}-finished`}>
              Finished <span className="font-normal text-muted-foreground">(optional)</span>
            </Label>
            <Input
              id={`${id}-finished`}
              type="month"
              min={started || undefined}
              max={max}
              value={finished}
              aria-invalid={invalid || undefined}
              onChange={(e) => onChange({ started, finished: e.target.value })}
              className="h-11 text-base"
            />
          </div>
        ) : null}
      </div>
      {invalid ? (
        <p className="text-sm text-destructive">The finish month is before the start month.</p>
      ) : hint ? (
        <p className="text-xs text-muted-foreground">{hint}</p>
      ) : null}
    </div>
  )
}

export function monthsValid(status: ShelfStatus, started: string, finished: string) {
  const ok = (m: string) => m === '' || /^\d{4}-\d{2}$/.test(m)
  return ok(started) && ok(finished) && !(status === 'finished' && started && finished && finished < started)
}

export function ReadingMonthsDialog({ book, onClose }: { book: ShelfBook | null; onClose: () => void }) {
  const { setReadingMonths } = useLibrary()
  const [started, setStarted] = useState('')
  const [finished, setFinished] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!book) return
    setStarted(book.started ?? '')
    setFinished(book.finished ?? '')
  }, [book])

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!book || busy || !monthsValid(book.status, started, finished)) return
    setBusy(true)
    try {
      await setReadingMonths(book.id, book.status === 'finished' ? { started, finished } : { started })
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
          <DialogTitle className="font-serif text-xl">When did you read it?</DialogTitle>
          <DialogDescription>{book?.title}</DialogDescription>
        </DialogHeader>
        {book ? (
          <form onSubmit={handleSubmit} className="flex min-w-0 flex-col gap-4">
            <ReadingMonthFields
              id="edit-months"
              status={book.status}
              started={started}
              finished={finished}
              onChange={(m) => {
                setStarted(m.started)
                setFinished(m.finished)
              }}
            />
            <Button
              type="submit"
              className="h-12 rounded-full text-base"
              disabled={busy || !monthsValid(book.status, started, finished)}
            >
              {busy ? 'Saving…' : 'Save dates'}
            </Button>
          </form>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
