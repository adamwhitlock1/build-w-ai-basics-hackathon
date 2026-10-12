## Why

Kids in a Porchlight family get the same search results as their parents. Brave runs at its default safesearch level and the LLM is never told the member is a child. Parents need to trust what their kids see before they hand them a homepage. This strengthens the Potential Impact story, and it reuses the parent/kid role that already exists.

## What Changes

- Content made for a kid (any member who isn't a parent) is kid-safe:
  - every Brave search runs with strict safesearch;
  - every LLM step that picks searches, picks events or shapes items is told the member is under 18 and must leave out unsuitable content.
- Kid-safety is decided by the member whose page the content is for, not by who is looking. So a parent viewing their kid's page sees the kid's kid-safe results.
- This applies to:
  - the curated cards (Articles, "Happening near you");
  - widgets a kid describes ("describe a widget").
- Widget responses say whether the content was made kid-safe. Saved described widgets remember it.
- Parents see a small "Kid-safe" marker on those cards. Kids don't see the marker.
- Nothing changes for parents' own content.

## Capabilities

### New Capabilities
- `kid-safe-content`: how content is filtered for kid members (search safety level, LLM age guidance), whose role decides it, and how parents can see that a card is kid-safe.

### Modified Capabilities
None. `openspec/specs/` is empty, because the PoC was specified in `devpost/` rather than in OpenSpec.

## Impact

- **Backend (Go):**
  - `internal/sources`: the `Source.Search` signature gains an audience argument, and Brave adds `safesearch=strict` for kids.
  - `internal/llm`: the curate, extract, spec and shape calls take an audience and add the kid guidance to their system prompts.
  - `internal/widgets`: the curated and from-prompt handlers work out the audience; the `Widget` JSON gains `kidSafe`.
  - All the test fakes that implement these interfaces need updating.
- **API:** `kidSafe` (boolean) is added to the curated widget responses and the saved widget responses. The change is additive and nothing breaks.
- **Data:** `kidSafe` is stored inside the existing `custom_widgets.render_spec` JSON, so no migration is needed. Widgets saved before this change read as not kid-safe.
- **Frontend:** `WidgetCard` shows a "Kid-safe" pill to parents. The `Widget` types in `api/client.ts` gain `kidSafe`.
- **Docs:** when the change is archived, add one line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
- **Fresh, fast widgets** (merged first): remembered curated results are reused only in the mode they were made in, and refreshing a described widget is kid-safe for kids (see design.md, decision 7).
