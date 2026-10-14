## 1. Writing the briefing (`internal/llm`)

- [x] 1.1 Add `BriefingFacts`, `BriefingEvent` and `BriefingNearby` from design decision 1, with an unexported `briefingUserMessage(f)` that writes the labelled block and leaves empty sections out. Table tests in `internal/llm` must check:
  - an all-empty section is absent;
  - tomorrow's events appear only when given;
  - the message never contains an `@` or the words "Name:" or "Location:".

  Verify with `go test ./internal/llm/`.
- [x] 1.2 Add `briefingSystemPrompt(now)` and `Client.WriteBriefing(ctx, facts, aud)`, using `callJSON` with the `{"briefing": string}` schema, `forAudience`, and a `check` that trims, rejects blank text or text over 400 characters (wrapping `ErrNoBriefing`), and cuts text between 320 and 400 characters at the last sentence end before 320. Using the existing fake OpenRouter server pattern in `llm_test.go`, test:
  - a good reply is returned;
  - a blank reply is retried once, then gives `ErrNoBriefing`;
  - a 350-character reply is trimmed to a sentence end of 320 characters or fewer;
  - `sources.Kids` adds the kid guidance to the system message;
  - a 402 from OpenRouter gives an `*UpstreamError` with `Status` 402.

  Verify with `go test ./internal/llm/`.

## 2. Briefing logic (`internal/widgets`)

- [x] 2.1 Move the `tz` parsing out of `TodayHandler.location` into a package function `locationFrom(r, logger)` that both handlers use, and add `partOfDay(t)` (`morning` before 12:00, `afternoon` before 18:00, then `evening`). Verify with `go test ./internal/widgets/` (the existing Today tests stay green) and a table test for `partOfDay` at 11:59, 12:00, 17:59 and 18:00.
- [x] 2.2 Add the pure function `shouldWrite` (it takes the remembered briefing for today, if any, with its fingerprint and write time, plus the new fingerprint and `now`) with the constants `maxBriefingWrites = 4` and `briefingRewriteGap = 10 * time.Minute` (design decision 2, step 6). Table-test each rule:
  - nothing from today → write;
  - the same fingerprint → don't;
  - a changed fingerprint with 4 writes → don't;
  - a changed fingerprint 3 minutes after the last write → don't;
  - a changed fingerprint 11 minutes after the last write with 2 writes → write.

  Verify with `go test ./internal/widgets/`.
- [x] 2.3 Add `memberLocks` (design decision 5). Test that two goroutines locking the same member run one at a time, and that different members don't block each other (use channels with a timeout, not sleeps). Verify with `go test -race ./internal/widgets/`.
- [x] 2.4 Implement `gatherFacts` (design decision 3) over the `ResultStore` and a weather helper. Using the fake store from `curated_test.go` and a fake weather source, test:
  - the calendar is used only for a matching date and time zone, with no `emptyReason`;
  - ended timed events leave the prompt facts but not the fingerprint;
  - tomorrow's events are included only from 18:00;
  - articles are used only when `inputs` matches the current interests and `kidSafe` matches the audience (both directions);
  - nearby facts follow the same checks;
  - at most 3 headlines and 3 nearby items are used;
  - `uses` lists only the sections that weren't empty;
  - the fingerprint is stable for the same facts and changes when an event title, the weather snippet or the part of day changes.

  Verify with `go test ./internal/widgets/`.
- [x] 2.5 Implement the weather helper (design decision 4): a remembered `briefing_weather` row is reused under 1 h for the same location and date; otherwise the forecast source is asked with a 6 s timeout and the `"Today ("` result's snippet is kept; a failure falls back to a same-day snippet or no weather; an empty location means no call. Test with a fake source that counts calls:
  - a second call within the hour makes no request;
  - a new location makes one;
  - a failure with a same-day snippet uses it;
  - a failure without one gives no weather;
  - the snippet never includes the place name.

  Verify with `go test ./internal/widgets/`.

## 3. Briefing endpoint

