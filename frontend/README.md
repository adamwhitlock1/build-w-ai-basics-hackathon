# Porchlight frontend

React 18 + TypeScript + Vite, styled with Tailwind CSS v4 and TanStack Query.

```sh
npm install
npm run dev     # http://localhost:5174 — /api is proxied to the Go backend on :8080
npm run build   # type-check (tsc -b) and build to dist/
npm run lint    # oxlint
```

Port 5174 is fixed (`strictPort`) because 5173 is used by another app on this machine.
