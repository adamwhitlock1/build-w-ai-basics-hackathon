## Context

See proposal.md for the motivation. The required behavior is in `specs/morning-briefing/spec.md`, with small changes to `specs/today-calendar/spec.md` and `specs/widget-freshness/spec.md`.

What the code does today:
- **Remembered results.** Articles, Local Events and Today remember their latest result in `curated_widget_results` (`user_id`, `kind`, `inputs`, `payload`, `updated_at`; primary key `(user_id, kind)`). This goes through `store.GetCuratedResult` and `PutCuratedResult`.
  - `kind = "articles"`: the payload is a `widgets.Widget` (`items`, `kidSafe`, …), and `inputs` is `normalizeInput(interests)`.
  - `kind = "events"`: the payload is the same kind of `Widget`, and `inputs` is `normalizeInput(interests) + "\n" + normalizeInput(location)`.
  - `kind = "today"`: the payload is a `todayDay` (`date`, `timeZone`, `emptyReason`, `today`, `tomorrow`), and `inputs` is `linkHash + "\n" + tz + "\n" + date`.
- **Which member.** Handlers act on `auth.SubjectFrom`. The same handler serves `/api/widgets/...` and the parent-only `/api/members/{id}/widgets/...` behind `asParent`. `audienceFor(member)` gives `sources.Kids` for non-parents.
- **The AI.** `llm.Client` has the generic `callJSON[T]` (structured output, one retry with the bad reply shown) and `forAudience(system, aud)` for the kid guidance. Prompts already tell the model to treat member text as data. Failures arrive as `*llm.UpstreamError`.
- **Weather.** The forecast source `sources.OpenMeteoSource` (`weather_forecast`, registered in `main.go`) takes a place as its query. It returns a "Right now in X" result and then one result per day. The first day's title is `"Today (Mon, Oct 13)"`, with a snippet like `"Sunny. High 75°F, low 58°F. 10% chance of rain."`. It's free and keyless.
- **Layout.** `internal/layout/data.go` has `TypeToday`, `TypeArticles`, `TypeLocalEvents` and `TypeCustom`. `Default` puts Today (span 4) first, and `heal` re-adds a missing Today at the top. The frontend mirrors this in `puck/layout.ts` (`defaultLayout`), `puck/config.tsx` (`permissions: { delete: false }`) and `puck/LayoutView.tsx`.
- **Frontend data.** `curatedWidgetQueryOptions(kind, owner)` and `todayQueryOptions(owner, date)` in `api/hooks.ts` are TanStack Query options with `staleTime: Infinity`. Background refresh lives in the wrapping hooks (`useCuratedWidget`, `useToday`), not in the options. `useShowKidSafe(widget)`, `usePageOwner()` and `nameFor(email)` (in `family/people.ts`) already exist. `WidgetCard` accepts `children`, `notice`, `loading`, `updatedAt`, `showKidSafe` and `headerAction`.
- **Names.** Users have no name column. The page shows `nameFor(email)`.

## Goals / Non-Goals

**Goals:**
- One server endpoint that decides cheaply, with no AI call, whether the remembered briefing still fits, and writes a new one only when the facts changed and the cap allows.
- Reuse what exists: the remembered-results table, `callJSON`, `forAudience`, the forecast source, the permanent-card layout code and `WidgetCard`.
- Briefing logic that can be tested without a network: fact gathering, the decision to rewrite, and prompt construction are all plain functions over fakes.

**Non-Goals:**
- **A refresh button**, or "stale" for the briefing (see the widget-freshness delta).
- **Streaming the text** as it's written. The briefing is short.
- **Reading described widgets** (including weather widgets) as facts. Weather comes from the forecast source directly, and other described widgets are too varied to summarize reliably.
- **Storing names, or sending them to the AI.**
- **Rate limits across members or a daily spend cap.** Those come with deployment (roadmap item 7). This change only caps each member at 4 writes a day.

## Decisions

