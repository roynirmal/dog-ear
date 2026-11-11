'use client'

import { useEffect, useRef, useState } from 'react'
import { Camera, Loader2, RotateCcw } from 'lucide-react'
import { BookCover } from '@/components/book-cover'
import { FilterChips } from '@/components/filter-chips'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'
import { scanShelf } from '@/lib/api'
import { shelfStatusOptions, toBook, type ScannedBook, type ShelfStatus } from '@/lib/data'

type Row = ScannedBook & { selected: boolean }

/** Keep the upload small enough to send, while preserving spine text. */
async function compressImage(file: File): Promise<File> {
  if (!file.type.startsWith('image/')) throw new Error('Choose a photo of your bookshelf.')
  try {
    const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
    const max = 1600
    const scale = Math.min(1, max / Math.max(bitmap.width, bitmap.height))
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(bitmap.width * scale))
    canvas.height = Math.max(1, Math.round(bitmap.height * scale))
    const context = canvas.getContext('2d')
    if (!context) return file
    context.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
    bitmap.close()
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.9))
    if (!blob) return file
    return new File([blob], 'shelf.jpg', { type: 'image/jpeg' })
  } catch {
    return file
  }
}

export function ShelfScan({ onAdded }: { onAdded: () => void }) {
  const { addManyToShelf } = useLibrary()
  const [photo, setPhoto] = useState<File | null>(null)
  const [preview, setPreview] = useState('')
  const [rows, setRows] = useState<Row[]>([])
  const [unreadable, setUnreadable] = useState(0)
  const [warning, setWarning] = useState('')
  const [status, setStatus] = useState<ShelfStatus>('want')
  const [scanning, setScanning] = useState(false)
  const [adding, setAdding] = useState(false)
  const [scanned, setScanned] = useState(false)
  const [scansLeft, setScansLeft] = useState<number | null>(null)
  const [error, setError] = useState('')
  const input = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!photo) {
      setPreview('')
      return
    }
    const url = URL.createObjectURL(photo)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [photo])

  async function choose(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    setError('')
    setRows([])
    setUnreadable(0)
    setWarning('')
    setScanned(false)
    try {
      setPhoto(await compressImage(file))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function runScan() {
    if (!photo || scanning) return
    setScanning(true)
    setError('')
    setWarning('')
    try {
      const result = await scanShelf(photo)
      setRows(
        result.books.map((book) => ({
          ...book,
          selected: book.confidence !== 'low' && !!book.title.trim(),
        })),
      )
      setUnreadable(result.unreadable)
      setWarning(result.warning || '')
      setScansLeft(result.scansLeft)
      setScanned(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'The shelf scan failed. Please try again.')
    } finally {
      setScanning(false)
    }
  }

  function reset() {
    setPhoto(null)
    setRows([])
    setUnreadable(0)
    setWarning('')
    setScanned(false)
    setScansLeft(null)
    setError('')
  }

  function toggle(index: number) {
    setRows((current) => current.map((row, i) => (i === index ? { ...row, selected: !row.selected } : row)))
  }

  function edit(index: number, field: 'title' | 'author', value: string) {
    setRows((current) =>
      current.map((row, i) => {
        if (i !== index) return row
        if (field === 'author') return { ...row, author: value }
        // A changed title no longer belongs to the catalogue match.
        return { ...row, title: value, matched: false, olKey: '', coverId: 0, coverUrl: '', year: 0 }
      }),
    )
  }

  const chosen = rows.filter((row) => row.selected && row.title.trim())
  const allSelected = rows.length > 0 && rows.every((row) => row.selected)

  async function addSelected() {
    if (!chosen.length || adding) return
    setAdding(true)
    setError('')
    try {
      await addManyToShelf(chosen.map(toBook), status)
      onAdded()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Those books could not be added.')
    } finally {
      setAdding(false)
    }
  }

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <input
        ref={input}
        type="file"
        accept="image/*"
        capture="environment"
        className="hidden"
        onChange={choose}
        aria-label="Choose a bookshelf photo"
      />

      {rows.length === 0 ? (
        <>
          <div className="rounded-2xl border border-dashed bg-muted/20 p-4">
            {preview ? (
              // A local object URL; next/image adds nothing in a static export.
              // eslint-disable-next-line @next/next/no-img-element
              <img src={preview} alt="Bookshelf to scan" className="mx-auto max-h-60 w-full rounded-xl object-cover" />
            ) : (
              <div className="flex flex-col items-center gap-2 py-8 text-center">
                <Camera aria-hidden="true" className="size-8 text-muted-foreground" />
                <p className="font-serif text-lg font-semibold">Photograph a shelf</p>
                <p className="max-w-xs text-sm text-muted-foreground">
                  Fill the frame with book spines in good light. We&apos;ll read them and look up each book.
                </p>
              </div>
            )}
          </div>
          <Button
            type="button"
            variant={preview ? 'outline' : 'default'}
            className="h-12 rounded-full text-base"
            disabled={scanning}
            onClick={() => input.current?.click()}
          >
            <Camera className="size-4" />
            {preview ? 'Choose another photo' : 'Take or choose a photo'}
          </Button>
          {preview ? (
            <Button type="button" className="h-12 rounded-full text-base" disabled={scanning} onClick={runScan}>
              {scanning ? <Loader2 className="size-4 animate-spin" /> : null}
              {scanning ? 'Reading the spines…' : 'Find books'}
            </Button>
          ) : null}
          {scanning ? (
            <p className="text-center text-sm text-muted-foreground">
              Usually 15–30 seconds. Keep this open while we read the shelf.
            </p>
          ) : null}
          {scanned && rows.length === 0 && !scanning ? (
            <p className="text-center text-sm text-muted-foreground">
              No books were readable. Try a closer, brighter photo.
            </p>
          ) : null}
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
          <p className="text-xs text-muted-foreground">
            The photo is sent to Google Gemini to read the spines; it is not stored. Only the extracted titles go to
            Google Books. You can scan up to 10 shelf photos a day.
          </p>
        </>
      ) : (
        <>
          <div className="flex items-center justify-between gap-3">
            <p className="text-sm font-medium">
              {rows.length} book{rows.length === 1 ? '' : 's'} found
            </p>
            <button
              type="button"
              className="text-sm font-medium text-primary underline-offset-4 hover:underline"
              onClick={() => setRows((current) => current.map((row) => ({ ...row, selected: !allSelected })))}
            >
              {allSelected ? 'Clear all' : 'Select all'}
            </button>
          </div>
          {warning ? <p className="text-sm text-amber-600">{warning}</p> : null}
          <ul className="flex max-h-[42dvh] flex-col divide-y overflow-y-auto rounded-2xl border bg-card">
            {rows.map((book, index) => (
              <li key={`${book.rawTitle}-${index}`} className="flex items-start gap-3 p-3">
                <input
                  type="checkbox"
                  className="mt-4 size-4 shrink-0 accent-primary"
                  checked={book.selected}
                  onChange={() => toggle(index)}
                  aria-label={`Include ${book.title || 'this book'}`}
                />
                <BookCover book={book} size="sm" />
                <div className="min-w-0 flex-1">
                  <input
                    value={book.title}
                    onChange={(event) => edit(index, 'title', event.target.value)}
                    aria-label="Title"
                    placeholder="Unreadable title"
                    className="w-full min-w-0 bg-transparent font-serif text-sm leading-snug font-semibold outline-none focus:underline"
                  />
                  <input
                    value={book.author}
                    onChange={(event) => edit(index, 'author', event.target.value)}
                    aria-label="Author"
                    placeholder="Add author"
                    className="w-full min-w-0 bg-transparent text-xs text-muted-foreground outline-none focus:underline"
                  />
                  <div className="mt-1 flex flex-wrap items-center gap-1.5">
                    {book.matched ? (
                      <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[0.65rem] font-medium text-primary">
                        Google Books
                      </span>
                    ) : (
                      <span className="rounded-full bg-amber-500/10 px-2 py-0.5 text-[0.65rem] font-medium text-amber-700 dark:text-amber-400">
                        Check this one
                      </span>
                    )}
                    {book.confidence !== 'high' ? (
                      <span className="rounded-full bg-muted px-2 py-0.5 text-[0.65rem] text-muted-foreground">
                        {book.confidence} confidence
                      </span>
                    ) : null}
                  </div>
                </div>
              </li>
            ))}
          </ul>
          {unreadable > 0 ? (
            <p className="text-xs text-muted-foreground">
              {unreadable} spine{unreadable === 1 ? ' was' : 's were'} unreadable and {unreadable === 1 ? 'is' : 'are'}{' '}
              not listed.
            </p>
          ) : null}
          <FilterChips label="Add as" wrap value={status} onChange={setStatus} options={shelfStatusOptions} />
          <Button
            type="button"
            className="h-12 rounded-full text-base"
            disabled={!chosen.length || adding}
            onClick={addSelected}
          >
            {adding ? 'Adding…' : `Add ${chosen.length} to shelf`}
          </Button>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
          {scansLeft !== null ? (
            <p className="text-xs text-muted-foreground">
              {scansLeft === 0
                ? 'You’ve used all 10 shelf scans for today.'
                : `${scansLeft} shelf scan${scansLeft === 1 ? '' : 's'} left today.`}
            </p>
          ) : null}
          <div className="flex items-center justify-between gap-3">
            <button
              type="button"
              onClick={reset}
              className="text-sm font-medium text-primary underline-offset-4 hover:underline"
            >
              Scan another photo
            </button>
            <button
              type="button"
              onClick={runScan}
              disabled={scanning || scansLeft === 0}
              className="inline-flex items-center gap-1 text-sm text-muted-foreground underline-offset-4 hover:underline disabled:no-underline disabled:opacity-50"
            >
              <RotateCcw className="size-3.5" />
              {scanning ? 'Scanning…' : scansLeft === 0 ? 'No scans left today' : 'Try again'}
            </button>
          </div>
          <p className="text-xs text-muted-foreground">
            Check titles and authors before adding; thin spines and editions can be misread.
          </p>
        </>
      )}
    </div>
  )
}
