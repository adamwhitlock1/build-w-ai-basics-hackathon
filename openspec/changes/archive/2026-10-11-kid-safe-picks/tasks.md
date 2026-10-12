## 1. Search audience (`internal/sources`)

- [x] 1.1 Add the `Audience` type with the `Everyone` and `Kids` constants to `internal/sources/sources.go`, and change `Source.Search` to `Search(ctx, query string, aud Audience)`. Verify with `go build ./internal/sources/`.
- [x] 1.2 In `BraveClient`, set `safesearch=strict` on news and web requests when the audience is `Kids`, and leave it out when it is `Everyone`. Add table cases to `brave_test.go` for news, web and events, each run as `Kids` (`"safesearch": "strict"`) and as `Everyone` (`"safesearch": ""`). Verify with `go test ./internal/sources/`.

## 2. Under-18 guidance (`internal/llm`)

- [x] 2.1 Add a `kidGuidance` constant. Add an `aud sources.Audience` parameter to `CurateArticles`, `CurateEvents`, `ExtractEvents`, `PickWidgetSpec` and `ShapeItems`, and append `kidGuidance` to each one's system prompt when the audience is `Kids`. Verify with `go build ./internal/llm/`.
- [x] 2.2 Update `curate_test.go` and `llm_test.go` callers. For each of the five calls, add a test that reads the `fakeOpenRouter`'s recorded system message and checks that `kidGuidance` is there for `Kids` and missing for `Everyone`. Verify with `go test ./internal/llm/`.

## 3. Handlers pick the audience (`internal/widgets`)

- [x] 3.1 Add `audienceFor(store.User) sources.Audience`, and update the `Curator` and `Planner` interfaces to the new `llm` signatures. Verify with `go vet ./internal/widgets/` after the fakes in 3.4 compile.
- [x] 3.2 In `CuratedHandler.Articles` and `Events`, work out the audience from the subject `member`. Pass it to every curator call and every `src.Search`, including the fallback-query paths (thread it through `search` and `searchAll`). Set `KidSafe` on the response.
- [x] 3.3 In `Handler.FromPrompt`, work out the audience from the logged-in user and pass it to `PickWidgetSpec`, `Search` and `ShapeItems`. Add `KidSafe` to `Widget` (`json:"kidSafe"`) and to `renderSpec` (`json:"kidSafe,omitempty"`). Make sure `save` and `FromStore` round-trip it.
- [x] 3.4 Update `fakeCurator`, `querySource`, `fakePlanner` and `fakeSource` in `curated_test.go` and `handler_test.go` so they record the audience they receive. Add tests:
  - a kid subject: every curator and search call gets `Kids`, and the response has `kidSafe: true`;
  - a parent: every call gets `Everyone`, and the response has `kidSafe: false`;
  - a parent viewing a kid (`auth.WithSubject`): every call gets `Kids`;
  - the fallback queries for a kid still search with `Kids`;
  - `FromPrompt` as a kid saves and returns `kidSafe: true`;
  - `FromStore` on a `render_spec` with no `kidSafe` key returns `false`.

  Verify with `go test ./internal/widgets/`.

## 4. End-to-end backend

- [x] 4.1 Update the fakes in `backend/routes_test.go` to the new signatures. Add an end-to-end test in `members_test.go` using `newTestFamily`: the kid's own `GET /api/widgets/articles` and the parent's `GET /api/members/{kid}/widgets/articles` both return `kidSafe: true`, and the parent's own call returns `kidSafe: false`. Verify with `go test ./...` and `go vet ./...` from `backend/`.

## 5. Frontend marker

- [x] 5.1 Add `kidSafe: boolean` to the widget response types in `frontend/src/api/client.ts`. Verify that `npm run build` (which runs `tsc -b`) passes.
- [x] 5.2 Give `WidgetCard` a `kidSafe` prop, and render a "Kid-safe" pill (in the `RoleTag` style, with a tooltip) only when `kidSafe && useMe().user.isAdmin`. Pass the prop from `CuratedWidgetCard` and `puck/blocks/Custom.tsx`. Verify that `npm run build` and `npm run lint` pass.

## 6. Verify in the running app

- [x] 6.1 Run the app with real keys. Check:
  - as the parent, the kid's page shows "Kid-safe" on Articles, "Happening near you" and a widget the kid described, and the parent's own page shows no marker;
  - as the kid, no marker is shown;
  - a kid widget saved before this change shows no marker.

  Confirm in the backend logs or with a request trace that the kid's Brave calls include `safesearch=strict`.

## 7. Align with fresh, fast widgets (merged to main first)

- [x] 7.1 Merge `origin/main` and resolve the conflicts in `curated.go`, `handler_test.go`, `WidgetCard.tsx` and `Custom.tsx`, keeping both sides. Show the marker in `CustomBlock`'s read-only (family member) branch. Verify that `go vet ./...`, `go test ./...` and `npm run build` pass.
- [x] 7.2 `RefreshCustom` searches and shapes with `audienceFor(user)` and saves `kidSafe`. Verify with `TestRefreshCustomIsKidSafeForKids` (an old kid widget becomes kid-safe; a parent's doesn't).
- [x] 7.3 `sendRemembered` treats a remembered result whose `kidSafe` doesn't match the audience as missing. Verify with `TestRememberedResultsMatchTheAudience` (all four combinations), and by checking that the freshness tests still pass with their rows seeded kid-safe.
- [x] 7.4 Re-run the live check with a kid who has a result remembered before this change. Verify that it's picked again with `safesearch=strict` and comes back `kidSafe: true`.

## 8. Wrap-up

- [x] 8.1 After archiving, add one line about kid-safe picks to "Post-PoC Additions" at the end of `devpost/prd.md` and `devpost/spec.md`. Verify with `git diff devpost/` that only those two lines were added.