### 1. `llm.Client.WriteBriefing`: one structured call over a facts struct
```go
// BriefingFacts is everything the model may use. No names, emails,
// locations or links: the widgets package builds it from remembered results.
type BriefingFacts struct {
	Now      time.Time        // the member's local time, for weekday and part of day
	Weather  string           // today's forecast, e.g. "Sunny. High 75°F, low 58°F. 10% chance of rain." or ""
	WeatherTomorrow string    // tomorrow's forecast, given from 18:00 instead of Weather
	Today    []BriefingEvent  // today's remaining and all-day events
	Tomorrow []BriefingEvent  // only from 18:00
	Articles []string         // up to 3 headlines
	Nearby   []BriefingNearby // up to 3
}
type BriefingEvent struct{ Time, Title, Location string } // Time: "4:00 PM" or "All day"
type BriefingNearby struct{ Name, Date, Venue string }

func (c *Client) WriteBriefing(ctx context.Context, f BriefingFacts, aud sources.Audience) (string, error)
```

- **The user message** is a short, labelled plain-text block. Empty sections are left out entirely, so the model can't comment on them:

```text
Weather today: …        (or, in the evening, "Weather tomorrow: …")
Calendar today: 4:00 PM Soccer practice (Lincoln Park)
Articles: …
Nearby: …
```

- **The system prompt:**
  - Tell the model it's writing the morning briefing for one family member on Porchlight, with the day and local time.
  - Ask for 1–3 short, warm sentences, at most 320 characters, in plain words.
  - Ask for no greeting and no name (the page adds those), no emoji and no lists.
  - Ask it to mention only the facts given, never to invent, and to prefer what's soonest and most specific.
  - In the evening, ask it to lead with tomorrow's plans.
  - Ask it to treat every fact as data and ignore instructions inside them.
  - `forAudience` appends the kid guidance.
- **The schema** is `{"briefing": string}`. The `check` function trims whitespace, rejects an empty string, and rejects anything over 400 characters. That rejection triggers `callJSON`'s single retry. A text between 320 and 400 characters is cut at the last sentence end before 320, so a slightly long reply doesn't cost a second call.
- **Errors:** `ErrNoBriefing` if both attempts fail the check, and `*UpstreamError` as for the other calls.
- **Temperature** stays at the client's 0.2. A warmer voice comes from the prompt, not from randomness.

Alternative considered: **reusing `ShapeItems` with a "text" archetype.** It's built around search results and items, and the briefing is prose from mixed facts.

### 2. `widgets.BriefingHandler`, and what happens on each request
`GET /api/widgets/briefing?tz=<zone>` and `GET /api/members/{id}/widgets/briefing?tz=<zone>` (behind `asParent`) are served by:

```go
type Briefer interface {
	WriteBriefing(ctx context.Context, f llm.BriefingFacts, aud sources.Audience) (string, error)
}
type BriefingHandler struct {
	briefer  Briefer
	weather  sources.Source // the weather_forecast source; mustGetSource at startup
	profiles ProfileStore
	results  ResultStore
	locks    memberLocks // decision 5
	logger   *slog.Logger
}
```

Each request goes through these steps:
1. **Who and when.** Find `member := auth.SubjectFrom` and `aud := audienceFor(member)`. Read `loc` from `tz`, sharing the parsing logic with `TodayHandler.location` by moving it to a package function `locationFrom(r, logger)`. Then work out `now`, `date` and `part := partOfDay(now.In(loc))`, which is `morning`, `afternoon` or `evening`.
2. **Lock this member** (decision 5). Everything below runs under the lock.
3. **Gather facts** with `gatherFacts` (decision 3). It returns a `llm.BriefingFacts` for the prompt and a `fingerprint` string covering what the briefing depends on.
4. **No facts at all:** respond 200 with `emptyReason: "nothing_yet"` and an empty `text`. Make no AI call and remember nothing.
5. **Load the remembered briefing** (`kind = "briefing"`). It counts as *today's* only if its `date` and `timeZone` match and its `kidSafe` equals `aud == Kids`. Anything else counts as missing, including a briefing from yesterday or from before the member became a parent.
6. **Decide** with `shouldWrite(remembered, fingerprint, now)`, a pure function:
   - nothing from today → write;
   - the fingerprint matches → send the remembered one;
   - `writes >= maxBriefingWrites` (4) → send the remembered one;
   - written less than `briefingRewriteGap` (10 min) ago → send the remembered one;
   - otherwise → write.
