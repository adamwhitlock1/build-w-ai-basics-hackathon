## Context

See proposal.md for why. The current state that shapes the approach:

- **Curated widgets** (`internal/widgets/curated.go`) are worked out on every `GET /api/widgets/articles|events`, and on the parents-only `/api/members/{id}/widgets/...` versions that share the same handlers through `auth.SubjectFrom`. On the frontend, `curatedWidgetQueryOptions` holds the result for the visit (`staleTime: Infinity`), and `reseedCuratedWidgets` refetches when interests or location change.
- **Described widgets** are saved in `custom_widgets` with `source`, `query` and a `render_spec` (title, archetype, items). They're served inside `GET /api/me/homepage` (`customWidgets`), and `CustomBlock` reads them from that cached page. The prompt isn't saved, but `llm.ShapeItems` needs it. It decides `future_only` and what the items should be about.
- SQLite through `internal/store`; migrations are embedded `.sql` files applied in order (`001`, `002`).
- There are no frontend tests. Go tests use fake LLM and search clients. Verification is `make check`, `npm run build`, `npm run lint`, and driving the app in the browser.

## Goals / Non-Goals

**Goals:**
- One way of behaving for every widget kind: show what's remembered, refresh when stale or asked.
- The server decides freshness (the windows live in Go), so the frontend never hard-codes them.
- Keep the existing handler and query structure, and add to it instead of restructuring.

**Non-Goals:**
- Refreshing on a timer while a page stays open. A refresh only happens on page load or by request.
- Re-planning a described widget (choosing a new source, query, title or archetype). Refresh re-runs the saved plan.
- Refreshing a family member's described widgets from a parent's view.
- Caching anything in memory or in a separate cache service. SQLite is enough at family scale.
- Rate limits per user or per day. Those belong to the deployment change; this change only adds the 1-minute repeat guard.

## Decisions

### 1. Curated results go in their own table, keyed by member and kind
A new table, `curated_widget_results`, with:
- `user_id`, `kind` (`articles` | `events`), primary key (`user_id`, `kind`);
- `inputs`: the normalized interests and location the result was picked for;
- `payload`: the `Widget` JSON;
- `updated_at`.

The handler compares `inputs` with the member's current profile. A mismatch means picking again. Comparing inputs means a profile change takes effect without any explicit invalidation step, and it covers edits made from a parent's view or in another tab.
- *Alternative: delete the rows on `PUT /profile`.* Rejected: it couples `internal/family` to widget storage, and it would miss any future path that changes a profile.
- *Alternative: an in-memory map.* Rejected: it's lost on restart, and every restart during the demo would spend credit.

### 2. Described widgets refresh in place
Migration `003` adds `custom_widgets.prompt TEXT NOT NULL DEFAULT ''` and `custom_widgets.refreshed_at DATETIME`. A refresh overwrites `render_spec.items` and sets `refreshed_at`. `updatedAt` is `refreshed_at`, or `created_at` if it was never refreshed. Widgets made before this change have an empty `prompt`, so the refresh shapes results using the widget's title as the prompt. Titles such as "Upcoming SpaceX Launches" carry the intent well enough.
- *Alternative: a separate history table of results.* Rejected: nobody needs history, and it adds joins to every homepage load.

### 3. The server reports staleness, and the client starts background refreshes
Responses gain `updatedAt` and `stale`. The freshness windows are Go constants next to each widget kind. The client shows whatever comes back, and when `stale` is true it starts one forced refresh:
- `GET …?refresh=1` for curated widgets;
- `POST /api/widgets/custom/{id}/refresh` for described widgets.

This works like "stale-while-revalidate", so a page load never waits on a search unless nothing is remembered yet.
- *Alternative: the server refreshes stale results before answering.* Rejected: it brings back the slow page loads this change exists to fix.
- *Alternative: a server-side background worker.* Rejected: it spends credit on pages nobody opens, and it's more moving parts than a hackathon timeline allows.

### 4. Forced refreshes for curated widgets reuse the GET route
`?refresh=1` skips the remembered result. It's the same handler, so the members' version gets it for free. It's a GET that writes to a cache, which is acceptable: it's safe to repeat, and the 1-minute guard makes it cheap to repeat.
- *Alternative: `POST /api/widgets/articles/refresh`.* Rejected: it would need two new routes for each of the `/api/me` and `/api/members/{id}` paths, for the same behavior.

### 5. The 1-minute repeat guard is checked on the server
For a forced refresh, if the current result's `updatedAt` is less than 60 seconds old, return it unchanged. It's checked against the database, so it holds across tabs and devices. For curated widgets, the guard applies only when the inputs still match (a profile change always picks again).

### 6. Failures never overwrite saved results
Only a successful result is written. For described widgets, a search that returns zero results leaves the saved items alone. If the search returns results but `ShapeItems` filters every one out as past (`future_only`), that does count as a successful empty result. If shaping fails, the raw results are mapped, as `FromPrompt` already does.

### 7. Frontend: the cached page is the source of truth for described widgets
A `useRefreshCustomWidget(widgetId)` mutation posts the refresh and writes the returned widget into the cached homepage's `customWidgets` entry (`setQueryData`). `CustomBlock` then re-renders with no extra query. The background refresh is a `useEffect` in `CustomBlock`, guarded so each widget id triggers it only once per stale result, and it runs only on the owner's page (`usePageOwner().isYou`). For curated widgets, the query function takes a `refresh` flag through a small wrapper that fetches with `?refresh=1` and `setQueryData`s the result. A `useEffect` in `useCuratedWidget` calls it once when `data.stale` is true. The "Updated … ago" label comes from a shared `useNow(60_000)` tick and a `formatUpdatedAgo` helper, so labels age without refetching.

## Risks / Trade-offs

- **[Background refreshes on a page with many stale described widgets fire several paid pipelines at once]** → Each one runs at most once per 6-hour window per widget, behind the 1-minute guard. Family pages have a handful of widgets. Deployment's rate limits will cap the rest.
- **[A title-as-prompt refresh of older widgets shapes results slightly differently]** → It's acceptable for widgets made before this change. Every new widget saves its real prompt.
- **[A GET that writes]** → It's documented in the handler. It's safe to repeat and protected by the guard. Browsers and proxies don't cache it, because the frontend fetches with `cache: 'no-store'` and the API sends no cache headers.
- **[Clock skew between the browser and the server makes the label wrong]** → Staleness is decided on the server, and the label is only cosmetic.
- **[A stale result refreshed by a parent's view counts as the member's refresh]** → That's intended. It's one shared result per member, and it saves credit.

## Migration Plan

`003_widget_freshness.sql` runs automatically on startup, like `001` and `002`. It only adds a table and columns that have defaults, so existing databases keep working. Rollback means running the previous binary: the old code ignores the new table and columns.
