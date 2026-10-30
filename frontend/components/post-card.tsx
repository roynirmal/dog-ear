'use client'

import { useState } from 'react'
import { BookMarked, Check, Heart, MessageCircle, Send, Star, Trash2 } from 'lucide-react'
import { BookCover } from '@/components/book-cover'
import { FriendAvatar } from '@/components/friend-avatar'
import { useLibrary } from '@/components/library-provider'
import { PersonButton } from '@/components/person-card'
import { Button } from '@/components/ui/button'
import { relativeTime, type Post } from '@/lib/data'
import { cn } from '@/lib/utils'

export function PostCard({ post }: { post: Post }) {
  const { me, person, toggleLike, addComment, requestBorrow, deletePost } = useLibrary()
  const [showComments, setShowComments] = useState(post.comments.length > 0)
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const author = person(post.authorId)
  const liked = post.likedBy.includes(me.id)
  const isMine = post.authorId === me.id

  async function submitComment(event: React.FormEvent) {
    event.preventDefault()
    const text = draft.trim()
    if (!text || sending) return
    setSending(true)
    try {
      await addComment(post.id, text)
      setDraft('')
    } catch {
      // The provider shows the error; keep the draft.
    } finally {
      setSending(false)
    }
  }

  return (
    <article id={`post-${post.id}`} className="scroll-mt-28 rounded-2xl border bg-card p-4">
      <header className="flex items-center gap-3">
        <PersonButton personId={post.authorId} className="shrink-0 rounded-full">
          <FriendAvatar friendId={post.authorId} />
          <span className="sr-only">View {author.name}</span>
        </PersonButton>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm">
            <PersonButton personId={post.authorId} className="inline font-semibold">
              {isMine ? 'You' : author.name}
            </PersonButton>{' '}
            <span className="text-muted-foreground">
              {post.status === 'reading' ? (isMine ? 'are reading' : 'is reading') : 'finished'}
            </span>
          </p>
          <p className="text-xs text-muted-foreground">
            <time dateTime={post.createdAt} suppressHydrationWarning>
              {relativeTime(post.createdAt)}
            </time>
          </p>
        </div>
      </header>

      <div className="mt-4 flex gap-4">
        <BookCover book={post.book} />
        <div className="flex min-w-0 flex-1 flex-col justify-center gap-1">
          <h3 className="font-serif text-lg leading-snug font-semibold text-balance">{post.book.title}</h3>
          <p className="text-sm text-muted-foreground">{post.book.author}</p>
          {post.status === 'reading' && post.progress !== undefined ? (
            <div className="mt-2 flex items-center gap-2">
              <div
                role="progressbar"
                aria-valuenow={post.progress}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-label="Reading progress"
                className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted"
              >
                <div className="h-full rounded-full bg-primary" style={{ width: `${post.progress}%` }} />
              </div>
              <span className="text-xs font-medium text-muted-foreground tabular-nums">{post.progress}%</span>
            </div>
          ) : null}
          {post.status === 'finished' && post.rating ? (
            <div className="mt-2 flex items-center gap-0.5" aria-label={`Rated ${post.rating} out of 5`}>
              {Array.from({ length: 5 }, (_, i) => (
                <Star
                  key={i}
                  aria-hidden="true"
                  className={cn('size-4', i < (post.rating ?? 0) ? 'fill-accent text-accent' : 'text-border')}
                />
              ))}
            </div>
          ) : null}
        </div>
      </div>

      {post.thoughts ? <p className="mt-4 text-[0.95rem] leading-relaxed text-pretty">{post.thoughts}</p> : null}

      <div className="mt-3 flex items-center gap-1 border-t pt-2 -mx-1">
        <Button
          variant="ghost"
          className="h-11 gap-1.5 px-3"
          aria-pressed={liked}
          onClick={() => toggleLike(post.id).catch(() => undefined)}
        >
          <Heart className={cn('size-5', liked && 'fill-accent text-accent')} />
          <span className="tabular-nums">{post.likedBy.length}</span>
          <span className="sr-only">{liked ? 'Unlike' : 'Like'}</span>
        </Button>
        <Button
          variant="ghost"
          className="h-11 gap-1.5 px-3"
          aria-expanded={showComments}
          onClick={() => setShowComments((open) => !open)}
        >
          <MessageCircle className="size-5" />
          <span className="tabular-nums">{post.comments.length}</span>
          <span className="sr-only">comments</span>
        </Button>
        {post.canLend && post.shelfBookId && !isMine ? (
          <Button
            variant={post.requested ? 'secondary' : 'outline'}
            className="ml-auto h-11 gap-1.5 rounded-full px-4"
            disabled={post.requested}
            onClick={() => requestBorrow(post.shelfBookId!).catch(() => undefined)}
          >
            {post.requested ? <Check className="size-4" /> : <BookMarked className="size-4" />}
            {post.requested ? 'Requested' : 'Ask to borrow'}
          </Button>
        ) : post.lentOut && !isMine ? (
          <span className="ml-auto rounded-full bg-muted px-3 py-1 text-xs font-medium text-muted-foreground">
            Lent out
          </span>
        ) : null}
        {isMine ? (
          <Button
            variant="ghost"
            className="ml-auto h-11 px-3 text-muted-foreground"
            onClick={() => {
              if (window.confirm('Delete this post and its comments?')) deletePost(post.id).catch(() => undefined)
            }}
          >
            <Trash2 className="size-4" />
            <span className="sr-only">Delete post</span>
          </Button>
        ) : null}
      </div>

      {showComments ? (
        <div className="mt-2 flex flex-col gap-3">
          {post.comments.map((comment) => (
            <div key={comment.id} className="flex gap-2.5">
              <PersonButton personId={comment.authorId} className="h-fit shrink-0 rounded-full">
                <FriendAvatar friendId={comment.authorId} size="sm" />
                <span className="sr-only">View {person(comment.authorId).name}</span>
              </PersonButton>
              <div className="min-w-0 flex-1 rounded-2xl rounded-tl-sm bg-muted px-3 py-2">
                <PersonButton personId={comment.authorId} className="text-xs font-semibold">
                  {person(comment.authorId).name}
                </PersonButton>
                <p className="text-sm leading-relaxed">{comment.text}</p>
              </div>
            </div>
          ))}
          <form onSubmit={submitComment} className="flex items-center gap-2">
            <label htmlFor={`comment-${post.id}`} className="sr-only">
              Add a comment
            </label>
            <input
              id={`comment-${post.id}`}
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder="Add a thought…"
              maxLength={1000}
              className="h-11 min-w-0 flex-1 rounded-full border bg-background px-4 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            />
            <Button type="submit" size="icon" className="size-11 rounded-full" disabled={!draft.trim() || sending}>
              <Send className="size-4" />
              <span className="sr-only">Post comment</span>
            </Button>
          </form>
        </div>
      ) : null}
    </article>
  )
}