- [x] 3.1 Add `widgets.BriefingHandler` with `NewBriefingHandler(briefer, srcs, profiles, results, logger)` and its `Briefing` method, following design decision 2's steps 1–7, plus the `BriefingWidget` response and the `briefingDay` payload. Wire it in `routes.go` (`GET /api/widgets/briefing` as private, `GET /api/members/{id}/widgets/briefing` behind `asParent`) and in `main.go` (the existing `llmClient` as the `Briefer`). Using a fake `Briefer` that counts calls, add handler tests:
  - no facts → 200 `nothing_yet`, 0 calls, nothing remembered;
  - first visit → 1 call, the text is returned and remembered with `writes: 1`;
  - a second visit with the same facts → 0 more calls;
  - a changed fact 11 minutes later → 1 more call, `writes: 2`;
  - a changed fact with `writes: 4` → 0 calls, the old text returned;
  - a remembered briefing from yesterday → a new call with `writes: 1`;
  - a kid → the `Briefer` gets `sources.Kids` and the response has `kidSafe: true`;
  - a parent's remembered briefing isn't sent after the member becomes a kid;
  - the `Briefer` fails with today's briefing remembered → 200 with the old text;
  - the `Briefer` fails with nothing remembered → 502 with a message and hint;
  - `stale` is always false;
  - the facts handed to the `Briefer` contain no email.

  Verify with `go test ./...` in `backend/`.
- [x] 3.2 Add route tests in `routes_test.go` and `members_test.go`:
  - `GET /api/widgets/briefing` needs a login (401);
  - a parent gets a family member's briefing through `/api/members/{id}/widgets/briefing`;
  - a kid gets 403 on another member's briefing route;
  - two concurrent requests for the same member with no briefing yet make exactly one `Briefer` call.

  Verify with `go test -race ./...` in `backend/`.

## 4. Layout

- [x] 4.1 Add `TypeBriefing` to `internal/layout/data.go`, along with `briefingBlock()`, `defaultID` for `"briefing"`, `Default` as `[Briefing, Today, Articles, LocalEvents, …]`, `heal` (at most once; when missing, put back above Today, after the Today step) and `parseLayout` accepting it once. Tests in `data_test.go` must check:
  - the default order and spans;
  - a layout with Today but no briefing gains the briefing first, with the rest unchanged;
  - a layout with neither gains briefing then Today on top;
  - a duplicate briefing is dropped;
  - a saved layout that moved the briefing lower keeps it there.

  Update any existing test that expected Today first. Verify with `go test ./internal/layout/ ./...` in `backend/`.

## 5. Frontend

- [x] 5.1 In `frontend/src/api/client.ts`, add the `BriefingWidget` type (`text`, `date`, `kidSafe`, `uses`, `emptyReason: '' | 'nothing_yet'`, plus `Freshness`) and `'Briefing'` in the layout block type union. In `api/hooks.ts`, add `useBriefing()` from design decision 7: a key with the local date and a `factsKey` built from the Today, Articles and Events queries' `updatedAt`, read with plain `useQuery` on their existing options; `enabled` once all three have settled; `keepPreviousData`; `staleTime: Infinity`; `retry: false`. Verify with `npm run build` and `npm run lint` in `frontend/`.
- [x] 5.2 Add `frontend/src/widgets/BriefingWidget.tsx` and `puck/blocks/Briefing.tsx`:
  - the greeting from `greetingFor(hour)` and `nameFor(email)`, with the member's email on their page, re-rendering with `useNow(60_000)`;
  - the text in a comfortable reading size;
  - a footer naming the sources from `uses` (e.g. "From your weather, calendar and news") and "Updated … ago";
  - the `useShowKidSafe` marker;
  - "Writing your briefing…" / "Writing their briefing…" while loading;
  - a `MissingNudge` for `nothing_yet` that opens the profile panel at `interests`;
  - a `LoadError` with "Try again" when there's no data;
  - no refresh button.

  Verify with `npm run build` and `npm run lint` in `frontend/`.
- [x] 5.3 Add the `Briefing` component to `puck/config.tsx` (`label: 'Morning briefing'`, span 4, `permissions: { delete: false }`), to the switch in `puck/LayoutView.tsx`, and to `defaultLayout` in `puck/layout.ts` (Briefing first, then Today). Verify with `npm run build` and `npm run lint`, and by checking that the Arrange editor offers no way to remove the card.

## 6. Live check

