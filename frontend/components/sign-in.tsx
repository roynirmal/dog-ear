'use client'

import { useEffect, useState } from 'react'
import { ArrowLeftRight, Heart, NotebookPen } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { fetchConfig, signInWithGoogle } from '@/lib/api'

const highlights = [
  { icon: Heart, text: 'See what friends are reading and heart their thoughts' },
  { icon: ArrowLeftRight, text: 'Remember who has which book, and when it’s due back' },
  { icon: NotebookPen, text: 'Keep your shelf and tagged notes in one place' },
]

export function SignIn({ hasInvite }: { hasInvite: boolean }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [configured, setConfigured] = useState(true)

  useEffect(() => {
    fetchConfig()
      .then((config) => setConfigured(config.google))
      .catch(() => setError('Dog-ear’s server is not reachable right now.'))
  }, [])

  async function signIn() {
    setBusy(true)
    setError('')
    try {
      await signInWithGoogle()
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : 'Sign-in did not finish. Please try again.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-lg flex-col justify-center gap-8 px-6 py-12">
      <div className="flex flex-col gap-3">
        <p className="font-serif text-lg font-semibold text-primary italic">Dog-ear</p>
        <h1 className="font-serif text-4xl leading-tight font-semibold text-balance">Read together, lend freely.</h1>
        {hasInvite ? (
          <p className="rounded-2xl bg-primary/10 px-4 py-3 text-sm text-primary">
            A friend invited you to their reading circle. Sign in to join them.
          </p>
        ) : null}
      </div>
      <ul className="flex flex-col gap-4">
        {highlights.map(({ icon: Icon, text }) => (
          <li key={text} className="flex items-center gap-3 text-sm">
            <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-accent/15 text-accent-foreground">
              <Icon aria-hidden="true" className="size-5" />
            </span>
            {text}
          </li>
        ))}
      </ul>
      <div className="flex flex-col gap-3">
        <Button className="h-12 rounded-full text-base" disabled={busy || !configured} onClick={signIn}>
          {busy ? 'Waiting for Google…' : 'Continue with Google'}
        </Button>
        {!configured ? (
          <p className="text-center text-sm text-muted-foreground">
            Google sign-in is not configured on this server yet.
          </p>
        ) : null}
        {error ? (
          <p role="alert" className="text-center text-sm text-destructive">
            {error}
          </p>
        ) : null}
      </div>
    </div>
  )
}
