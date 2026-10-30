'use client'

import { useCallback, useEffect, useState } from 'react'
import { ArrowLeftRight, BookOpen, BookPlus, NotebookPen, PenLine, Plus, User, Users } from 'lucide-react'
import { AddBookDialog } from '@/components/add-book-dialog'
import { FeedView } from '@/components/feed-view'
import { FriendsView } from '@/components/friends-view'
import { LibraryProvider, useLibrary } from '@/components/library-provider'
import { LoanDialog } from '@/components/loan-dialog'
import { LoansView } from '@/components/loans-view'
import { NoteDialog } from '@/components/note-dialog'
import { NotificationsButton } from '@/components/notifications'
import { PersonCardProvider } from '@/components/person-card'
import { ProfileView } from '@/components/profile-view'
import { ShareDialog } from '@/components/share-dialog'
import { SignIn } from '@/components/sign-in'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { api, pb } from '@/lib/api'
import { isOverdue, type Book, type LibraryState, type Note } from '@/lib/data'
import { cn } from '@/lib/utils'

type Tab = 'feed' | 'loans' | 'friends' | 'you'

const tabMeta: Record<Tab, { label: string; title: string; icon: typeof BookOpen }> = {
  feed: { label: 'Feed', title: 'What friends are reading', icon: BookOpen },
  loans: { label: 'Loans', title: 'Lent & borrowed', icon: ArrowLeftRight },
  friends: { label: 'Friends', title: 'Your friends', icon: Users },
  you: { label: 'You', title: 'Your shelf', icon: User },
}

const INVITE_KEY = 'dogear.invite'

function storage() {
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}

export function AppShell() {
  const [authed, setAuthed] = useState<boolean | null>(null)
  const [initial, setInitial] = useState<LibraryState | null>(null)
  const [loadError, setLoadError] = useState('')
  const [attempt, setAttempt] = useState(0)
  const [notice, setNotice] = useState<{ text: string; error?: boolean } | null>(null)

  const showError = useCallback((text: string) => setNotice({ text, error: true }), [])

  useEffect(() => {
    // Keep an invite code across the Google sign-in, then tidy the address bar.
    const url = new URL(window.location.href)
    const invite = url.searchParams.get('invite')
    if (invite) {
      storage()?.setItem(INVITE_KEY, invite)
      url.searchParams.delete('invite')
      window.history.replaceState(null, '', url.pathname + url.search + url.hash)
    }
    setAuthed(pb.authStore.isValid)
    return pb.authStore.onChange(() => setAuthed(pb.authStore.isValid))
  }, [])

  useEffect(() => {
    if (!authed) {
      setInitial(null)
      return
    }
    let cancelled = false
    setLoadError('')
    ;(async () => {
      try {
        let state = await api('state')
        const invite = storage()?.getItem(INVITE_KEY)
        if (invite) {
          storage()?.removeItem(INVITE_KEY)
          const before = new Set(state.friends)
          try {
            state = await api('befriend', { code: invite })
            const added = state.friends.find((id) => !before.has(id))
            const name = state.people.find((p) => p.id === added)?.name
            if (added) setNotice({ text: `You and ${name ?? 'your friend'} are now reading together.` })
          } catch (error) {
            setNotice({ text: error instanceof Error ? error.message : String(error), error: true })
          }
        }
        if (!cancelled) setInitial(state)
      } catch (error) {
        if (!cancelled && pb.authStore.isValid) setLoadError(error instanceof Error ? error.message : String(error))
      }
    })()
    return () => {
      cancelled = true
    }
  }, [authed, attempt])

  useEffect(() => {
    if (!notice) return
    const timer = window.setTimeout(() => setNotice(null), notice.error ? 6000 : 4000)
    return () => window.clearTimeout(timer)
  }, [notice])

  let content: React.ReactNode
  if (authed === null) content = null
  else if (!authed) content = <SignIn hasInvite={!!storage()?.getItem(INVITE_KEY)} />
  else if (loadError)
    content = (
      <div className="mx-auto flex min-h-dvh max-w-lg flex-col items-center justify-center gap-4 px-6 text-center">
        <p className="text-sm text-muted-foreground">{loadError}</p>
        <Button className="h-11 rounded-full px-5" onClick={() => setAttempt((n) => n + 1)}>
          Try again
        </Button>
      </div>
    )
  else if (!initial)
    content = (
      <div className="flex min-h-dvh items-center justify-center">
        <p className="font-serif text-lg text-primary italic">Opening your shelf…</p>
      </div>
    )
  else
    content = (
      <LibraryProvider initial={initial} onError={showError}>
        <PersonCardProvider>
          <Shell />
        </PersonCardProvider>
      </LibraryProvider>
    )

  return (
    <>
      {content}
      {notice ? (
        <div className="pointer-events-none fixed inset-x-0 top-[max(env(safe-area-inset-top),0.75rem)] z-[60] flex justify-center px-4">
          <p
            role={notice.error ? 'alert' : 'status'}
            className={cn(
              'pointer-events-auto max-w-md rounded-2xl px-4 py-3 text-sm shadow-lg',
              notice.error ? 'bg-destructive text-white' : 'bg-primary text-primary-foreground',
            )}
            onClick={() => setNotice(null)}
          >
            {notice.text}
          </p>
        </div>
      ) : null}
    </>
  )
}

