'use client'

import { useState } from 'react'
import { BookOpen, Check, ChevronRight, Clock, Copy, Search, Share2, UserPlus } from 'lucide-react'
import { FriendAvatar } from '@/components/friend-avatar'
import { useLibrary } from '@/components/library-provider'
import { usePersonCard } from '@/components/person-card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { findPerson, type FoundPerson } from '@/lib/api'
import { relativeTime, type FriendRequest } from '@/lib/data'

export function FriendsView() {
  const { posts, loans, library, friends, friendRequests, person, code } = useLibrary()
  const openPerson = usePersonCard()
  const [copied, setCopied] = useState(false)
  const people = friends.map(person).sort((a, b) => a.name.localeCompare(b.name))
  const link = typeof window === 'undefined' ? '' : `${window.location.origin}/?invite=${code}`
  const canShare = typeof navigator !== 'undefined' && typeof navigator.share === 'function'

  async function copy() {
    try {
      await navigator.clipboard.writeText(link)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      window.prompt('Copy your invite link', link)
    }
  }

  async function share() {
    try {
      await navigator.share({ title: 'Read with me on Dog-ear', text: 'Be my friend on Dog-ear', url: link })
    } catch {
      // Dismissed share sheets reject; nothing to do.
    }
  }

  return (
    <div className="flex flex-col gap-4 px-4">
      {friendRequests.length > 0 ? (
        <section aria-labelledby="friend-requests" className="flex flex-col gap-3">
          <h2 id="friend-requests" className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
            Friend requests
          </h2>
          {friendRequests.map((request) => (
            <FriendRequestCard key={request.id} request={request} />
          ))}
        </section>
      ) : null}

      <FindByEmail />

      <section
        aria-labelledby="invite-heading"
        className="flex flex-col gap-3 rounded-2xl bg-primary p-4 text-primary-foreground"
      >
        <div>
          <h2 id="invite-heading" className="font-serif text-lg font-semibold">
            Invite a friend
          </h2>
          <p className="text-sm opacity-80">
            Anyone who opens your link and signs in becomes your friend. Only share it with people you know.
          </p>
        </div>
        <p className="truncate rounded-xl bg-primary-foreground/10 px-3 py-2 font-mono text-xs">{link}</p>
        <div className="flex gap-2">
          <Button variant="secondary" className="h-11 flex-1 gap-1.5 rounded-full" onClick={copy}>
            {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
            {copied ? 'Copied' : 'Copy link'}
          </Button>
          {canShare ? (
            <Button variant="secondary" className="h-11 flex-1 gap-1.5 rounded-full" onClick={share}>
              <Share2 className="size-4" />
              Share
            </Button>
          ) : null}
        </div>
      </section>

      <p className="text-sm text-muted-foreground">{people.length === 1 ? '1 friend' : `${people.length} friends`}</p>
      {people.length > 0 ? (
        <ul className="flex flex-col divide-y rounded-2xl border bg-card">
          {people.map((friend) => {
            const current = posts.find((post) => post.authorId === friend.id && post.status === 'reading')
            const latest = current ?? posts.find((post) => post.authorId === friend.id)
            const active = loans.filter((loan) => loan.friendId === friend.id && !loan.returnedAt)
            const theyHave = active.filter((loan) => loan.direction === 'lent').length
            const youHave = active.filter((loan) => loan.direction === 'borrowed').length
            const lends = library.filter((book) => book.ownerId === friend.id && book.available).length

            return (
              <li key={friend.id}>
                <button
                  type="button"
                  onClick={() => openPerson(friend.id)}
                  className="flex w-full items-start gap-3 p-4 text-left outline-none hover:bg-muted/50 focus-visible:bg-muted/50"
                >
                  <FriendAvatar friendId={friend.id} />
                  <div className="min-w-0 flex-1">
                    <p className="font-semibold">{friend.name}</p>
                    {latest ? (
                      <p className="mt-0.5 flex items-center gap-1.5 text-sm text-muted-foreground">
                        <BookOpen aria-hidden="true" className="size-3.5 shrink-0" />
                        <span className="truncate">
                          {current ? 'Reading ' : 'Finished '}
                          <span className="font-serif text-foreground italic">{latest.book.title}</span>
                        </span>
                      </p>
                    ) : (
                      <p className="mt-0.5 text-sm text-muted-foreground">No updates yet</p>
                    )}
                    {theyHave + youHave + lends > 0 ? (
                      <div className="mt-2 flex flex-wrap gap-1.5">
                        {theyHave > 0 ? (
                          <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
                            Has {theyHave} of yours
                          </span>
                        ) : null}
                        {youHave > 0 ? (
                          <span className="rounded-full bg-accent/15 px-2 py-0.5 text-xs font-medium text-accent-foreground">
                            You have {youHave} of theirs
                          </span>
                        ) : null}
                        {lends > 0 ? (
                          <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                            {lends === 1 ? '1 book to borrow' : `${lends} books to borrow`}
                          </span>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                  <ChevronRight aria-hidden="true" className="mt-2 size-4 shrink-0 text-muted-foreground" />
                </button>
              </li>
            )
          })}
        </ul>
      ) : (
        <p className="rounded-2xl border border-dashed p-8 text-center text-sm text-muted-foreground">
          No friends yet. Send your link to someone you swap books with.
        </p>
      )}
    </div>
  )
}

function FriendRequestCard({ request }: { request: FriendRequest }) {
  const { person, respondToFriendRequest } = useLibrary()
  const openPerson = usePersonCard()
  const [busy, setBusy] = useState(false)

  async function respond(accept: boolean) {
    setBusy(true)
    try {
      await respondToFriendRequest(request.id, accept)
    } catch {
      setBusy(false)
    }
  }

  return (
    <article className="flex items-center gap-3 rounded-2xl border border-accent/40 bg-accent/5 p-4">
      <button type="button" onClick={() => openPerson(request.fromId)} className="shrink-0 rounded-full">
        <FriendAvatar friendId={request.fromId} />
        <span className="sr-only">View {person(request.fromId).name}</span>
      </button>
      <div className="min-w-0 flex-1">
        <p className="truncate font-semibold">{person(request.fromId).name}</p>
        <p className="text-xs text-muted-foreground">Asked {relativeTime(request.createdAt)}</p>
      </div>
      <Button className="h-10 rounded-full px-4" disabled={busy} onClick={() => respond(true)}>
        Accept
      </Button>
      <Button variant="ghost" className="h-10 px-2" disabled={busy} onClick={() => respond(false)}>
        Ignore
      </Button>
    </article>
  )
}

function FindByEmail() {
  const { friendRequests, sendFriendRequestByEmail, respondToFriendRequest } = useLibrary()
  const openPerson = usePersonCard()
  const [email, setEmail] = useState('')
  const [searched, setSearched] = useState('')
  const [found, setFound] = useState<FoundPerson | null>(null)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  async function search(event: React.FormEvent) {
    event.preventDefault()
    const query = email.trim()
    if (!query || busy) return
    setBusy(true)
    setFound(null)
    setMessage('')
    try {
      setFound(await findPerson(query))
      setSearched(query)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
    }
  }

  async function act(work: () => Promise<void>, relation: FoundPerson['relation']) {
    if (!found) return
    setBusy(true)
    try {
      await work()
      setFound({ ...found, relation })
    } catch {
      // The provider shows the error.
    } finally {
      setBusy(false)
    }
  }

  const incoming = found ? friendRequests.find((r) => r.fromId === found.id) : undefined

  return (
    <section aria-labelledby="find-heading" className="flex flex-col gap-3 rounded-2xl border bg-card p-4">
      <h2 id="find-heading" className="font-serif text-lg font-semibold">
        Find a friend by email
      </h2>
      <form onSubmit={search} className="flex gap-2">
        <label htmlFor="find-email" className="sr-only">
          Their email address
        </label>
        <Input
          id="find-email"
          type="email"
          inputMode="email"
          autoComplete="off"
          value={email}
          onChange={(e) => {
            setEmail(e.target.value)
            setMessage('')
          }}
          placeholder="friend@example.com"
          className="h-11 min-w-0 flex-1 text-base"
        />
        <Button type="submit" className="h-11 gap-1.5 rounded-full px-4" disabled={!email.trim() || busy}>
          <Search className="size-4" />
          Find
        </Button>
      </form>
      {message ? (
        <p role="status" className="text-sm text-muted-foreground">
          {message.startsWith('No one')
            ? `${message.replace(/\.$/, '')}. You can send them your invite link below.`
            : message}
        </p>
      ) : null}
      {found ? (
        <div className="flex items-center gap-3 rounded-xl bg-muted/50 p-3">
          <FriendAvatar
            friendId={found.relation === 'friend' ? found.id : undefined}
            name={found.relation === 'friend' ? undefined : found.name}
          />
          <div className="min-w-0 flex-1">
            <p className="truncate font-semibold">{found.relation === 'self' ? 'That’s you' : found.name}</p>
            <p className="truncate text-xs text-muted-foreground">{searched}</p>
          </div>
          {found.relation === 'none' ? (
            <Button
              className="h-10 shrink-0 gap-1.5 rounded-full px-3"
              disabled={busy}
              onClick={() => act(() => sendFriendRequestByEmail(searched), 'sent')}
            >
              <UserPlus className="size-4" />
              Add friend
            </Button>
          ) : found.relation === 'sent' ? (
            <span className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-muted-foreground">
              <Clock className="size-3.5" />
              Request sent
            </span>
          ) : found.relation === 'incoming' && incoming ? (
            <Button
              className="h-10 shrink-0 rounded-full px-3"
              disabled={busy}
              onClick={() => act(() => respondToFriendRequest(incoming.id, true), 'friend')}
            >
              Accept
            </Button>
          ) : found.relation === 'friend' ? (
            <Button variant="outline" className="h-10 shrink-0 rounded-full px-3" onClick={() => openPerson(found.id)}>
              Friends
            </Button>
          ) : null}
        </div>
      ) : null}
    </section>
  )
}
