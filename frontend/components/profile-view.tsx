'use client'

import { useState } from 'react'
import {
  ArrowLeftRight,
  BookPlus,
  Download,
  HandHeart,
  Lock,
  LogOut,
  NotebookPen,
  PenLine,
  Share,
  Trash2,
} from 'lucide-react'
import { shelfStatusOptions } from '@/components/add-book-dialog'
import { BookCover } from '@/components/book-cover'
import { FilterChips } from '@/components/filter-chips'
import { FriendAvatar } from '@/components/friend-avatar'
import { useLibrary } from '@/components/library-provider'
import { PostCard } from '@/components/post-card'
import { useInstall } from '@/components/service-worker'
import { Button } from '@/components/ui/button'
import { signOut } from '@/lib/api'
import { cn } from '@/lib/utils'
import { firstName, relativeTime, type Book, type Note, type ShelfBook, type ShelfStatus } from '@/lib/data'

type Section = 'shelf' | 'notes' | 'thoughts'

export function ProfileView({
  onShare,
  onLend,
  onAddBook,
  onNote,
}: {
  onShare: (book?: Book) => void
  onLend: (book: Book) => void
  onAddBook: () => void
  onNote: (options: { note?: Note; bookId?: string }) => void
}) {
  const { me, posts, loans, friends, shelf, notes } = useLibrary()
  const [section, setSection] = useState<Section>('shelf')
  const mine = posts.filter((post) => post.authorId === me.id)

  const stats = [
    { label: 'On shelf', value: shelf.length },
    { label: 'Finished', value: shelf.filter((book) => book.status === 'finished').length },
    { label: 'Ever lent', value: loans.filter((loan) => loan.direction === 'lent').length },
  ]

  return (
    <div className="flex flex-col gap-6 px-4">
      <section className="flex flex-col items-center gap-3 rounded-2xl border bg-card p-6 text-center">
        <FriendAvatar friendId={me.id} size="lg" />
        <div>
          <h2 className="font-serif text-2xl font-semibold">{me.name}</h2>
          <p className="text-sm text-muted-foreground">
            reading with {friends.length === 1 ? '1 friend' : `${friends.length} friends`}
          </p>
        </div>
        <dl className="mt-2 grid w-full grid-cols-3 divide-x">
          {stats.map((stat) => (
            <div key={stat.label} className="flex flex-col-reverse gap-0.5">
              <dt className="text-xs text-muted-foreground">{stat.label}</dt>
              <dd className="font-serif text-2xl font-semibold tabular-nums">{stat.value}</dd>
            </div>
          ))}
        </dl>
        <InstallApp />
        <Button variant="ghost" className="h-11 gap-1.5 px-3 text-muted-foreground" onClick={signOut}>
          <LogOut className="size-4" />
          Sign out
        </Button>
      </section>

      <FilterChips
        label="Your library"
        value={section}
        onChange={setSection}
        options={[
          { value: 'shelf', label: 'Shelf', count: shelf.length },
          { value: 'notes', label: 'Notes', count: notes.length },
          { value: 'thoughts', label: 'Thoughts', count: mine.length },
        ]}
      />

      {section === 'shelf' ? (
        <ShelfSection onShare={onShare} onLend={onLend} onAddBook={onAddBook} onNote={onNote} />
      ) : null}
      {section === 'notes' ? <NotesSection onNote={onNote} /> : null}
      {section === 'thoughts' ? (
        <section aria-label="Your thoughts" className="flex flex-col gap-4">
          {mine.map((post) => (
            <PostCard key={post.id} post={post} />
          ))}
          {mine.length === 0 ? (
            <EmptyState
              text="You haven’t shared anything yet. What’s on your nightstand?"
              action="Share what you're reading"
              icon={PenLine}
              onClick={() => onShare()}
            />
          ) : null}
        </section>
      ) : null}
    </div>
  )
}

