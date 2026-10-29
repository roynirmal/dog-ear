'use client'

import { useState } from 'react'
import { BookOpen, Check, Copy, Share2, UserMinus } from 'lucide-react'
import { FriendAvatar } from '@/components/friend-avatar'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'

export function CircleView() {
  const { posts, loans, friends, person, code, unfriend } = useLibrary()
  const [copied, setCopied] = useState(false)
  const circle = friends.map(person).sort((a, b) => a.name.localeCompare(b.name))
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
      await navigator.share({ title: 'Read with me on Dog-ear', text: 'Join my reading circle on Dog-ear', url: link })
    } catch {
      // Dismissed share sheets reject; nothing to do.
    }
  }

  return (
    <div className="flex flex-col gap-4 px-4">
      <section
        aria-labelledby="invite-heading"
        className="flex flex-col gap-3 rounded-2xl bg-primary p-4 text-primary-foreground"
      >
        <div>
          <h2 id="invite-heading" className="font-serif text-lg font-semibold">
            Invite a friend
          </h2>
          <p className="text-sm opacity-80">
            Anyone who opens your link and signs in joins your circle. Only share it with people you know.
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

      <p className="text-sm text-muted-foreground">
        {circle.length === 1 ? '1 friend' : `${circle.length} friends`} in your reading circle
      </p>
      {circle.length > 0 ? (
        <ul className="flex flex-col divide-y rounded-2xl border bg-card">
          {circle.map((friend) => {
            const current = posts.find((post) => post.authorId === friend.id && post.status === 'reading')
            const latest = current ?? posts.find((post) => post.authorId === friend.id)
            const active = loans.filter((loan) => loan.friendId === friend.id && !loan.returnedAt)
            const theyHave = active.filter((loan) => loan.direction === 'lent').length
            const youHave = active.filter((loan) => loan.direction === 'borrowed').length

            return (
              <li key={friend.id} className="flex items-start gap-3 p-4">
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
                  {theyHave + youHave > 0 ? (
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
                    </div>
                  ) : null}
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-11 text-muted-foreground"
                  onClick={() => {
                    if (window.confirm(`Remove ${friend.name} from your circle? Loans between you are kept.`))
                      unfriend(friend.id).catch(() => undefined)
                  }}
                >
                  <UserMinus className="size-4" />
                  <span className="sr-only">Remove {friend.name}</span>
                </Button>
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
