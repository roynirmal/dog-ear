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

## Local development

Requires Go 1.27 (or automatic toolchain downloads), Node.js 22+, and npm.

```sh
make install
make dev
```

Open http://127.0.0.1:5174; PocketBase admin is at http://127.0.0.1:8091/_/.
Restart after Go changes; Ctrl+C stops both servers.

For Google login, enable Google under PocketBase > users > Options > OAuth2 and
enter your client ID and secret (or export `GOOGLE_CLIENT_ID` and
`GOOGLE_CLIENT_SECRET` before starting; `.env` files are not loaded).
In the Google OAuth client, add origins `http://127.0.0.1:5174` and
`http://127.0.0.1:8091`, with redirect URI
`http://127.0.0.1:8091/api/oauth2-redirect`. If using `localhost:8091`, also register
origin `http://localhost:8091` and redirect
`http://localhost:8091/api/oauth2-redirect`; Google requires an exact host match.
Keep the production origin `https://dog-ear.app` and redirect
`https://dog-ear.app/api/oauth2-redirect` registered too.

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

## Install on a phone

Dog-ear is a progressive web app. Over HTTPS (your public URL), Android and desktop
Chrome offer an install prompt (also under You → Install Dog-ear); on iPhone use
Share → Add to Home Screen. `frontend/public/sw.js` caches the app shell and book
covers so it opens instantly and offline; library data is never cached. Bump
`VERSION` in `sw.js` if you change its caching rules. The service worker is only
registered in production builds (`make up`), not in `make dev`.

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
