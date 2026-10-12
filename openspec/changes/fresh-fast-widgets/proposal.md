## Why

Porchlight promises that "every widget is alive", but today a widget you describe is a snapshot frozen at the moment it was built. Its refresh never worked, and the empty-state copy tells people to describe it again instead. Meanwhile the Articles and Local Events cards do the opposite: they re-run up to 8 Brave searches plus several LLM calls on every page load. Pages are slow, credit runs down, and every family member pays that cost each visit.

Both problems have the same fix, which is to remember results and refresh them when they're old. It has to land first: the demo video needs a page that loads instantly, and the planned public deployment needs searches to stop running on every visit.

## What Changes

- **Remembered results for Articles and Local Events.** Each member's latest picks are saved. Opening the page shows them instantly instead of searching again. They're thrown away automatically when that member's interests or location change.
- **Each widget kind has a freshness window.** Saved results older than the window (Articles 3 hours, Local Events 12 hours, described widgets 6 hours) still show at once, and then refresh quietly in the background. The new items replace the old ones when they arrive. Only a widget that has never loaded makes you wait.
- **Described widgets can refresh.** A new refresh button, plus the automatic background refresh, re-runs the widget's saved search and re-shapes the results. It keeps the widget's title and display style, so the card stays recognizably the same widget. New widgets also remember the sentence they were described with, so a refresh shapes results the same way the original did.
- **"Updated 2h ago" on every card**, so people can tell how fresh a card is.
- **A failed refresh keeps the old items** with a small "Couldn't load fresh …" notice and "Try again". A refresh whose search finds nothing at all also keeps the old items.
- **Refreshing a widget twice within a minute doesn't search again.** The second request gets the result that's already there. This protects credit from double clicks and multiple tabs, and it's groundwork for deployment's rate limits.
- The refresh button on Articles and Local Events always gets new picks, even when the saved ones are still fresh.

## Capabilities

### New Capabilities
- `widget-freshness`: how every widget on a page (Articles, Local Events, and described widgets) is saved, shown instantly, judged fresh or stale, refreshed automatically or on request, and labeled with when it was last updated. Covers failures and the protection against repeated refreshes.

### Modified Capabilities
<!-- None. openspec/specs/ has no specs yet: the PoC's behavior is described in devpost/prd.md. -->

## Impact

- **Database:** migration `003`.
  - A new table for each member's saved Articles and Local Events results, with the interests and location they were picked for.
  - `custom_widgets` gains `prompt` (empty for widgets made before this change) and `refreshed_at`.
- **Backend (Go):**
  - `internal/store`: read and write saved curated results; update a described widget's items.
  - `internal/widgets`: Articles and Local Events check the saved results first, and accept `?refresh=1`. FromPrompt saves the prompt. There's a new `POST /api/widgets/custom/{id}/refresh`.
  - `internal/layout`: the homepage's `customWidgets` carry `updatedAt` and `stale`.
- **API responses:** curated and saved widgets gain `updatedAt` (RFC 3339) and `stale` (boolean). Existing fields don't change; nothing is **BREAKING**.
- **Frontend (React):**
  - The curated-widget hooks show saved results and refresh stale ones in the background.
  - There's a new refresh mutation for described widgets.
  - `CustomBlock` gets a refresh button, background refresh and failure notice, but only on your own page.
  - `WidgetCard` shows "Updated … ago", and its empty-state copy changes now that refresh works.
- **Cost:** a page visit within the freshness window spends no searches or LLM calls. A stale card spends what a refresh costs today, at most once per window.
- **Docs:** when the change is archived, add a line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`. Update the README's note that "nothing is cached".
