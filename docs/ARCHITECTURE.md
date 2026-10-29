# Architecture

## Shape

- Next.js / React / Tailwind UI, exported as static files (`output: 'export'`) and
  served by PocketBase from `backend/pb_public`. All data loading is client-side
  through the PocketBase JS SDK (auth) and `fetch` (domain API).
- PocketBase extended in Go: Google OAuth, SQLite, migrations and custom routes.
- No other services. Open Library is called from Go only.

## Storage

PocketBase's `users` auth collection holds the Google-verified account plus `name`
and a random 16-character `code` used in invite links. Application collections are
plain base collections with **no API rules** (superuser-only); clients cannot read
or write them through `/api/collections/...`.

```text
friendships  a, b (user IDs, a < b, unique pair)       mutual, created by invite code
books        owner, title, author, olKey, coverId, year, status (want|reading|finished)
notes        owner, book (own shelf book ID or ""), text, tags[]
posts        owner, book{title, author, olKey, coverId, year}, status (reading|finished),
             progress 0-100, rating 1-5, thoughts, canLend
likes        post, user (unique pair)
comments     post, user, text
requests     post, owner, requester, status (pending|accepted|declined)
loans        lender, borrower (user IDs; one may be ""), contact (name of a person
             outside the app), book{...}, createdBy, since, due (YYYY-MM-DD),
             returned, nudged
```

Posts and loans copy the book details instead of referencing a shelf row, so
removing a book from your shelf never rewrites history.

## API

All domain calls are `POST /api/dogear/{action}` with a JSON body, a PocketBase
bearer token and a verified `users` record. Every action runs in one
`RunInTransaction`, authorizes against current data, mutates, and returns the
caller's full `State` (`me, code, people, friends, posts, loans, requests, shelf,
notes`). The frontend replaces its state with each response; a 60-second and
on-focus refresh picks up friends' changes.

| Action | Body | Rule |
| --- | --- | --- |
| `state` | `{}` | |
| `befriend` | `code` | Creates a mutual friendship; own code or repeat is a no-op |
| `unfriend` | `id` | Either side; loans between you remain visible |
| `shelf-add` | `book, status` | Upserts by Open Library key or case-insensitive title+author |
| `shelf-update`, `shelf-remove` | `id[, status]` | Owner only; removal unlinks notes |
| `note-save` | `id?, bookId?, text, tags[]` | Owner only; `bookId` must be your shelf book |
| `note-delete` | `id` | Owner only |
| `post` | `book, status, progress/rating, thoughts, canLend` | Also upserts the shelf entry |
| `post-delete` | `id` | Author only; removes likes, comments, requests |
| `like` (toggle), `comment` | `id[, text]` | Post must be yours or a current friend's |
| `request` | `id` | Friend's post with `canLend`; one pending request per person |
| `respond` | `id, accept` | Post author; accepting creates a loan to the requester |
| `loan` | `direction, friend \| contact, book, due?` | `friend` must be a current friend |
| `return`, `unreturn` | `id` | Either party |
| `nudge` | `id` | Lender only, active loan with an app friend |
| `loan-delete` | `id` | Whoever logged it |

`GET /api/dogear/search?q=` (authenticated) proxies Open Library's search with an
identifying User-Agent, a 10-second timeout and a 1-hour in-memory cache.
`GET /api/dogear/config` is public and reports only whether Google sign-in is set up.

Unknown JSON fields are rejected; bodies are limited to 64 KiB. Text limits: title
300, author 200, thoughts 5,000, comment 1,000, note 10,000 characters; up to 10 tags
of 32 characters, normalized to lowercase-hyphenated.

## Privacy

- Feed posts are visible to the author and the author's current friends. Comments
  and hearts are visible to whoever can see the post, which can include friends of
  the author you are not friends with yourself (their names are shown).
- Shelf and notes are private to their owner.
- A loan is visible to its lender and borrower. Unfriending does not hide it.
- Invite links are bearer links: anyone with the link who signs in becomes your
  friend. There is no code rotation yet.

## Auth

Same hardening as payback: users can only be created through PocketBase's OAuth2
flow, a hook requires the `google` provider with an email and overrides any
client-supplied `createData`, and password auth, OTP, email changes and generic user
updates are disabled. Superusers are for administration only and are not app users.

## Limits and tradeoffs

The feed loads the newest 200 posts from you and your friends without pagination,
and every action returns the full state. That is fine for a circle of friends, not
for large networks. There is no push or email notification; nudges and borrow
requests appear in the app on the next refresh.
