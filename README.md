# Dog-ear

Share what you're reading with friends, and keep track of the books you lend and borrow.

- Add and manage your book collection (want to read / reading / finished), and
  choose which books friends may borrow
- Online book search, powered by [Open Library](https://openlibrary.org)
- Private notes, grouped with tags and optionally linked to a book
- A record of who you lend books to and borrow from, with due dates, nudges and history
- See what friends are reading and their thoughts; heart, comment, and borrow from
  their lending shelf
- Add friends by invite link or by tapping someone in the comments
- One in-app notification when a friend starts reading a book

Built with Next.js (static export) and PocketBase extended in Go, the same backend
stack as payback.

## Setup

Requires Go 1.27 (or automatic Go toolchain downloads), Node.js 22+, and npm.

```sh
make install
```

Configure Google sign-in in the backend shell; `.env` files are not loaded:

| Variable                                   | Purpose                                                                               |
| ------------------------------------------ | ------------------------------------------------------------------------------------- |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | Google sign-in credentials (required, or configure the Google provider in PocketBase) |

Register each origin you use as an authorized JavaScript origin of the Google OAuth
client, and `<origin>/api/oauth2-redirect` as a redirect URI:

- `http://127.0.0.1:8091` for `make up` locally, and also for `make dev` (the browser
  talks to PocketBase directly in dev, so the redirect is on 8091)
- your public origin, e.g. `https://dogear.example.com`, when served through a tunnel

PocketBase applies migrations on startup and stores data in `backend/pb_data/`.
Its admin dashboard is at http://127.0.0.1:8091/_/.

## Run

```sh
make up
```

Builds the frontend into `backend/pb_public/` and serves the whole app from
PocketBase on **http://127.0.0.1:8091**. To publish it, point a named Cloudflare
Tunnel (or any reverse proxy) at that port; there is only one service.

For frontend work with hot reload:

```sh
make dev
```

Opens Next.js on http://127.0.0.1:5174 against the backend on 8091. Restart after Go
changes. Ports 8090/5173 are left free for payback.

## Test

```sh
make test
```

Runs gofmt, `go vet`, backend tests (OAuth and Open Library stubbed), Prettier, the
TypeScript check, unit tests and the static export build. CI runs the same checks.

## Code organisation

```text
backend/schema*.go       Migrations: locked collections and users hardening
backend/service.go       Every domain action, authorized inside one transaction
backend/state.go         The per-user state every action returns
backend/openlibrary.go   Cached Open Library search proxy
backend/main.go          HTTP routes, Google-only OAuth hook, static frontend
frontend/app/            Next.js entry, layout, manifest
frontend/components/     Views, dialogs and the LibraryProvider (server state)
frontend/lib/            API client, types and date/tag helpers
scripts/dev.sh           Runs backend and Next dev server together
```

See [architecture](docs/ARCHITECTURE.md) for the data model, API and privacy rules.
