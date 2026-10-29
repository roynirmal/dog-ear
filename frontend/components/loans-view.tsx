'use client'

import { useState } from 'react'
import { ArrowDownLeft, ArrowUpRight, Plus } from 'lucide-react'
import { BookCover } from '@/components/book-cover'
import { FilterChips } from '@/components/filter-chips'
import { FriendAvatar } from '@/components/friend-avatar'
import { useLibrary } from '@/components/library-provider'
import { LoanCard } from '@/components/loan-card'
import { Button } from '@/components/ui/button'
import { firstName, isOverdue, parseDayValue, relativeTime, type BorrowRequest } from '@/lib/data'

type Tab = 'lent' | 'borrowed' | 'returned'

export function LoansView({ onLogLoan }: { onLogLoan: () => void }) {
  const { loans, requests } = useLibrary()
  const [tab, setTab] = useState<Tab>('lent')

  const active = loans.filter((loan) => !loan.returnedAt)
  const lent = active.filter((loan) => loan.direction === 'lent')
  const borrowed = active.filter((loan) => loan.direction === 'borrowed')
  const returned = loans.filter((loan) => loan.returnedAt)
  const overdue = active.filter(isOverdue)

  const sortByDue = (a: (typeof loans)[number], b: (typeof loans)[number]) =>
    (a.dueDate ? parseDayValue(a.dueDate) : Infinity) - (b.dueDate ? parseDayValue(b.dueDate) : Infinity)

  const visible =
    tab === 'lent'
      ? [...lent].sort(sortByDue)
      : tab === 'borrowed'
        ? [...borrowed].sort(sortByDue)
        : [...returned].sort((a, b) => new Date(b.returnedAt!).getTime() - new Date(a.returnedAt!).getTime())

  return (
    <div className="flex flex-col gap-5 px-4">
      <section aria-label="Lending summary" className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-1 rounded-2xl bg-primary p-4 text-primary-foreground">
          <ArrowUpRight aria-hidden="true" className="size-5 opacity-70" />
          <p className="font-serif text-3xl font-semibold tabular-nums">{lent.length}</p>
          <p className="text-sm opacity-80">books lent out</p>
        </div>
        <div className="flex flex-col gap-1 rounded-2xl border bg-card p-4">
          <ArrowDownLeft aria-hidden="true" className="size-5 text-muted-foreground" />
          <p className="font-serif text-3xl font-semibold tabular-nums">{borrowed.length}</p>
          <p className="text-sm text-muted-foreground">books borrowed</p>
        </div>
      </section>

      {requests.length > 0 ? (
        <section aria-labelledby="borrow-requests" className="flex flex-col gap-3">
          <h2 id="borrow-requests" className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
            Asking to borrow
          </h2>
          {requests.map((request) => (
            <RequestCard key={request.id} request={request} />
          ))}
        </section>
      ) : null}

      {overdue.length > 0 ? (
        <p role="status" className="rounded-2xl bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {overdue.length === 1 ? '1 book is' : `${overdue.length} books are`} past due. A gentle nudge might help.
        </p>
      ) : null}

      <div className="flex items-center justify-between gap-3">
        <FilterChips
          label="Loan type"
          value={tab}
          onChange={setTab}
          options={[
            { value: 'lent', label: 'Lent', count: lent.length },
            { value: 'borrowed', label: 'Borrowed', count: borrowed.length },
            { value: 'returned', label: 'History', count: returned.length },
          ]}
        />
      </div>

      <div className="flex flex-col gap-3">
        {visible.map((loan) => (
          <LoanCard key={loan.id} loan={loan} />
        ))}
        {visible.length === 0 ? (
          <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed p-8 text-center">
            <p className="text-sm text-muted-foreground">
              {tab === 'returned' ? 'No returned books yet.' : 'Your shelf is all accounted for.'}
            </p>
            {tab !== 'returned' ? (
              <Button className="h-11 gap-1.5 rounded-full px-5" onClick={onLogLoan}>
                <Plus className="size-4" />
                Log a loan
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  )
}

function RequestCard({ request }: { request: BorrowRequest }) {
  const { person, respondToRequest } = useLibrary()
  const [busy, setBusy] = useState(false)
  const name = firstName(person(request.requesterId).name)

  async function respond(accept: boolean) {
    setBusy(true)
    try {
      await respondToRequest(request.id, accept)
    } catch {
      setBusy(false)
    }
  }

  return (
    <article className="flex gap-4 rounded-2xl border border-accent/40 bg-accent/5 p-4">
      <BookCover book={request.book} size="sm" />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <p className="text-sm">
          <span className="inline-flex items-center gap-1.5 align-middle">
            <FriendAvatar friendId={request.requesterId} size="sm" />
            <span className="font-semibold">{name}</span>
          </span>{' '}
          would like to borrow <span className="font-serif font-semibold italic">{request.book.title}</span>
        </p>
        <p className="text-xs text-muted-foreground">
          <time dateTime={request.createdAt}>{relativeTime(request.createdAt)}</time>
        </p>
        <div className="flex flex-wrap gap-2">
          <Button className="h-11 rounded-full px-4" disabled={busy} onClick={() => respond(true)}>
            Lend it
          </Button>
          <Button variant="ghost" className="h-11 px-3" disabled={busy} onClick={() => respond(false)}>
            Not now
          </Button>
        </div>
      </div>
    </article>
  )
}
