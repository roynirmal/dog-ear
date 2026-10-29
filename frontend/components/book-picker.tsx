'use client'

import { useEffect, useState } from 'react'
import { Loader2, Search, X } from 'lucide-react'
import { BookCover } from '@/components/book-cover'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { searchBooks } from '@/lib/api'
import type { Book } from '@/lib/data'

/**
 * Pick a book from your shelf or Open Library, or type one in by hand.
 * `value` is the chosen book; null while the reader is still searching.
 */
export function BookPicker({
  id,
  value,
  onChange,
}: {
  id: string
  value: Book | null
  onChange: (book: Book | null) => void
}) {
  const { shelf } = useLibrary()
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<Book[]>([])
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState('')
  const [manual, setManual] = useState(false)
  const [title, setTitle] = useState('')
  const [author, setAuthor] = useState('')

  const trimmed = query.trim()
  const shelfMatches = trimmed
    ? shelf.filter((b) => `${b.title} ${b.author}`.toLowerCase().includes(trimmed.toLowerCase())).slice(0, 3)
    : []

  useEffect(() => {
    if (trimmed.length < 2 || manual || value) {
      setResults([])
      setSearching(false)
      return
    }
    const controller = new AbortController()
    setSearching(true)
    setError('')
    const timer = window.setTimeout(() => {
      searchBooks(trimmed, controller.signal)
        .then(setResults)
        .catch((err) => {
          if (!controller.signal.aborted) setError(err instanceof Error ? err.message : String(err))
        })
        .finally(() => {
          if (!controller.signal.aborted) setSearching(false)
        })
    }, 350)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [trimmed, manual, value])

  useEffect(() => {
    if (manual) onChange(title.trim() ? { title: title.trim(), author: author.trim() } : null)
    // onChange is a fresh closure on every parent render; only typed values matter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [manual, title, author])

  function choose(book: Book) {
    const { title, author, olKey, coverId, year } = book
    onChange({ title, author, olKey, coverId, year })
    setQuery('')
    setResults([])
  }

  if (value && !manual) {
    return (
      <div className="flex flex-col gap-2">
        <Label>Book</Label>
        <div className="flex items-center gap-3 rounded-2xl border bg-card p-3">
          <BookCover book={value} size="sm" />
          <div className="min-w-0 flex-1">
            <p className="font-serif leading-snug font-semibold">{value.title}</p>
            <p className="truncate text-sm text-muted-foreground">
              {[value.author, value.year].filter(Boolean).join(' · ')}
            </p>
          </div>
          <Button type="button" variant="ghost" size="icon" className="size-11" onClick={() => onChange(null)}>
            <X className="size-4" />
            <span className="sr-only">Choose a different book</span>
          </Button>
        </div>
      </div>
    )
  }

  if (manual) {
    return (
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor={`${id}-title`}>Title</Label>
          <Input
            id={`${id}-title`}
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="e.g. Piranesi"
            className="h-11 text-base"
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor={`${id}-author`}>Author</Label>
          <Input
            id={`${id}-author`}
            value={author}
            onChange={(e) => setAuthor(e.target.value)}
            placeholder="e.g. Susanna Clarke"
            className="h-11 text-base"
          />
        </div>
        <button
          type="button"
          className="self-start text-sm font-medium text-primary underline-offset-4 hover:underline"
          onClick={() => {
            setManual(false)
            onChange(null)
          }}
        >
          Search instead
        </button>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={`${id}-search`}>Book</Label>
      <div className="relative">
        <Search aria-hidden="true" className="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          id={`${id}-search`}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search by title, author or ISBN"
          className="h-11 pl-9 text-base"
          autoComplete="off"
        />
        {searching ? (
          <Loader2
            aria-label="Searching"
            className="absolute top-1/2 right-3 size-4 -translate-y-1/2 animate-spin text-muted-foreground"
          />
        ) : null}
      </div>
      {shelfMatches.length + results.length > 0 ? (
        <ul className="flex max-h-72 flex-col divide-y overflow-y-auto rounded-2xl border bg-card">
          {shelfMatches.map((book) => (
            <ResultRow key={`shelf-${book.id}`} book={book} badge="On your shelf" onPick={() => choose(book)} />
          ))}
          {results.map((book, i) => (
            <ResultRow key={`${book.olKey ?? book.title}-${i}`} book={book} onPick={() => choose(book)} />
          ))}
        </ul>
      ) : null}
      {error ? <p className="text-sm text-destructive">{error}</p> : null}
      {trimmed.length >= 2 && !searching && !error && results.length === 0 && shelfMatches.length === 0 ? (
        <p className="text-sm text-muted-foreground">No matches on Open Library.</p>
      ) : null}
      <button
        type="button"
        className="self-start text-sm font-medium text-primary underline-offset-4 hover:underline"
        onClick={() => {
          setManual(true)
          setTitle(trimmed)
        }}
      >
        Can’t find it? Enter it yourself
      </button>
      <p className="text-xs text-muted-foreground">Search powered by Open Library.</p>
    </div>
  )
}

function ResultRow({ book, badge, onPick }: { book: Book; badge?: string; onPick: () => void }) {
  return (
    <li>
      <button
        type="button"
        onClick={onPick}
        className="flex w-full items-center gap-3 p-3 text-left outline-none hover:bg-muted focus-visible:bg-muted"
      >
        <BookCover book={book} size="sm" />
        <span className="min-w-0 flex-1">
          <span className="block font-serif leading-snug font-semibold">{book.title}</span>
          <span className="block truncate text-sm text-muted-foreground">
            {[book.author, book.year].filter(Boolean).join(' · ')}
          </span>
          {badge ? (
            <span className="mt-1 inline-block rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">
              {badge}
            </span>
          ) : null}
        </span>
      </button>
    </li>
  )
}
