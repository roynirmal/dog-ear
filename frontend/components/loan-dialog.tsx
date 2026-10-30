'use client'

import { useEffect, useState } from 'react'
import { BookPicker } from '@/components/book-picker'
import { FilterChips } from '@/components/filter-chips'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Book, LoanDirection, ShelfBook } from '@/lib/data'

const SOMEONE_ELSE = ''

export function LoanDialog({
  open,
  initialBook,
  onOpenChange,
  onLogged,
}: {
  open: boolean
  initialBook?: Book
  onOpenChange: (open: boolean) => void
  onLogged: () => void
}) {
  const { addLoan, friends, person } = useLibrary()
  const people = friends.map(person).sort((a, b) => a.name.localeCompare(b.name))
  const [direction, setDirection] = useState<LoanDirection>('lent')
  const [friendId, setFriendId] = useState(SOMEONE_ELSE)
  const [contact, setContact] = useState('')
  const [book, setBook] = useState<Book | null>(null)
  const [dueDate, setDueDate] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setDirection('lent')
    setFriendId(
      friends.length ? [...friends].sort((a, b) => person(a).name.localeCompare(person(b).name))[0] : SOMEONE_ELSE,
    )
    setContact('')
    setBook(initialBook ?? null)
    setDueDate('')
    // Reset only when the dialog opens, not as friends refresh in the background.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initialBook])

  // Lending straight from your shelf links that copy, so friends see it as lent out.
  const shelfBookId =
    direction === 'lent' && initialBook && book === initialBook && 'id' in initialBook
      ? (initialBook as ShelfBook).id
      : undefined

  const ready = !!book && (friendId !== SOMEONE_ELSE || contact.trim() !== '')

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!book || !ready || busy) return
    setBusy(true)
    try {
      await addLoan({
        direction,
        friend: friendId || undefined,
        contact: friendId ? undefined : contact.trim(),
        book,
        shelfBook: shelfBookId,
        due: dueDate || undefined,
      })
      onOpenChange(false)
      onLogged()
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
          <DialogTitle className="font-serif text-xl">Log a loan</DialogTitle>
          <DialogDescription>Keep track of who has which book.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex min-w-0 flex-col gap-4">
          <FilterChips
            label="Loan direction"
            value={direction}
            onChange={setDirection}
            options={[
              { value: 'lent', label: 'I lent a book' },
              { value: 'borrowed', label: 'I borrowed one' },
            ]}
          />
          <div className="flex flex-col gap-2">
            <Label htmlFor="loan-friend">{direction === 'lent' ? 'Lent to' : 'Borrowed from'}</Label>
            <select
              id="loan-friend"
              value={friendId}
              onChange={(e) => setFriendId(e.target.value)}
              className="h-11 rounded-lg border border-input bg-transparent px-3 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              {people.map((friend) => (
                <option key={friend.id} value={friend.id}>
                  {friend.name}
                </option>
              ))}
              <option value={SOMEONE_ELSE}>Someone not on Dog-ear…</option>
            </select>
            {friendId === SOMEONE_ELSE ? (
              <Input
                aria-label="Their name"
                value={contact}
                onChange={(e) => setContact(e.target.value)}
                placeholder="Their name"
                maxLength={120}
                className="h-11 text-base"
              />
            ) : (
              <p className="text-xs text-muted-foreground">
                {person(friendId).name.split(' ')[0]} will see this loan too, and either of you can mark it returned.
              </p>
            )}
          </div>
          <BookPicker id="loan-book" value={book} onChange={setBook} />
          <div className="flex flex-col gap-2">
            <Label htmlFor="loan-due">
              Return by <span className="font-normal text-muted-foreground">(optional)</span>
            </Label>
            <Input
              id="loan-due"
              type="date"
              value={dueDate}
              onChange={(e) => setDueDate(e.target.value)}
              className="h-11 text-base"
            />
          </div>
          <Button type="submit" className="h-12 rounded-full text-base" disabled={!ready || busy}>
            {busy ? 'Saving…' : 'Save loan'}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