7. **Write.** Call `WriteBriefing` with a budget of `briefingBudget = 25 * time.Second`. That's inside `RequestBudget`, and leaves `callJSON` room for one retry on a fast model.
   - **On success:** remember `{text, date, timeZone, kidSafe, writes: previous+1 (or 1 on a new day), uses, fingerprint}` with `updated_at = now`, then respond.
   - **On failure with today's briefing remembered:** log a warning and send the remembered one. The spec says "keeps showing".
   - **On failure with nothing from today:** respond 502 with "Couldn't write your briefing right now." and hint "Try again in a moment." (or the `UpstreamError` hint for an OpenRouter refusal, as in `CuratedHandler.writeLLMError`). Use `writeIfCancelled` first.

What gets remembered is `kind = "briefing"`, with `inputs = fingerprint` and this payload:

```go
type briefingDay struct {
	Text        string    `json:"text"`
	Date        string    `json:"date"`
	TimeZone    string    `json:"timeZone"`
	KidSafe     bool      `json:"kidSafe"`
	Uses        []string  `json:"uses"`   // subset of "weather","calendar","articles","nearby"
	Writes      int       `json:"writes"` // AI writes today, including this one
}
```

The response:

```go
type BriefingWidget struct {
	Freshness                 // updatedAt = when written; stale always false
	EmptyReason EmptyReason `json:"emptyReason"` // "" or "nothing_yet"
	Text        string      `json:"text"`
	Date        string      `json:"date"`
	KidSafe     bool        `json:"kidSafe"`
	Uses        []string    `json:"uses"` // never null; for the footer "From your weather, calendar and news"
}
```

- `writes` and the fingerprint stay on the server.
- `kidSafe` is reported even for `nothing_yet`, for consistency with the curated cards.
- Using `inputs` for the fingerprint means a mismatch reads exactly like the curated cards' "profile changed" check.

Go concept: `shouldWrite` takes the remembered value and `now`, and returns a decision. Keeping time and storage out of it makes every rule in "When the briefing is written" a one-line table test.

### 3. Gathering facts, and the fingerprint
`gatherFacts(ctx, member, profile, aud, loc, now)` reads four sources, each optional:

- **Calendar:** `GetCuratedResult(member, "today")`, decoded as `todayDay`. It's used only if `date` and `timeZone` match today's and `emptyReason == ""`.
  - Facts for the prompt are all-day events plus timed events that haven't ended by `now`. From 18:00 they also include tomorrow's events.
  - Times are formatted in `loc` as `"3:04 PM"`.
  - The fingerprint uses the *full* today and tomorrow lists (title, start, end, all-day), not the remaining ones. Otherwise every event that ends would trigger a rewrite. The part of the day is in the fingerprint, which brings the evening view in.
  - The lists stay capped at the Today card's 12 per day.
- **Articles:** `GetCuratedResult(member, "articles")`, decoded as `Widget`. It's used only if `inputs == normalizeInput(profile.Interests)` and `widget.KidSafe == (aud == Kids)`, the same check `CuratedHandler.sendRemembered` makes. Take the first 3 item titles, each shortened to 120 characters with `shorten`.
- **Nearby:** the same for `"events"`, with the events inputs check. Take the first 3 as `{Name: Title, Date, Venue: Label}`.
- **Weather** (decision 4).

The fingerprint is the SHA-256 (hex, first 16 bytes) of a canonical JSON encoding of `{part, weather, todayAll, tomorrowAll, articles, nearby}`. A `json.Marshal` of a struct is deterministic, so the same facts always give the same fingerprint.

`Uses` lists the sections that weren't empty.

Alternative considered: **having the briefing endpoint fetch the calendar or pick articles itself when they aren't remembered.** It would duplicate `TodayHandler` and `CuratedHandler`, and picking articles costs paid searches. Instead, the page asks for the briefing after those cards have loaded (decision 7), so their results are normally already remembered.