function Shell() {
  const { loans, requests, friendRequests } = useLibrary()
  const [tab, setTab] = useState<Tab>('feed')
  const [chooserOpen, setChooserOpen] = useState(false)
  const [share, setShare] = useState<{ open: boolean; book?: Book }>({ open: false })
  const [loan, setLoan] = useState<{ open: boolean; book?: Book }>({ open: false })
  const [addBookOpen, setAddBookOpen] = useState(false)
  const [note, setNote] = useState<{ open: boolean; note?: Note; bookId?: string }>({ open: false })

  const badges: Partial<Record<Tab, number>> = {
    loans: loans.filter(isOverdue).length + requests.length,
    friends: friendRequests.length,
  }

  function goTo(next: Tab) {
    setTab(next)
    window.scrollTo({ top: 0 })
  }

  function openPost(postId: string) {
    setTab('feed')
    // Wait for the feed to render before scrolling to the post.
    window.setTimeout(() => {
      const post = document.getElementById(`post-${postId}`)
      if (post) post.scrollIntoView({ behavior: 'smooth', block: 'center' })
      else window.scrollTo({ top: 0 })
    }, 50)
  }

  const leftTabs: Tab[] = ['feed', 'loans']
  const rightTabs: Tab[] = ['friends', 'you']

  function renderTab(id: Tab) {
    const { label, icon: Icon } = tabMeta[id]
    const active = tab === id
    return (
      <button
        key={id}
        type="button"
        onClick={() => goTo(id)}
        aria-current={active ? 'page' : undefined}
        className={cn(
          'relative flex min-h-14 flex-1 flex-col items-center justify-center gap-1 text-xs font-medium outline-none focus-visible:ring-3 focus-visible:ring-ring/50 rounded-xl',
          active ? 'text-primary' : 'text-muted-foreground',
        )}
      >
        <span className="relative">
          <Icon aria-hidden="true" className={cn('size-6', active && 'stroke-[2.25]')} />
          {badges[id] ? (
            <span className="absolute -top-1 -right-2 flex size-4 items-center justify-center rounded-full bg-destructive text-[0.6rem] font-bold text-white">
              {badges[id]}
              <span className="sr-only"> need attention</span>
            </span>
          ) : null}
        </span>
        {label}
      </button>
    )
  }

  const choices = [
    {
      icon: PenLine,
      title: "Share what I'm reading",
      description: 'Post progress, a rating, and your thoughts.',
      onClick: () => setShare({ open: true }),
    },
    {
      icon: ArrowLeftRight,
      title: 'Log a loan',
      description: 'Record a book you lent or borrowed.',
      onClick: () => setLoan({ open: true }),
    },
    {
      icon: BookPlus,
      title: 'Add to my shelf',
      description: 'Find a book and keep it in your collection.',
      onClick: () => setAddBookOpen(true),
    },
    {
      icon: NotebookPen,
      title: 'Write a note',
      description: 'Jot down a quote or idea and tag it.',
      onClick: () => setNote({ open: true }),
    },
  ]

  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-lg flex-col">
      <header className="sticky top-0 z-30 border-b bg-background/90 px-4 pt-[max(env(safe-area-inset-top),0.75rem)] pb-3 backdrop-blur">
        <div className="flex items-end justify-between gap-3">
          <div className="min-w-0">
            <p className="font-serif text-sm font-semibold tracking-tight text-primary italic">Dog-ear</p>
            <h1 className="font-serif text-2xl leading-tight font-semibold text-balance">{tabMeta[tab].title}</h1>
          </div>
          <NotificationsButton onOpenPost={openPost} />
        </div>
      </header>

      <main className="flex-1 pt-5 pb-32">
        {tab === 'feed' ? <FeedView onShare={() => setShare({ open: true })} onInvite={() => goTo('friends')} /> : null}
        {tab === 'loans' ? <LoansView onLogLoan={() => setLoan({ open: true })} /> : null}
        {tab === 'friends' ? <FriendsView /> : null}
        {tab === 'you' ? (
          <ProfileView
            onShare={(book) => setShare({ open: true, book })}
            onLend={(book) => setLoan({ open: true, book })}
            onAddBook={() => setAddBookOpen(true)}
            onNote={(options) => setNote({ open: true, ...options })}
          />
        ) : null}
      </main>

      <nav
        aria-label="Main"
        className="fixed inset-x-0 bottom-0 z-30 border-t bg-background/95 pb-[env(safe-area-inset-bottom)] backdrop-blur"
      >
        <div className="mx-auto flex max-w-lg items-center gap-1 px-2 py-1">
          {leftTabs.map(renderTab)}
          <div className="flex flex-1 justify-center">
            <button
              type="button"
              onClick={() => setChooserOpen(true)}
              className="-mt-6 flex size-14 items-center justify-center rounded-full bg-accent text-white shadow-lg ring-4 ring-background outline-none focus-visible:ring-ring"
            >
              <Plus aria-hidden="true" className="size-7" />
              <span className="sr-only">Add</span>
            </button>
          </div>
          {rightTabs.map(renderTab)}
        </div>
      </nav>

      <Dialog open={chooserOpen} onOpenChange={setChooserOpen}>
        <DialogContent className="rounded-2xl p-5 sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="font-serif text-xl">Add to Dog-ear</DialogTitle>
            <DialogDescription>Share a read or keep tabs on a book.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            {choices.map((choice) => (
              <ChooserOption
                key={choice.title}
                {...choice}
                onClick={() => {
                  setChooserOpen(false)
                  choice.onClick()
                }}
              />
            ))}
          </div>
        </DialogContent>
      </Dialog>

      <ShareDialog
        open={share.open}
        initialBook={share.book}
        onOpenChange={(open) => setShare((current) => ({ ...current, open }))}
        onShared={() => goTo('feed')}
      />
      <LoanDialog
        open={loan.open}
        initialBook={loan.book}
        onOpenChange={(open) => setLoan((current) => ({ ...current, open }))}
        onLogged={() => goTo('loans')}
      />
      <AddBookDialog open={addBookOpen} onOpenChange={setAddBookOpen} onAdded={() => goTo('you')} />
      <NoteDialog
        open={note.open}
        note={note.note}
        bookId={note.bookId}
        onOpenChange={(open) => setNote((current) => ({ ...current, open }))}
        onSaved={() => goTo('you')}
      />
    </div>
  )
}

function ChooserOption({
  icon: Icon,
  title,
  description,
  onClick,
}: {
  icon: typeof BookOpen
  title: string
  description: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-4 rounded-2xl border bg-card p-4 text-left transition-colors outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <span className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
        <Icon aria-hidden="true" className="size-6" />
      </span>
      <span className="flex flex-col">
        <span className="font-semibold">{title}</span>
        <span className="text-sm text-muted-foreground">{description}</span>
      </span>
    </button>
  )
}
