import { describe, expect, it } from 'vitest'
import { coverUrl, daysUntil, isOverdue, parseTags, toneFor, type Loan } from './data'

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
  })
  it('gives a stable tone between 1 and 5', () => {
    expect(toneFor('Piranesi')).toBe(toneFor('Piranesi'))
    for (const key of ['a', 'bb', 'ccc', 'dddd', 'eeeee']) expect(toneFor(key)).toBeGreaterThanOrEqual(1)
  })
})
