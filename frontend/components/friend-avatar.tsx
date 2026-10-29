'use client'

import { cn } from '@/lib/utils'
import { useLibrary } from '@/components/library-provider'
import { toneFor } from '@/lib/data'

const toneClasses = {
  1: 'bg-primary text-primary-foreground',
  2: 'bg-chart-2/15 text-chart-2',
  3: 'bg-chart-3/15 text-chart-3',
  4: 'bg-chart-4/25 text-foreground',
  5: 'bg-chart-5/15 text-chart-5',
}

const sizeClasses = {
  sm: 'size-7 text-[0.65rem]',
  md: 'size-10 text-sm',
  lg: 'size-16 text-xl',
}

export function initials(name: string) {
  return name
    .split(' ')
    .map((part) => part[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
}

/** Pass `friendId` for someone on Dog-ear, or `name` for a contact outside it. */
export function FriendAvatar({
  friendId,
  name,
  size = 'md',
  className,
}: {
  friendId?: string
  name?: string
  size?: keyof typeof sizeClasses
  className?: string
}) {
  const { person, me } = useLibrary()
  const label = friendId ? person(friendId).name : (name ?? '?')
  const tone = friendId === me.id ? 1 : toneFor(friendId ?? label)

  return (
    <span
      aria-hidden="true"
      className={cn(
        'inline-flex shrink-0 items-center justify-center rounded-full font-semibold',
        friendId ? toneClasses[tone] : 'border border-dashed bg-muted text-muted-foreground',
        sizeClasses[size],
        className,
      )}
    >
      {initials(label)}
    </span>
  )
}
