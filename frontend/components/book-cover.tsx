'use client'

import { useState } from 'react'
import { cn } from '@/lib/utils'
import { coverUrl, toneFor, type Book, type CoverTone } from '@/lib/data'

const toneClasses: Record<CoverTone, string> = {
  1: 'bg-chart-1 text-primary-foreground',
  2: 'bg-chart-2 text-primary-foreground',
  3: 'bg-chart-3 text-primary-foreground',
  4: 'bg-chart-4 text-foreground',
  5: 'bg-chart-5 text-primary-foreground',
}

const sizeClasses = {
  sm: 'h-16 w-11 p-1.5 text-[0.5rem]',
  md: 'h-24 w-16 p-2 text-[0.6rem]',
}

export function BookCover({
  book,
  size = 'md',
  className,
}: {
  book: Book
  size?: keyof typeof sizeClasses
  className?: string
}) {
  const [failed, setFailed] = useState(false)
  const image = failed ? undefined : coverUrl(book, size === 'sm' ? 'S' : 'M')
  const frame = 'relative flex shrink-0 overflow-hidden rounded-r-md rounded-l-sm shadow-[2px_3px_0_0_var(--border)]'

  if (image) {
    return (
      <div aria-hidden="true" className={cn(frame, 'bg-muted', sizeClasses[size], 'p-0', className)}>
        {/* Open Library covers are already small; next/image adds nothing in a static export. */}
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src={image} alt="" loading="lazy" className="size-full object-cover" onError={() => setFailed(true)} />
        <span className="absolute inset-y-0 left-1 w-px bg-white opacity-30" />
      </div>
    )
  }

  return (
    <div
      aria-hidden="true"
      className={cn(frame, 'flex-col justify-between', toneClasses[toneFor(book.title)], sizeClasses[size], className)}
    >
      <span className="absolute inset-y-0 left-1 w-px bg-current opacity-20" />
      {size === 'sm' ? (
        <span className="flex flex-1 items-center justify-center pl-1 font-serif text-2xl font-semibold">
          {book.title.replace(/^(The|A|An) /, '').charAt(0)}
        </span>
      ) : (
        <>
          <span className="line-clamp-4 pl-1.5 font-serif leading-tight font-semibold break-words text-balance">
            {book.title}
          </span>
          <span className="truncate pl-1.5 text-[0.85em] opacity-75">{book.author.split(' ').at(-1)}</span>
        </>
      )}
    </div>
  )
}
