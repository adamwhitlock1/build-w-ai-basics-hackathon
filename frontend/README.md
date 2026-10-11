# Porchlight frontend

React 18 + TypeScript + Vite, styled with Tailwind CSS v4, with TanStack Router (file-based, SPA) and TanStack Query.

```sh
npm install
npm run dev     # http://localhost:5174 — /api is proxied to the Go backend on :8080
npm run build   # build to dist/ (vite build), then type-check (tsc -b)
npm run lint    # oxlint
```

Port 5174 is fixed (`strictPort`) because 5173 is used by another app on this machine.

## Routes

Each file in `src/routes` is a page; the Vite plugin turns them into `src/routeTree.gen.ts`
while `npm run dev` or `npm run build` runs. `npm run build` runs Vite before `tsc -b` for
this reason: a route you just added or renamed is in the tree before it's type-checked. The
file is generated but committed, so `tsc -b` works on a fresh checkout — don't edit it by hand.

| URL              | File                          | Who sees it                                   |
| ---------------- | ----------------------------- | --------------------------------------------- |
| `/login`         | `routes/login.tsx`            | logged out (logged in → `/`)                  |
| `/signup`        | `routes/signup.tsx`           | logged out (logged in → `/`)                  |
| `/create-family` | `routes/create-family.tsx`    | logged in, family not set up yet              |
| `/`              | `routes/_authed/index.tsx`    | logged in with a family (guarded by `_authed.tsx`) |

The guards only make the app feel right; the Go API checks the session on every private request.

## Test overrides

`PORCHLIGHT_API` (default `http://127.0.0.1:8080`) and `PORCHLIGHT_WEB_PORT` (default `5174`)
point a second dev server at another API, e.g.
`PORCHLIGHT_API=http://127.0.0.1:8091 PORCHLIGHT_WEB_PORT=5191 npm run dev`.
