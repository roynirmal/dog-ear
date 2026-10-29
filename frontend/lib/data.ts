export type CoverTone = 1 | 2 | 3 | 4 | 5

export type Book = {
  title: string
  author: string
  olKey?: string
  coverId?: number
  year?: number
}

export type Person = {
  id: string
  name: string
}

export type ReadingStatus = 'reading' | 'finished'

export type ShelfStatus = 'want' | 'reading' | 'finished'

export type Comment = {
  id: string
  authorId: string
  text: string
  createdAt: string
}

export type Post = {
  id: string
  authorId: string
  book: Book
  status: ReadingStatus
  progress?: number
  rating?: number
  thoughts: string
  createdAt: string
  likedBy: string[]
  comments: Comment[]
  canLend: boolean
  requested: boolean
}

export type LoanDirection = 'lent' | 'borrowed'

export type Loan = {
  id: string
  direction: LoanDirection
  /** The other person when they are on Dog-ear; otherwise `contact` names them. */
  friendId?: string
  contact?: string
  book: Book
  since: string
  /** Calendar date, YYYY-MM-DD. */
  dueDate?: string
  returnedAt?: string
  nudgedAt?: string
  /** Logged by the current user, who alone may delete it. */
  mine: boolean
}

export type BorrowRequest = {
  id: string
  postId: string
  book: Book
  requesterId: string
  createdAt: string
}

export type ShelfBook = Book & {
  id: string
  status: ShelfStatus
  addedAt: string
}

export type Note = {
  id: string
  bookId?: string
  text: string
  tags: string[]
  createdAt: string
  updatedAt: string
}

export type LibraryState = {
  me: Person
  code: string
  people: Person[]
  friends: string[]
  posts: Post[]
  loans: Loan[]
  requests: BorrowRequest[]
  shelf: ShelfBook[]
  notes: Note[]
}

export const shelfStatusLabels: Record<ShelfStatus, string> = {
  want: 'Want to read',
  reading: 'Reading',
  finished: 'Finished',
}

function hash(text: string) {
  let h = 0
  for (let i = 0; i < text.length; i++) h = (h * 31 + text.charCodeAt(i)) | 0
  return Math.abs(h)
}

/** Stable colour for a person or a coverless book. */
export function toneFor(key: string): CoverTone {
  return ((hash(key) % 5) + 1) as CoverTone
}

export function coverUrl(book: Book, size: 'S' | 'M' | 'L' = 'M') {
  return book.coverId ? `https://covers.openlibrary.org/b/id/${book.coverId}-${size}.jpg` : undefined
}

export function firstName(name: string) {
  return name.split(' ')[0]
}

/** Lowercase, de-duplicated tags from free text such as "#quotes, sci fi". */
export function parseTags(input: string) {
  const tags: string[] = []
  for (const raw of input.split(',')) {
    const tag = raw.trim().replace(/^#+/, '').toLowerCase().split(/\s+/).filter(Boolean).join('-')
    if (tag && !tags.includes(tag)) tags.push(tag)
  }
  return tags
}

/** YYYY-MM-DD dates are calendar days in the reader's own timezone. */
function parseDay(iso: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso)
  return match ? new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3])) : new Date(iso)
}

/** Sortable timestamp for a YYYY-MM-DD or ISO date. */
export function parseDayValue(iso: string) {
  return parseDay(iso).getTime()
}

export function relativeTime(iso: string) {
  const diffMs = Date.now() - new Date(iso).getTime()
  const minutes = Math.round(diffMs / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.round(hours / 24)
  if (days < 7) return `${days}d ago`
  return shortDate(iso)
}

export function daysUntil(iso: string, today = new Date()) {
  const start = new Date(today)
  start.setHours(0, 0, 0, 0)
  const end = parseDay(iso)
  end.setHours(0, 0, 0, 0)
  return Math.round((end.getTime() - start.getTime()) / 86400000)
}

export function shortDate(iso: string) {
  return parseDay(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

export function isOverdue(loan: Loan) {
  return !loan.returnedAt && !!loan.dueDate && daysUntil(loan.dueDate) < 0
}