function ShelfSection({
  onShare,
  onLend,
  onAddBook,
  onNote,
}: {
  onShare: (book: Book) => void
  onLend: (book: Book) => void
  onAddBook: () => void
  onNote: (options: { bookId?: string }) => void
}) {
  const { shelf, loans, notes, person, setShelfStatus, setLendable, removeFromShelf } = useLibrary()
  const [filter, setFilter] = useState<ShelfStatus | 'all'>('all')
  const visible = filter === 'all' ? shelf : shelf.filter((book) => book.status === filter)

  function lentTo(book: ShelfBook) {
    const loan = loans.find(
      (l) =>
        l.direction === 'lent' &&
        !l.returnedAt &&
        (l.shelfBookId
          ? l.shelfBookId === book.id
          : book.olKey
            ? l.book.olKey === book.olKey
            : l.book.title.toLowerCase() === book.title.toLowerCase()),
    )
    if (!loan) return null
    return loan.friendId ? firstName(person(loan.friendId).name) : (loan.contact ?? 'someone')
  }

  return (
    <section aria-label="Your shelf" className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <div className="min-w-0 flex-1">
          <FilterChips
            label="Shelf filter"
            value={filter}
            onChange={setFilter}
            options={[
              { value: 'all', label: 'All' },
              ...shelfStatusOptions.map((o) => ({ ...o, count: shelf.filter((b) => b.status === o.value).length })),
            ]}
          />
        </div>
      </div>
      <Button variant="outline" className="h-11 gap-1.5 rounded-full" onClick={onAddBook}>
        <BookPlus className="size-4" />
        Add a book
      </Button>
      {visible.map((book) => {
        const holder = lentTo(book)
        const noteCount = notes.filter((n) => n.bookId === book.id).length
        return (
          <article key={book.id} className="flex gap-4 rounded-2xl border bg-card p-4">
            <BookCover book={book} />
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <div>
                <h3 className="font-serif leading-snug font-semibold text-balance">{book.title}</h3>
                <p className="text-sm text-muted-foreground">{[book.author, book.year].filter(Boolean).join(' · ')}</p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <label className="sr-only" htmlFor={`status-${book.id}`}>
                  Shelf status
                </label>
                <select
                  id={`status-${book.id}`}
                  value={book.status}
                  onChange={(e) => setShelfStatus(book.id, e.target.value as ShelfStatus).catch(() => undefined)}
                  className="h-9 rounded-full border border-input bg-background px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                >
                  {shelfStatusOptions.map((o) => (
                    <option key={o.value} value={o.value}>
                      {o.label}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  role="switch"
                  aria-checked={book.lendable}
                  onClick={() => setLendable(book.id, !book.lendable).catch(() => undefined)}
                  className={cn(
                    'inline-flex h-9 items-center gap-1.5 rounded-full border px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
                    book.lendable
                      ? 'border-primary/30 bg-primary/10 text-primary'
                      : 'bg-background text-muted-foreground',
                  )}
                >
                  {book.lendable ? <HandHeart className="size-4" /> : <Lock className="size-4" />}
                  {book.lendable ? 'Friends can borrow' : 'Not for lending'}
                </button>
                {holder ? (
                  <span className="rounded-full bg-accent/15 px-2 py-0.5 text-xs font-medium text-accent-foreground">
                    With {holder}
                  </span>
                ) : null}
                {noteCount > 0 ? (
                  <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                    {noteCount === 1 ? '1 note' : `${noteCount} notes`}
                  </span>
                ) : null}
              </div>
              <div className="-mx-2 flex flex-wrap items-center">
                <Button variant="ghost" className="h-11 gap-1.5 px-2" onClick={() => onShare(book)}>
                  <PenLine className="size-4" />
                  Share
                </Button>
                <Button variant="ghost" className="h-11 gap-1.5 px-2" onClick={() => onLend(book)}>
                  <ArrowLeftRight className="size-4" />
                  Lend
                </Button>
                <Button variant="ghost" className="h-11 gap-1.5 px-2" onClick={() => onNote({ bookId: book.id })}>
                  <NotebookPen className="size-4" />
                  Note
                </Button>
                <Button
                  variant="ghost"
                  className="ml-auto h-11 px-2 text-muted-foreground"
                  onClick={() => {
                    if (window.confirm(`Remove ${book.title} from your shelf? Its notes are kept.`))
                      removeFromShelf(book.id).catch(() => undefined)
                  }}
                >
                  <Trash2 className="size-4" />
                  <span className="sr-only">Remove from shelf</span>
                </Button>
              </div>
            </div>
          </article>
        )
      })}
      {visible.length === 0 ? (
        <p className="rounded-2xl border border-dashed p-8 text-center text-sm text-muted-foreground">
          {shelf.length === 0 ? 'Your shelf is empty. Add the books you own or want to read.' : 'No books here.'}
        </p>
      ) : null}
    </section>
  )
}

function NotesSection({ onNote }: { onNote: (options: { note?: Note }) => void }) {
  const { notes, shelf } = useLibrary()
  const [tag, setTag] = useState('all')
  const counts = new Map<string, number>()
  for (const note of notes) for (const t of note.tags) counts.set(t, (counts.get(t) ?? 0) + 1)
  const tags = [...counts.keys()].sort()
  const activeTag = tag === 'all' || counts.has(tag) ? tag : 'all'
  const visible = activeTag === 'all' ? notes : notes.filter((note) => note.tags.includes(activeTag))
  const titles = new Map(shelf.map((book) => [book.id, book.title]))

  return (
    <section aria-label="Your notes" className="flex flex-col gap-4">
      {tags.length > 0 ? (
        <FilterChips
          label="Filter notes by tag"
          value={activeTag}
          onChange={setTag}
          options={[
            { value: 'all', label: 'All notes' },
            ...tags.map((t) => ({ value: t, label: `#${t}`, count: counts.get(t) })),
          ]}
        />
      ) : null}
      <Button variant="outline" className="h-11 gap-1.5 rounded-full" onClick={() => onNote({})}>
        <NotebookPen className="size-4" />
        New note
      </Button>
      {visible.map((note) => (
        <button
          key={note.id}
          type="button"
          onClick={() => onNote({ note })}
          className="flex flex-col gap-2 rounded-2xl border bg-card p-4 text-left outline-none hover:bg-muted/50 focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {note.bookId && titles.has(note.bookId) ? (
            <span className="font-serif text-sm font-semibold text-primary italic">{titles.get(note.bookId)}</span>
          ) : null}
          <span className="line-clamp-6 text-[0.95rem] leading-relaxed whitespace-pre-wrap">{note.text}</span>
          <span className="flex flex-wrap items-center gap-1.5">
            {note.tags.map((t) => (
              <span
                key={t}
                className="rounded-full bg-accent/15 px-2 py-0.5 text-xs font-medium text-accent-foreground"
              >
                #{t}
              </span>
            ))}
            <span className="ml-auto text-xs text-muted-foreground">{relativeTime(note.updatedAt)}</span>
          </span>
        </button>
      ))}
      {notes.length === 0 ? (
        <p className="rounded-2xl border border-dashed p-8 text-center text-sm text-muted-foreground">
          Quotes, ideas, questions for book club. Tag them to find them again.
        </p>
      ) : null}
    </section>
  )
}

function EmptyState({
  text,
  action,
  icon: Icon,
  onClick,
}: {
  text: string
  action: string
  icon: typeof PenLine
  onClick: () => void
}) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed p-8 text-center">
      <p className="text-sm text-pretty text-muted-foreground">{text}</p>
      <Button className="h-11 gap-1.5 rounded-full px-5" onClick={onClick}>
        <Icon className="size-4" />
        {action}
      </Button>
    </div>
  )
}

function InstallApp() {
  const { mode, install } = useInstall()
  if (mode === 'prompt')
    return (
      <Button className="h-11 gap-1.5 rounded-full px-5" onClick={() => install().catch(() => undefined)}>
        <Download className="size-4" />
        Install Dog-ear
      </Button>
    )
  if (mode === 'ios')
    return (
      <p className="max-w-xs text-sm text-pretty text-muted-foreground">
        Add Dog-ear to your home screen: tap <Share aria-label="Share" className="inline size-4 align-[-3px]" /> then
        “Add to Home Screen”.
      </p>
    )
  return null
}