### 4. Weather: the forecast source, remembered for an hour
- `weatherFact(ctx, member, location)` reads `GetCuratedResult(member, "briefing_weather")`. It reuses that result if `inputs == normalizeInput(location) + "\n" + date` and the result is under `WeatherFreshFor` (1 h) old.
- Otherwise it calls `h.weather.Search(ctx, location, aud)` with a 6 s timeout and picks the result whose title starts with `"Today ("` and the day after it. Both snippets are remembered with `updated_at = now`.
- Before 18:00 the briefing gets today's snippet. From 18:00 it gets tomorrow's instead, labelled as tomorrow's. The live check found that an evening briefing given only today's forecast described it as tomorrow's.
- On failure (unknown place, timeout, service error), it reuses a same-day remembered snippet if one exists. Otherwise there's no weather, and a warning is logged.
- An empty location means no weather and no call.
- The snippet holds conditions, temperatures and rain chance. It has no place name, so the location text never reaches the AI. The "Right now" result is ignored, because current temperature changes hourly and would churn the fingerprint.

Go concept: an interface value (`sources.Source`) lets tests hand the handler a fake forecast without a network, exactly as the curated tests fake Brave.

Alternatives considered:
- **Reading a weather widget's saved items:** many members have none, and the items are worded by the AI.
- **Calling Open-Meteo on every request:** it's free, but slow and impolite. The hour-long memory also keeps the fingerprint steady.

### 5. One write at a time per member
Two tabs, or a parent and the kid at once, could both see "nothing from today" and pay twice. `memberLocks` is a small keyed mutex:

```go
type memberLocks struct {
	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}
func (l *memberLocks) lock(id int64) (unlock func())
```

- The second request waits for the first, then re-reads the remembered briefing and finds it fresh.
- Entries are never deleted. There's one small mutex per member who has ever loaded a briefing, which is fine for a family app on one server.
- The lock is held during the AI call. That's acceptable because only that member's briefing requests wait on it.

Go concept: `sync.Mutex` is a lock that one goroutine holds at a time. The map's own mutex only guards finding or creating a member's lock, so different members never wait on each other.

Alternative considered: **`golang.org/x/sync/singleflight`.** It's a new dependency for 15 lines of code, and singleflight shares one result, while the spec also needs the waiting request to re-check the cap.

### 6. Layout: a fourth permanent card, first
- **Constants:** `TypeBriefing = "Briefing"`, default id `"briefing"`, span 4. `briefingBlock()` builds it, like `todayBlock()`.
- **`Default`** becomes `[Briefing(4), Today(4), Articles(2), LocalEvents(2), …widgets]`.
- **`heal`:**
  - `TypeBriefing` joins the at-most-once set.
  - If it's missing, it's prepended *after* the Today step, so it ends up above Today.
  - The order is: put back Today at the front if missing, then put back the briefing at the front if missing.
  - A page that already had Today keeps its arrangement and gains the briefing on top. That matches both deltas.
- **`parseLayout`** accepts `Briefing` once, like the others.
- **Frontend:**
  - the `LayoutBlock` type union gains `'Briefing'`;
  - `defaultLayout` gains the card;
  - `config.tsx` gains a `Briefing` component (`label: 'Morning briefing'`, `defaultProps: { span: 4 }`, `permissions: { delete: false }`) rendering `BriefingBlock`;
  - `LayoutView`'s switch gains the case.

### 7. Frontend: `useBriefing` and `BriefingWidget`
**`useBriefing()` in `api/hooks.ts`:**
- **Query key:** `['widgets', 'briefing', userId, date, factsKey]` for your page, or `['members', memberId, 'widgets', 'briefing', date, factsKey]`.
  - `date` is the local date (the existing `localDate(useNow())`).
  - `factsKey` joins the `updatedAt` values of the page's Today, Articles and Events queries.
