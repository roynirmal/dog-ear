'use client'

import { useState } from 'react'
import { PenLine, UserPlus } from 'lucide-react'
import { BookCover } from '@/components/book-cover'
import { FilterChips } from '@/components/filter-chips'
import { useLibrary } from '@/components/library-provider'
import { PostCard } from '@/components/post-card'
import { Button } from '@/components/ui/button'
import { firstName } from '@/lib/data'

const filters = [
  { value: 'all', label: 'Everything' },
  { value: 'reading', label: 'Reading now' },
  { value: 'finished', label: 'Finished' },
] as const

type Filter = (typeof filters)[number]['value']

export function FeedView({ onShare, onInvite }: { onShare: () => void; onInvite: () => void }) {
  const { posts, me, friends, person } = useLibrary()
  const [filter, setFilter] = useState<Filter>('all')

  const currentlyReading = posts.filter(
    (post, index, all) =>
      post.status === 'reading' &&
      post.authorId !== me.id &&
      all.findIndex((p) => p.authorId === post.authorId && p.status === 'reading') === index,
  )
  const visible = filter === 'all' ? posts : posts.filter((post) => post.status === filter)

  return (
    <div className="flex flex-col gap-6">
      {currentlyReading.length > 0 ? (
        <section aria-labelledby="on-nightstands">
          <h2 id="on-nightstands" className="px-4 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
            On friends&apos; nightstands
          </h2>
          <ul className="mt-3 flex gap-4 overflow-x-auto px-4 pb-1 [scrollbar-width:none]">
            {currentlyReading.map((post) => (
              <li key={post.id} className="flex w-20 shrink-0 flex-col items-center gap-2 text-center">
                <BookCover book={post.book} className="h-28 w-20" />
                <span className="w-full truncate text-xs font-medium">{firstName(person(post.authorId).name)}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <section aria-labelledby="feed-heading" className="flex flex-col gap-4 px-4">
        <div className="flex flex-col gap-3">
          <h2 id="feed-heading" className="sr-only">
            Reading feed
          </h2>
          <FilterChips options={filters} value={filter} onChange={setFilter} label="Filter feed" />
        </div>
        {visible.map((post) => (
          <PostCard key={post.id} post={post} />
        ))}
        {visible.length === 0 && friends.length === 0 && filter === 'all' ? (
          <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed p-8 text-center">
            <p className="text-sm text-pretty text-muted-foreground">
              Dog-ear is better with friends. Invite a few and their reads will show up here.
            </p>
            <div className="flex flex-wrap justify-center gap-2">
              <Button className="h-11 gap-1.5 rounded-full px-5" onClick={onInvite}>
                <UserPlus className="size-4" />
                Invite friends
              </Button>
              <Button variant="outline" className="h-11 gap-1.5 rounded-full px-5" onClick={onShare}>
                <PenLine className="size-4" />
                Share a book
              </Button>
            </div>
          </div>
        ) : visible.length === 0 ? (
          <p className="rounded-2xl border border-dashed p-8 text-center text-sm text-muted-foreground">
            Nothing here yet.
          </p>
        ) : null}
      </section>
    </div>
  )
}