- [x] 6.1 Run the app against a throwaway database with `make seed`'s parent and kid. Give the parent interests, a location and the Google calendar from the Today card's check. Then check:
  - the parent's page opens with "Good morning, …" (or the right part of the day) above Today, and 1–3 sentences that mention the real forecast, today's events and at least one headline or nearby event, with nothing invented;
  - reloading makes no new AI call (check the backend log for `briefing written` versus `briefing remembered`);
  - removing the calendar and reloading after 10 minutes rewrites the briefing without events;
  - a brand-new member with nothing filled in sees the "Add your interests, location or calendar…" nudge, and no AI call is logged;
  - the kid's page shows a kid-appropriate briefing, and the parent viewing it sees the "Kid-safe" marker and "Good morning, Kid…";
  - an older saved layout gains the card at the top;
  - the card can be moved and resized in Arrange, but not removed.

  Grep the backend log and an OpenRouter request dump (log the user message at debug level while checking, then remove that) for the calendar link, emails and the location text, none of which may appear. Record what you saw in this task's notes.

  **Notes (2026-10-13, 23:03–23:11 Denver):** done on a throwaway database (`backend/porchlight.db.briefing-check`, API on port 8092, web on 5182) with `make seed`'s parent and kid. It was evening, so this exercised the evening path. A temporary log line printed the exact facts sent to the AI; it has been removed.
  - **Parent's page:** interests "space launches, soccer, gardening" and location Denver. "Good evening, Contact.codestuff" sat above Today, with "Writing your briefing…" until Articles and Events loaded. Then 3 sentences came from real Open-Meteo weather, 3 headlines and 3 nearby events, with the footer "From your weather, news, and local events · Updated just now". Every place, time and event traced back to the facts.
  - **Reload:** `briefing remembered`, 1 ms, no AI call.
  - **Rewrite on change:** the remembered briefing was aged 11 minutes in the database and the interests changed. Articles were picked again, the page asked again, and the briefing was rewritten (`writes=2`).
  - **New member:** the kid with nothing filled in, viewed by the parent, showed "Good evening, Contact.codestuff+kid", the Kid-safe marker, and the "Add their interests, location or calendar…" nudge, with no AI call.
  - **Kid-safe briefing:** after the parent set the kid's interests, the kid got a briefing from kid-safe picks (`kid_safe=true`), and the parent saw the marker. The kid's own request then got the same remembered text, with no new call.
  - **Older layout:** a layout saved without the briefing came back from `GET /api/me/homepage` with Briefing first and the rest unchanged.
  - **Arrange:** the card shows "Morning briefing" with widths 1/4–4/4 and no Remove. Resizing to 2/4 saved, and Delete and Backspace left it in place.
  - **Privacy:** no facts payload contained a name, email, location text or calendar link. The weather line names no place.
  - **Fixed during the check:** at 11 pm the briefing was given only today's forecast and described it as "tomorrow's weather" in all 3 briefings. With the user's agreement, evening briefings now get tomorrow's forecast, labelled as tomorrow's, and the spec and design were updated. Re-checked live: "Expect rain tomorrow with a high of 53 and a low of 42", which matches Open-Meteo for Oct 14.
  - **Not checked live:** calendar events in the briefing, and "removing the calendar rewrites it". No calendar with events today or tomorrow could be connected without the user's private link. Unit and handler tests cover both: `TestGatherFactsCalendar`, `TestBriefingRewrittenWhenFactsChange`.

  **Notes (2026-10-14, about 14:00 Denver):** the two items above were checked live with the demo family from the `demo-family-seed` change (`make demo`, `make demo-serve`). It has fixture calendars, which the server builds itself, instead of a private Google link.
  - **Calendar events in the briefing:** Maya's first briefing said "You have a dentist appointment at 3:23 this afternoon… Recycling day is all day…", which matched the Today card. `uses` was weather, calendar, articles, nearby, and the log said `briefing written … uses=weather,calendar,articles,nearby writes=1`. A reload was `briefing remembered`.
  - **Removing the calendar rewrites it:** after `DELETE /api/me/calendar`, Today read `no_calendar`. Asking at once returned the remembered briefing, inside the 10-minute gap. With the remembered briefing aged 11 minutes in the database, the next request rewrote it without events: "Expect a rainy day with a high near 53°F… night sky viewing at Chamberlin Observatory… bike-lane plan". `uses` was weather, articles, nearby, and the log said `writes=2`.

## 7. Docs and wrap-up

- [x] 7.1 Add a short "Morning briefing" note to `README.md`: what it uses, that it costs at most 4 AI calls per member per day and no searches, and that it waits for the other cards to load. Verify by reading the rendered README.
- [x] 7.2 Mark roadmap item 5 done in `openspec/roadmap.md`. After archiving, add one line about the morning briefing to "Post-PoC Additions" at the end of `devpost/prd.md` and `devpost/spec.md`. Verify with `git diff devpost/` that only those two lines were added.
