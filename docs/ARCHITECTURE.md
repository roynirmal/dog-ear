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
friendships  a, b (user IDs, a < b, unique pair)       mutual; invite code or accepted request
friend_requests  from, to, status (pending|declined), unique pair
books        owner, title, author, olKey, coverId, year, status (want|reading|finished),
             lendable, started, finished (YYYY-MM)
notes        owner, book (own shelf book ID or ""), text, tags[]
posts        owner, book{title, author, olKey, coverId, year}, status (reading|finished),
             progress 0-100, rating 1-5, thoughts, canLend, shelfBook
likes        post, user (unique pair)
comments     post, user, text
requests     book (owner's shelf book), owner, requester, status (pending|accepted|declined)
loans        lender, borrower (user IDs; one may be ""), contact (name of a person
             outside the app), book{...}, shelfBook (lender's copy, if linked),
             createdBy, since, due (YYYY-MM-DD), returned, nudged
notifications  user, actor, post, bookKey, read; unique (user, actor, bookKey)
```

Posts and loans copy the book details instead of relying on a shelf row, so
removing a book from your shelf never rewrites history.

### Lending from the shelf

Every book you post about goes on your shelf. Each shelf book has a `lendable`
switch (new books default to lendable unless only "want to read"; a post's "friends
can borrow my copy" sets it). Friends see your lendable books in `library` and can
ask to borrow any that are not lent out. A copy is lent out while a loan with that
`shelfBook` has no `returned` date; a copy can be on only one active loan. Loans
logged by hand link the lender's matching shelf copy (by Open Library key or title
and author) when it is free.

### Reading months

Shelf books record the month you started and finished them. Moving a book to
"reading" from any other status stamps this month as the start and clears the
finish (so a re-read starts over); finishing stamps the finish month if it has
none; "want to read" clears both. Explicit `started` / `finished` values from the
share dialog, add-book dialog or "Edit dates" then override (`""` clears one). Months
cannot be in the future and the finish cannot precede the start. Posts show the
author's months for that book.

### Notifications

The only notification: when a friend posts that they are reading a book, each of
their current friends gets one in-app notification. The unique index on
`(user, actor, bookKey)` makes it once per friend per book, ever (`bookKey` is the
Open Library work key, or lowercase title|author). Progress updates and finished
posts never notify. There is no email or push.

## API

All domain calls are `POST /api/dogear/{action}` with a JSON body, a PocketBase
bearer token and a verified `users` record. Every action runs in one
`RunInTransaction`, authorizes against current data, mutates, and returns the
caller's full `State` (`me, code, people, friends, friendRequests, sentRequests,
posts, loans, requests, shelf, library, notes, notifications`). The frontend replaces its state with each response; a 60-second and
on-focus refresh picks up friends' changes.

| Action | Body | Rule |
| --- | --- | --- |
| `state` | `{}` | |
| `befriend` | `code` | Creates a mutual friendship; own code or repeat is a no-op |
| `unfriend` | `id` | Either side; loans between you remain visible |
| `friend-request` | `id` (user) | Requires a mutual friend; asking someone who asked you accepts; repeats and declined requests are quiet no-ops |
| `friend-respond` | `id, accept` | Recipient only |
| `shelf-add` | `book, status, lendable?, started?, finished?` | Upserts by Open Library key or case-insensitive title+author |
| `shelf-update` | `id, status?, lendable?, started?, finished?` | Owner only |
| `shelf-remove` | `id` | Owner only; unlinks notes, drops pending borrow requests |
| `note-save` | `id?, bookId?, text, tags[]` | Owner only; `bookId` must be your shelf book |
| `note-delete` | `id` | Owner only |
| `post` | `book, status, progress/rating, thoughts, canLend, started?, finished?` | Upserts the shelf entry and its `lendable`; a reading post notifies friends once per book |
| `post-delete` | `id` | Author only; removes likes, comments, its notifications |
| `like` (toggle), `comment` | `id[, text]` | Post must be yours or a current friend's |
| `request` | `id` (shelf book) | A friend's lendable copy that is not lent out; one pending request per person |
| `respond` | `id, accept` | Book owner; accepting needs a free copy and a current friend, and creates a linked loan |
| `loan` | `direction, friend \| contact, book, shelfBook?, due?` | `friend` must be a current friend; `shelfBook` is your free copy when lending |
| `return`, `unreturn` | `id` | Either party; undo is refused if the copy is lent out again |
| `nudge` | `id` | Lender only, active loan with an app friend |
| `loan-delete` | `id` | Whoever logged it |
| `notifications-read` | `{}` | Marks your notifications read |

`GET /api/dogear/search?q=` (authenticated) proxies Open Library's search with an
identifying User-Agent, a 10-second timeout and a 1-hour in-memory cache.
`GET /api/dogear/config` is public and reports only whether Google sign-in is set up.

Unknown JSON fields are rejected; bodies are limited to 64 KiB. Text limits: title
300, author 200, thoughts 5,000, comment 1,000, note 10,000 characters; up to 10 tags
of 32 characters, normalized to lowercase-hyphenated.

## Privacy

- Feed posts are visible to the author and the author's current friends. Comments
  and hearts are visible to whoever can see the post, which can include friends of
  the author you are not friends with yourself. Tapping their name offers a friend
  request; requests are only allowed between people with a mutual friend.
- Notes are private. Of your shelf, friends see only lendable books (and whether
  they are lent out); the rest is private.
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
