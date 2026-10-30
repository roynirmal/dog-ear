'use client'

import { useState } from 'react'
import { Bell } from 'lucide-react'
import { BookCover } from '@/components/book-cover'
import { FriendAvatar } from '@/components/friend-avatar'
import { useLibrary } from '@/components/library-provider'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { firstName, relativeTime } from '@/lib/data'
import { cn } from '@/lib/utils'

/** Bell with the "friend started reading" notifications. */
export function NotificationsButton({ onOpenPost }: { onOpenPost: (postId: string) => void }) {
  const { notifications, person, markNotificationsRead } = useLibrary()
  const [open, setOpen] = useState(false)
  // Keep what was unread when the panel opened highlighted while it stays open.
  const [freshIds, setFreshIds] = useState<string[]>([])
  const unread = notifications.filter((n) => !n.read)

  function show() {
    setFreshIds(unread.map((n) => n.id))
    setOpen(true)
    if (unread.length > 0) markNotificationsRead().catch(() => undefined)
  }

  return (
    <>
      <button
        type="button"
        onClick={show}
        className="relative flex size-11 items-center justify-center rounded-full text-foreground outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <Bell aria-hidden="true" className="size-6" />
        {unread.length > 0 ? (
          <span className="absolute top-1.5 right-1.5 flex min-w-4 items-center justify-center rounded-full bg-accent px-1 text-[0.6rem] font-bold text-white">
            {unread.length > 9 ? '9+' : unread.length}
          </span>
        ) : null}
        <span className="sr-only">Notifications{unread.length > 0 ? `, ${unread.length} new` : ''}</span>
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[90dvh] overflow-y-auto rounded-2xl p-5 sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="font-serif text-xl">Notifications</DialogTitle>
            <DialogDescription>When a friend starts reading something new.</DialogDescription>
          </DialogHeader>
          {notifications.length > 0 ? (
            <ul className="flex min-w-0 flex-col gap-2">
              {notifications.map((n) => (
                <li key={n.id}>
                  <button
                    type="button"
                    onClick={() => {
                      setOpen(false)
                      onOpenPost(n.postId)
                    }}
                    className={cn(
                      'flex w-full items-center gap-3 rounded-2xl border p-3 text-left outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50',
                      freshIds.includes(n.id) ? 'border-accent/40 bg-accent/5' : 'bg-card',
                    )}
                  >
                    <FriendAvatar friendId={n.actorId} size="sm" />
                    <span className="min-w-0 flex-1 text-sm">
                      <span className="font-semibold">{firstName(person(n.actorId).name)}</span> started reading{' '}
                      <span className="font-serif font-semibold italic">{n.book.title}</span>
                      <span className="block text-xs text-muted-foreground">{relativeTime(n.createdAt)}</span>
                    </span>
                    <BookCover book={n.book} size="sm" />
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="rounded-2xl border border-dashed p-6 text-center text-sm text-muted-foreground">
              Nothing yet. You’ll hear when a friend starts a new book.
            </p>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
