'use client'

import { useEffect, useState } from 'react'
import { useLibrary } from '@/components/library-provider'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { parseTags, type Note } from '@/lib/data'

export function NoteDialog({
  open,
  note,
  bookId: initialBookId,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  /** Edit this note; omit to write a new one. */
  note?: Note
  bookId?: string
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const { shelf, notes, saveNote, deleteNote } = useLibrary()
  const [text, setText] = useState('')
  const [tags, setTags] = useState('')
  const [bookId, setBookId] = useState('')
  const [busy, setBusy] = useState(false)

  const knownTags = [...new Set(notes.flatMap((n) => n.tags))].sort()
  const chosen = parseTags(tags)

  useEffect(() => {
    if (!open) return
    setText(note?.text ?? '')
    setTags(note?.tags.join(', ') ?? '')
    setBookId(note?.bookId ?? initialBookId ?? '')
  }, [open, note, initialBookId])

  function addTag(tag: string) {
    setTags([...chosen, tag].join(', '))
  }

  async function act(work: () => Promise<void>) {
    if (busy) return
    setBusy(true)
    try {
      await work()
      onOpenChange(false)
      onSaved()
    } catch {
      // The provider shows the error; keep the draft.
    } finally {
      setBusy(false)
    }
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!text.trim()) return
    act(() => saveNote({ id: note?.id, bookId: bookId || undefined, text: text.trim(), tags: chosen }))
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto rounded-2xl p-5 sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-serif text-xl">{note ? 'Edit note' : 'New note'}</DialogTitle>
          <DialogDescription>Notes are private to you.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex min-w-0 flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="note-text">Note</Label>
            <Textarea
              id="note-text"
              required
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder="A quote, an idea, a question for book club…"
              rows={6}
              maxLength={10000}
              className="text-base"
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="note-book">
              Book <span className="font-normal text-muted-foreground">(optional)</span>
            </Label>
            <select
              id="note-book"
              value={bookId}
              onChange={(e) => setBookId(e.target.value)}
              className="h-11 rounded-lg border border-input bg-transparent px-3 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <option value="">Not about a particular book</option>
              {shelf.map((book) => (
                <option key={book.id} value={book.id}>
                  {book.title}
                </option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="note-tags">Tags</Label>
            <Input
              id="note-tags"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              placeholder="quotes, book club"
              className="h-11 text-base"
            />
            {knownTags.filter((t) => !chosen.includes(t)).length > 0 ? (
              <div className="flex flex-wrap gap-1.5">
                {knownTags
                  .filter((t) => !chosen.includes(t))
                  .map((tag) => (
                    <button
                      key={tag}
                      type="button"
                      onClick={() => addTag(tag)}
                      className="min-h-8 rounded-full border bg-card px-3 text-xs font-medium hover:bg-muted"
                    >
                      #{tag}
                    </button>
                  ))}
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">Separate tags with commas.</p>
            )}
          </div>
          <Button type="submit" className="h-12 rounded-full text-base" disabled={!text.trim() || busy}>
            {busy ? 'Saving…' : 'Save note'}
          </Button>
          {note ? (
            <Button
              type="button"
              variant="destructive"
              className="h-11 rounded-full"
              disabled={busy}
              onClick={() => {
                if (window.confirm('Delete this note?')) act(() => deleteNote(note.id))
              }}
            >
              Delete note
            </Button>
          ) : null}
        </form>
      </DialogContent>
    </Dialog>
  )
}
