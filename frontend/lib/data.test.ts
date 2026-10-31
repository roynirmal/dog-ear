import { describe, expect, it } from 'vitest'
import { coverUrl, daysUntil, isOverdue, parseTags, readingSpan, thisMonth, toneFor, type Loan } from './data'

describe('parseTags', () => {
  it('normalizes, de-duplicates and hyphenates', () => {
    expect(parseTags('#Quotes, quotes,  sci fi , ,##Book Club')).toEqual(['quotes', 'sci-fi', 'book-club'])
  })
})

describe('daysUntil', () => {
  const today = new Date(2026, 9, 30, 23, 30)
  it('treats YYYY-MM-DD as a local calendar day', () => {
    expect(daysUntil('2026-10-30', today)).toBe(0)
    expect(daysUntil('2026-10-31', today)).toBe(1)
    expect(daysUntil('2026-10-27', today)).toBe(-3)
  })
})

describe('isOverdue', () => {
  const loan: Loan = {
    id: 'l',
    direction: 'lent',
    book: { title: 'A', author: 'B' },
    since: '2026-01-01T00:00:00Z',
    mine: true,
  }
  it('ignores returned loans and loans without a due date', () => {
    expect(isOverdue(loan)).toBe(false)
    expect(isOverdue({ ...loan, dueDate: '2000-01-01' })).toBe(true)
    expect(isOverdue({ ...loan, dueDate: '2000-01-01', returnedAt: '2000-01-02T00:00:00Z' })).toBe(false)
  })
})

describe('covers and tones', () => {
  it('uses Open Library covers only when a cover ID exists', () => {
    expect(coverUrl({ title: 'A', author: '' })).toBeUndefined()
    expect(coverUrl({ title: 'A', author: '', coverId: 42 }, 'S')).toBe('https://covers.openlibrary.org/b/id/42-S.jpg')
    const google = 'https://books.google.com/books/content?id=abc&img=1&zoom=1'
    expect(coverUrl({ title: 'A', author: '', coverUrl: google })).toBe(google)
    expect(coverUrl({ title: 'A', author: '', coverId: 42, coverUrl: google }, 'S')).toContain('openlibrary')
  })
  it('gives a stable tone between 1 and 5', () => {
    expect(toneFor('Piranesi')).toBe(toneFor('Piranesi'))
    for (const key of ['a', 'bb', 'ccc', 'dddd', 'eeeee']) expect(toneFor(key)).toBeGreaterThanOrEqual(1)
  })
})

describe('reading months', () => {
  it('formats this month as YYYY-MM', () => {
    expect(thisMonth(new Date(2026, 0, 31))).toBe('2026-01')
  })
  it('summarizes a read', () => {
    expect(readingSpan('2026-03')).toBe('Since Mar 2026')
    expect(readingSpan('2026-03', '2026-10')).toBe('Mar – Oct 2026')
    expect(readingSpan('2025-11', '2026-02')).toBe('Nov 2025 – Feb 2026')
    expect(readingSpan('2026-05', '2026-05')).toBe('May 2026')
    expect(readingSpan(undefined, '2026-05')).toBe('Finished May 2026')
    expect(readingSpan()).toBe('')
  })
})
