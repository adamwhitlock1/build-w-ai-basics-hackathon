# Porchlight Post-PoC Roadmap

The proof of concept is done (tag `poc-complete`). This roadmap orders the features added after it. Each item becomes one OpenSpec change (`/opsx:propose` → `/opsx:apply` → `/opsx:archive`). When a change is archived, add a line to "Post-PoC Additions" at the end of `devpost/prd.md` and `devpost/spec.md`.

**Deadline:** Devpost submission closes **2026-10-26, 5:00 pm ET**. Feature freeze is **2026-10-23**. October 24–26 go to `6-ship` (demo video, public repo, write-up), with a day of buffer. Aim to submit by October 25.

**How features were ranked:** by the four equally weighted judging criteria: Design (a complete product, not just a proof of concept), Potential Impact (a real problem for a real family), Innovation (AI-built widgets) and Presentation (a demo under 3 minutes).

## Phase 1: Make the core solid (Oct 12–16)

1. **Fresh, fast widgets** (~2 days)
   - Cache curated (Articles, Local Events) and described-widget results per member, and show "Updated 2h ago".
   - The refresh button fetches new data for described widgets too, re-running their saved `source` and `query`.
   - Why: described widgets are frozen snapshots today, and every page load runs up to 8 Brave searches plus LLM calls. That's slow, costs credit, and looks bad in the video. It also has to land before deployment.
2. **Kid-safe picks** (~½–1 day)
   - Members with the Kid role get Brave `safesearch=strict` and an age-appropriate instruction to the LLM. Parents see a "kid-safe" marker.
   - Why: parents need to trust what kids see. It strengthens the Impact case, and it uses the role that already exists.
3. **Weather source** (~1 day)
   - Add Open-Meteo (free, no key; non-commercial terms, CC BY 4.0 attribution) to the describe-a-widget source list.
   - Why: weather was in the scope's data-source list but was never built. It's a good card for the morning glance.

## Deployment with safeguards (Oct 17–18)

4. **Public demo deployment**
   - Host the Go API and the built frontend somewhere SQLite can live on a persistent disk.
   - Provide a demo family account (testing credentials go in the Devpost testing instructions).
   - Rate-limit describe-a-widget and refresh, and cap daily spend.
   - API keys live only in the host's secrets.
   - The user creates the hosting account and enters the keys themselves.
   - Why: judges can try the real product. Rate limits protect OpenRouter and Brave credit.

## Phase 2: Shared family widgets (Oct 19–22)

5. **Shared family widgets**
   - A parent can pin a widget to every member's page, or send a widget to one member's page.
   - Shared widgets are labeled and can't be edited by the people receiving them.
   - Why: it makes Porchlight a *family* product rather than separate personal pages. It's the clearest family moment for the demo, and it reuses the existing widget and layout code.

**If time is left:** refine a described widget with a follow-up sentence ("only show launches this month").

## Not before the deadline

Homework review, gamification, unified family calendar or iCal "Today" card, mobile app, external sign-in, multiple families, open-ended UI generation, sending real invite emails. Each is a large new system that wouldn't show well in a 3-minute demo. These stay on the v2 list in `devpost/scope.md`.
