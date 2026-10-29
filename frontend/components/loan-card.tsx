'use client'

import { useState } from 'react'
import { BellRing, Check, RotateCcw, Trash2 } from 'lucide-react'
import { BookCover } from '@/components/book-cover'
import { FriendAvatar } from '@/components/friend-avatar'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'
import { daysUntil, firstName, relativeTime, shortDate, type Loan } from '@/lib/data'
import { cn } from '@/lib/utils'

function DueBadge({ loan }: { loan: Loan }) {
  if (loan.returnedAt) {
    return (
      <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
        Returned {shortDate(loan.returnedAt)}
      </span>
    )
  }
  if (!loan.dueDate) {
    return (
      <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">No due date</span>
    )
  }
  const days = daysUntil(loan.dueDate)
  const overdue = days < 0
  const soon = days >= 0 && days <= 7
  return (
    <span
      className={cn(
        'rounded-full px-2 py-0.5 text-xs font-medium',
        overdue && 'bg-destructive/10 text-destructive',
        soon && 'bg-accent/15 text-accent-foreground',
        !overdue && !soon && 'bg-primary/10 text-primary',
      )}
    >
      {overdue ? `${Math.abs(days)}d overdue` : days === 0 ? 'Due today' : `Due in ${days}d`}
    </span>
  )
}

export function LoanCard({ loan }: { loan: Loan }) {
  const { person, markReturned, undoReturned, nudge, deleteLoan } = useLibrary()
  const [busy, setBusy] = useState(false)
  const name = loan.friendId ? firstName(person(loan.friendId).name) : (loan.contact ?? 'someone')
  // Nudges are visible to both sides; a fresh one disables the button for a day.
  const recentlyNudged = !!loan.nudgedAt && Date.now() - new Date(loan.nudgedAt).getTime() < 86_400_000

  async function act(work: () => Promise<void>) {
    setBusy(true)
    try {
      await work()
    } catch {
      // The provider shows the error.
    } finally {
      setBusy(false)
    }
  }

  return (
    <article className="flex gap-4 rounded-2xl border bg-card p-4">
      <BookCover book={loan.book} size="sm" className={cn(loan.returnedAt && 'opacity-60 grayscale')} />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <div>
          <h3 className="font-serif leading-snug font-semibold text-balance">{loan.book.title}</h3>
          <p className="text-sm text-muted-foreground">{loan.book.author}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1.5 text-sm">
            <FriendAvatar friendId={loan.friendId} name={loan.contact} size="sm" />
            <span>
              {loan.direction === 'lent' ? 'With ' : 'From '}
              <span className="font-medium">{name}</span>
            </span>
          </span>
          <span className="text-xs text-muted-foreground">since {shortDate(loan.since)}</span>
        </div>
        {loan.nudgedAt && !loan.returnedAt ? (
          <p className="text-xs text-accent-foreground">
            <BellRing aria-hidden="true" className="mr-1 inline size-3.5 align-[-2px]" />
            {loan.direction === 'lent' ? 'You nudged' : `${name} nudged you`} {relativeTime(loan.nudgedAt)}
          </p>
        ) : null}
        <div className="flex flex-wrap items-center justify-between gap-2">
          <DueBadge loan={loan} />
          <div className="flex items-center gap-1">
            {loan.returnedAt ? (
              <>
                <Button
                  variant="ghost"
                  className="h-11 gap-1.5 px-3"
                  disabled={busy}
                  onClick={() => act(() => undoReturned(loan.id))}
                >
                  <RotateCcw className="size-4" />
                  Undo
                </Button>
                {loan.mine ? (
                  <Button
                    variant="ghost"
                    className="h-11 px-3 text-muted-foreground"
                    disabled={busy}
                    onClick={() => {
                      if (window.confirm('Remove this loan from your history?')) act(() => deleteLoan(loan.id))
                    }}
                  >
                    <Trash2 className="size-4" />
                    <span className="sr-only">Delete loan</span>
                  </Button>
                ) : null}
              </>
            ) : (
              <>
                {loan.direction === 'lent' && loan.friendId ? (
                  <Button
                    variant="ghost"
                    className="h-11 gap-1.5 px-3"
                    disabled={busy || recentlyNudged}
                    onClick={() => act(() => nudge(loan.id))}
                  >
                    <BellRing className="size-4" />
                    {recentlyNudged ? 'Nudged' : 'Nudge'}
                  </Button>
                ) : null}
                <Button
                  variant="outline"
                  className="h-11 gap-1.5 rounded-full px-4"
                  disabled={busy}
                  onClick={() => act(() => markReturned(loan.id))}
                >
                  <Check className="size-4" />
                  Returned
                </Button>
              </>
            )}
          </div>
        </div>
      </div>
    </article>
  )
}
