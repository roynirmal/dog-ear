'use client'

import { cn } from '@/lib/utils'

type Option<T extends string> = { value: T; label: string; count?: number }

export function FilterChips<T extends string>({
  options,
  value,
  onChange,
  label,
  wrap = false,
}: {
  options: readonly Option<T>[]
  value: T
  onChange: (value: T) => void
  label: string
  /** Wrap onto more lines instead of scrolling sideways. */
  wrap?: boolean
}) {
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className={cn('flex gap-2', wrap ? 'flex-wrap' : 'overflow-x-auto [scrollbar-width:none]')}
    >
      {options.map((option) => {
        const active = option.value === value
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(option.value)}
            className={cn(
              'inline-flex h-11 shrink-0 items-center gap-1.5 rounded-full border px-4 text-sm font-medium transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
              active ? 'border-primary bg-primary text-primary-foreground' : 'bg-card text-foreground hover:bg-muted',
            )}
          >
            {option.label}
            {option.count !== undefined ? (
              <span
                className={cn(
                  'rounded-full px-1.5 text-xs tabular-nums',
                  active ? 'bg-primary-foreground/20' : 'bg-muted text-muted-foreground',
                )}
              >
                {option.count}
              </span>
            ) : null}
          </button>
        )
      })}
    </div>
  )
}