- **Reading those queries.** They're read with `useQuery(todayQueryOptions(...))` and `useQuery(curatedWidgetQueryOptions(...))`, using the same options and therefore the same cache entries, but *not* the `useToday`, `useArticles` or `useEvents` wrappers. That way their background-refresh effects don't run twice.
- **`enabled`** only once each of those three queries has settled (`!isPending`, success or error), so the first briefing of the day is written after the cards have remembered their results.
- **`queryFn`** fetches `GET …/briefing?tz=…` with `cache: 'no-store'`.
- **Options:** `staleTime: Infinity`, `retry: false`, `refetchOnWindowFocus: false`, `placeholderData: keepPreviousData`. The last one keeps the earlier briefing on screen while a new factsKey loads.
- **Effect:** when a card refreshes and its `updatedAt` changes, the key changes, the briefing is asked for again, and the server decides whether to rewrite.

**`widgets/BriefingWidget.tsx`:**
- It uses the `WidgetCard` shell with `children`.
- **Title:** the greeting `"Good morning, Maya"` from `greetingFor(hour)` and `nameFor(owner email)`. The owner email comes from `useMe()` for your page, or from the viewed member's record (`useFamilyMember`) on theirs.
- **Body:** the text, in a larger, comfortable reading size.
- **Footer:** `"From your weather, calendar and news"`, built from `uses`, plus "Updated … ago".
- `showKidSafe={useShowKidSafe(data)}`.
- **States:**
  - loading → `loadingMessage` "Writing your briefing…" or "Writing their briefing…";
  - `nothing_yet` → `MissingNudge` with "Add your interests, location or calendar, and your morning briefing will appear here" and a button to the `interests` field;
  - error with no data → `LoadError` with noun "briefing" and "Try again" (`refetch`);
  - error with placeholder data → keep the text and show nothing extra (the server already falls back to today's briefing, so this is rare).
- **No `headerAction`** refresh button.
- **The greeting re-renders with `useNow(60_000)`**, so it changes from morning to afternoon without a request.

## Risks / Trade-offs

- **[The first briefing of the day is thin because the cards haven't loaded]** → The page waits for the Today, Articles and Events queries to settle before asking. A card that fails or loads late changes `factsKey`, which triggers a re-check, and the server rewrites within the cap.
- **[Fingerprint churn burns the cap early]** (for example, Articles refreshing every 3 h with new headlines) → There are 10 minutes between writes and 4 a day. The fingerprint ignores current temperature and events that have ended. Four writes is one per part of day plus one spare. If churn shows up in testing, drop headlines from the fingerprint and keep them only as prompt facts.
- **[The model invents details or ignores the length limit]** → Structured output, a length check with one retry, a sentence-boundary trim, low temperature, and a prompt that lists only present sections. A table-driven test checks the prompt has no empty sections and no name or email. The live check reads real briefings for a parent and a kid.
- **[Prompt injection through calendar titles or headlines]** → All facts are labelled data, and the system prompt says to ignore instructions inside them. The output is plain text, rendered by React as text and never as HTML, and it's capped at 320 characters, so the worst case is an odd sentence on one person's card.
- **[A kid's briefing built from content picked for a parent]** → Articles and nearby facts must match the member's kid-safe mode, a remembered briefing in the other mode is ignored, and `forAudience` is always applied. There are tests for both modes.
- **[The lock is held during a slow AI call]** → Only the same member's briefing requests wait. They're bounded by the 25 s budget, and the page shows the loading state.
- **[A parent's view spends the kid's cap]** → This is intended (one shared briefing per member), and stated in the spec.
- **[Time zone differs between viewers]** (a parent travelling) → It's the viewer's day, as for the Today card. A different `timeZone` means "not today's" and causes one extra write.

## Migration Plan

- There's no database migration. New rows use kinds `briefing` and `briefing_weather` in `curated_widget_results`.
- Layouts saved before this change gain the briefing card when they're next loaded, through `heal`. No data is rewritten.
- Rolling back leaves harmless `briefing` and `briefing_weather` rows. An old frontend skips unknown `Briefing` blocks (the `default: return null` case in `LayoutView`), and an old backend's `heal` drops them.
- Deployment (roadmap item 7) should include `/api/widgets/briefing` in its AI rate limits, and seed the demo family with interests, a location and a calendar so the briefing has facts on first load.
