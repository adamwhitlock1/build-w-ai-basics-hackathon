## Context

See proposal.md for the motivation and specs/kid-safe-content/spec.md for the required behavior.

What the code does today:
- **There is no Kid role as such.** A kid is a `store.User` with `IsAdmin == false` (`backend/migrations/001_init.sql:10`). There is no age field.
- **Brave:** all searches go through `sources.Source.Search(ctx, query)` (`internal/sources/sources.go:19`), which `BraveClient` implements for `news_search`, `web_search` and `events_search` (`internal/sources/brave.go`). The `safesearch` parameter is never set.
- **LLM:** every call builds a system prompt in Go (`internal/llm/curate.go`, `events.go`, `spec.go`, `items.go`) and receives only interests, location and prompt strings. It never learns who the member is.
- **Who the content is for:**
  - Curated handlers already load the subject member, using `auth.SubjectFrom` in `CuratedHandler.loadProfile` (`internal/widgets/curated.go:276`). That is the page owner, even when a parent is viewing.
  - `Handler.FromPrompt` (`internal/widgets/handler.go:150`) only ever builds widgets for the logged-in user, so there the user is the owner.
- **Saved widgets** keep their content in `custom_widgets.render_spec` (JSON), which `renderSpec` (`handler.go:113`) decodes.
- **Frontend:** `WidgetCard` (`frontend/src/widgets/WidgetCard.tsx`) draws every card. The viewer's role comes from `useMe().user.isAdmin`.

## Goals / Non-Goals

**Goals:**
- One explicit value travels from the handler down to every Brave and LLM call, saying who the content is for.
- Kid-safety is shown to parents from the server's own record (`kidSafe`), not guessed by the frontend.
- No schema migration.

**Non-Goals:**
- **Age tiers:** there is no age field. One "under 18" standard covers the ~12- and ~15-year-olds in scope.
- **Re-filtering old saved widgets right away:** they keep their content and read as `kidSafe: false` until they're refreshed (see decision 7).
- **A parent-controlled setting to turn kid-safe mode off.**
- **Moderating what a kid types into "describe a widget"** beyond the AI guidance and strict search.
- **Describing a widget on someone else's page:** there is no route for it today.

## Decisions

### 1. A named `Audience` type, passed explicitly
Add this to `internal/sources`:

```go
// Audience says who search results are for.
type Audience int

const (
	Everyone Audience = iota // a parent: no extra filtering
	Kids                     // a kid: strict safe-search and age guidance
)
```

Add `audienceFor(u store.User) sources.Audience` (it returns `Kids` unless `u.IsAdmin`) to `internal/widgets`. That package already imports both `store` and `sources`. Putting it there keeps `sources` free of any dependency on `store`, as it is today.

`Source.Search` becomes `Search(ctx, query string, aud Audience)`. The `llm` functions behind `widgets.Curator` and `widgets.Planner` gain an `aud sources.Audience` parameter too. `llm` already imports `sources`, so this adds no new dependency edges.

Go concept: `iota` numbers the constants 0, 1, …, and the named type stops a stray `int` or `bool` from being passed by mistake. The call site reads `Search(ctx, q, sources.Kids)` instead of `Search(ctx, q, true)`. `Everyone` is the zero value, so any `Audience` left unset means "no filtering". Tests must therefore check that the kid paths really pass `Kids`.

Alternatives considered:
- **A context value** (`sources.WithKids(ctx)`). Rejected: it hides a parameter that changes behavior, and Go's guidance is to keep context values for request-scoped plumbing like deadlines and auth, not for options. If someone forgets to set it, content silently falls back to adult results, which is the failure we must avoid.
- **A `kidSafe bool` parameter.** Rejected: a bare bool reads badly at call sites, and a named type leaves room for an age tier later.
- **Wrapping the Brave client in a `KidSafeSource` decorator.** Rejected: it fixes search but not the LLM prompts, so the handler would still need to know the audience.

### 2. Brave: `safesearch=strict` for `Kids` only
`BraveClient` sets `safesearch=strict` on both `/news/search` and `/web/search` when the audience is `Kids`. When the audience is `Everyone`, it leaves the parameter out, so parents keep exactly today's behavior and the existing `brave_test.go` expectations still hold.

