'use client'

import { createContext, useContext, useState } from 'react'
import { BookMarked, BookOpen, Check, Clock, UserMinus, UserPlus } from 'lucide-react'
import { BookCover } from '@/components/book-cover'
import { FriendAvatar } from '@/components/friend-avatar'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { firstName, type FriendBook } from '@/lib/data'
import { cn } from '@/lib/utils'

const PersonCardContext = createContext<(id: string) => void>(() => undefined)

/** Opens the profile card for anyone shown in the app. */
export function usePersonCard() {
  return useContext(PersonCardContext)
}

export function PersonCardProvider({ children }: { children: React.ReactNode }) {
  const [personId, setPersonId] = useState<string | null>(null)
  return (
    <PersonCardContext.Provider value={setPersonId}>
      {children}
      <PersonCard personId={personId} onClose={() => setPersonId(null)} />
    </PersonCardContext.Provider>
  )
}

/** A person's avatar and/or name that opens their card when tapped. */
export function PersonButton({
  personId,
  className,
  children,
}: {
  personId: string
  className?: string
  children: React.ReactNode
}) {
  const open = usePersonCard()
  return (
    <button
      type="button"
      onClick={() => open(personId)}
      className={cn(
        'rounded-md text-left outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50',
        className,
      )}
    >
      {children}
    </button>
  )
}

function PersonCard({ personId, onClose }: { personId: string | null; onClose: () => void }) {
  const {
    me,
    person,
    friends,
    friendRequests,
    sentRequests,
    posts,
    library,
    sendFriendRequest,
    respondToFriendRequest,
    unfriend,
  } = useLibrary()
  const [busy, setBusy] = useState(false)
  const id = personId ?? ''
  const name = person(id).name
  const isMe = id === me.id
  const isFriend = friends.includes(id)
  const incoming = friendRequests.find((r) => r.fromId === id)
  const sent = sentRequests.includes(id)
  const reading = posts.find((p) => p.authorId === id && p.status === 'reading')
  const books = library.filter((b) => b.ownerId === id)

  async function act(work: () => Promise<void>, close = false) {
    setBusy(true)
    try {
      await work()
      if (close) onClose()
    } catch {
      // The provider shows the error.
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={personId !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto rounded-2xl p-5 sm:max-w-md">
        <DialogHeader className="items-center text-center">
          {personId ? <FriendAvatar friendId={id} size="lg" /> : null}
          <DialogTitle className="font-serif text-xl">{isMe ? 'You' : name}</DialogTitle>
          <DialogDescription>
            {isMe
              ? 'This is how friends see you.'
              : isFriend
                ? 'Friends on Dog-ear'
                : incoming
                  ? `${firstName(name)} wants to be friends`
                  : sent
                    ? 'Friend request sent'
                    : 'Not friends yet'}
          </DialogDescription>
        </DialogHeader>

        {!isMe && !isFriend ? (
          incoming ? (
            <div className="flex gap-2">
              <Button
                className="h-11 flex-1 gap-1.5 rounded-full"
                disabled={busy}
                onClick={() => act(() => respondToFriendRequest(incoming.id, true))}
              >
                <Check className="size-4" />
                Accept
              </Button>
              <Button
                variant="ghost"
                className="h-11 px-4"
                disabled={busy}
                onClick={() => act(() => respondToFriendRequest(incoming.id, false), true)}
              >
                Not now
              </Button>
            </div>
          ) : sent ? (
            <Button variant="secondary" className="h-11 gap-1.5 rounded-full" disabled>
              <Clock className="size-4" />
              Request sent
            </Button>
          ) : (
            <Button
              className="h-11 gap-1.5 rounded-full"
              disabled={busy}
              onClick={() => act(() => sendFriendRequest(id))}
            >
              <UserPlus className="size-4" />
              Add friend
            </Button>
          )
        ) : null}

        {isFriend ? (
          <>
            {reading ? (
              <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                <BookOpen aria-hidden="true" className="size-4 shrink-0" />
                <span className="truncate">
                  Reading <span className="font-serif text-foreground italic">{reading.book.title}</span>
                </span>
              </p>
            ) : null}
            <section aria-labelledby="friend-shelf" className="flex flex-col gap-2">
              <h3 id="friend-shelf" className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
                Books {firstName(name)} lends
              </h3>
              {books.length > 0 ? (
                <ul className="flex flex-col divide-y rounded-2xl border bg-card">
                  {books.map((book) => (
                    <LendableBookRow key={book.id} book={book} />
                  ))}
                </ul>
              ) : (
                <p className="rounded-2xl border border-dashed p-4 text-center text-sm text-muted-foreground">
                  Nothing on {firstName(name)}’s lending shelf yet.
                </p>
              )}
            </section>
            <Button
              variant="ghost"
              className="h-11 gap-1.5 text-muted-foreground"
              disabled={busy}
              onClick={() => {
                if (window.confirm(`Remove ${name} from your friends? Loans between you are kept.`))
                  act(() => unfriend(id), true)
              }}
            >
              <UserMinus className="size-4" />
              Remove friend
            </Button>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function LendableBookRow({ book }: { book: FriendBook }) {
  const { requestBorrow } = useLibrary()
  const [busy, setBusy] = useState(false)
  return (
    <li className="flex items-center gap-3 p-3">
      <BookCover book={book} size="sm" />
      <div className="min-w-0 flex-1">
        <p className="font-serif leading-snug font-semibold">{book.title}</p>
        <p className="truncate text-sm text-muted-foreground">{book.author}</p>
      </div>
      {!book.available ? (
        <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
          Lent out
        </span>
      ) : book.requested ? (
        <span className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-muted-foreground">
          <Check className="size-3.5" />
          Requested
        </span>
      ) : (
        <Button
          variant="outline"
          className="h-10 shrink-0 gap-1.5 rounded-full px-3"
          disabled={busy}
          onClick={async () => {
            setBusy(true)
            await requestBorrow(book.id).catch(() => undefined)
            setBusy(false)
          }}
        >
          <BookMarked className="size-4" />
          Borrow
        </Button>
      )}
    </li>
  )
}