### 3. LLM: one shared guidance block appended to the system prompt
Add one `kidGuidance` constant in `internal/llm`. Each prompt builder (`articlesSystemPrompt`, `eventsSystemPrompt`, `extractEventsSystemPrompt`, `specSystemPrompt`, `shapeSystemPrompt`) appends it when the audience is `Kids`. The block:
- says the member is under 18;
- lists what to leave out (the categories in the spec);
- tells the model to steer queries toward age-appropriate angles;
- tells the model to drop an item when it is unsure.

One constant keeps the wording consistent and testable, since a test can assert it's present or absent. The guidance goes in the system message rather than the user message, because the user message carries member-typed text that the prompts already treat as data.

### 4. The page owner sets the audience
- **Curated handlers:** `audienceFor(member)`, the subject from `loadProfile`, not the viewer.
- **`FromPrompt`:** `audienceFor(user)`.

Each handler works out the audience once and passes the same value to every search and LLM call it makes, including the fallback search paths.

### 5. `kidSafe` lives on `Widget` and inside `render_spec`
- **`Widget`:** add `KidSafe bool \`json:"kidSafe"\``. `Widget` is embedded in both `CuratedWidget` and `SavedWidget`, so both responses get the field.
- **`renderSpec`:** add `KidSafe bool \`json:"kidSafe,omitempty"\``. Old rows don't have the key, so they decode as `false`, which is the behavior the spec requires.

This needs no migration and no store change. A new `custom_widgets.kid_safe` column was considered and rejected: it is more work for the same result, and the flag describes the content, which is exactly what `render_spec` holds.

### 6. The marker is shown when `widget.kidSafe && me.isAdmin`
`WidgetCard` is purely presentational and owns no data fetching, so it doesn't read `useMe()` itself. Instead:
- a `useShowKidSafe(widget)` hook in `api/hooks.ts` makes the decision;
- its two callers, `CuratedWidgetCard` and `puck/blocks/Custom.tsx`, call the hook and pass the result to `WidgetCard`'s `showKidSafe` prop;
- `WidgetCard` then draws a "Kid-safe" pill with a shield icon beside the heading. It uses the kid `RoleTag` style and has the tooltip "Searched with strict safe-search and chosen for under-18s". Kids never see the marker, because a teen would find it patronizing and it tells them nothing they can act on.

### 7. Reuse remembered results only in the mode they were made in
Fresh, fast widgets (merged first) remembers one curated result per member and card, and reuses it while the profile inputs match. `sendRemembered` now also requires the remembered widget's `kidSafe` to match the member's audience. Without that, a kid would be sent a result remembered before this change.
- *Alternative: add the audience to the cache's `inputs` key.* Rejected: on deploy it would change every key, including parents', and every family would pay for a fresh round of searches. Comparing `kidSafe` invalidates only the rows that are actually wrong. The flag is already part of the remembered payload, because the whole `Widget` is stored.

`RefreshCustom` searches and shapes with `audienceFor(user)`. Only owners refresh their own widgets, so the owner is the right audience. It also writes `kidSafe` into the new render spec, so an old kid widget becomes kid-safe at its first refresh, and stale widgets refresh in the background.

## Risks / Trade-offs

- **[The LLM misses something]** → Strict safe-search is the main filter and the AI guidance is a second layer. That's defense in depth, not a guarantee. The product copy says "kid-safe", not "guaranteed safe".
- **[Fallback paths skip the AI filter]** → Raw-result fallbacks (`itemsFromResults`, `Fallback*Queries`) still use strict search. The other option, an error card for kids, was rejected because it would make kids' pages break more often than parents'.
- **[Kid-typed prompts]** → A kid could describe a widget for unsuitable content. Strict search plus the guidance should produce safe or empty results. The existing defense against prompt injection, member text treated as data, stays as it is (`prd.md` explicitly declines adversarial hardening).
- **[Old kid results in the cache]** → Kids' remembered results from before this change were made without filtering. Decision 7 picks them again on the kid's next visit.
- **[Interface churn]** → Changing `Source.Search` and the `Curator`/`Planner` signatures touches every fake (`curated_test.go`, `handler_test.go`, `routes_test.go`). It's mechanical, and the compiler finds every one.

## Migration Plan

There's no data migration. Deploying is just shipping the new build. To roll back, revert the commit. Saved widgets that contain `"kidSafe": true` decode fine on the old code, because unknown JSON keys are ignored.
