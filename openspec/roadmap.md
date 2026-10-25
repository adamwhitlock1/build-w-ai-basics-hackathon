# Porchlight Post-PoC Roadmap

The proof of concept is done (tag `poc-complete`). This roadmap orders the features added after it. Each item becomes one OpenSpec change (`/opsx:propose` → `/opsx:apply` → `/opsx:archive`). When a change is archived, add a line to "Post-PoC Additions" at the end of `devpost/prd.md` and `devpost/spec.md`.

**Deadline:** Devpost submission closes **2026-10-26, 5:00 pm ET**. Feature freeze is **2026-10-23**. October 24–26 go to `6-ship` (demo video, public repo, write-up), with a day of buffer. Aim to submit by October 25.

**How features were ranked:** by the four equally weighted judging criteria: Design (a complete product, not just a proof of concept), Potential Impact (a real problem for a real family), Innovation (AI-built widgets) and Presentation (a demo under 3 minutes).

## Phase 1: Make the core solid (done Oct 11–12)

Phase 1 finished four days early. Its three changes are archived in `openspec/changes/archive/`.

1. **Fresh, fast widgets** (done 2026-10-11)
   - Remembers curated and described-widget results per member, refreshes them in the background, and shows "Updated 2h ago".
2. **Kid-safe picks** (done 2026-10-11)
   - Kids get Brave strict safe-search and under-18 AI guidance. Parents see a "Kid-safe" marker.
3. **Weather source** (done 2026-10-12)
   - Open-Meteo forecasts for described widgets, using the member's saved location when no place is named.

## Phase 1b: The family's morning (Oct 13–17)

Added on 2026-10-12 to use the days Phase 1 freed up. The riskiest item goes first, so problems show up while there's still slack.

4. **Family "Today" card** (done 2026-10-12; planned for Oct 13–14)
   - A member pastes a private calendar link (the iCal / ICS address Google Calendar and iCloud provide), and a "Today" card lists that day's events in the member's time zone.
   - A parent can set the link for a kid, as they already set a kid's interests.
   - The link is a secret: it's stored on the server, never sent back to the browser in full, and never shown to other members.
   - Repeating events and all-day events must show correctly on the right day.
   - Why: it's the strongest family-impact story, and it partly delivers the "unified family calendar" that the scope cut. It also needs the most care: repeating events, time zones and storing the secret.
5. **Morning briefing** (done 2026-10-13; planned for Oct 15)
   - An AI-written card of a few warm sentences that sums up the member's own day from what their other cards already hold: today's weather, today's calendar events, nearby events and articles. For example: "Morning, Sam! 75° and sunny, soccer at 4, and a Starlink launch Tuesday."
   - It's written once per member per day from results already remembered, so it costs one AI call a day and no new searches. It uses kid-safe guidance for kids.
   - Why: it's the best demo opener, and an AI idea a judge sees at once. It also ties the other cards together into one morning glance.
6. **Chore checklist** (~1.5 days, Oct 16–17)
   - A parent gives a kid (or anyone) a short list of chores. The member ticks them off on their own page, and the list resets each day.
   - Parents see each member's progress on their page. There's no points ledger or rewards: that part of the scope's gamification idea stays in v2.
   - Why: a daily job real families do, and a parent → kid moment for the demo. It defines how a parent puts something on someone else's page, which shared family widgets can reuse.
   - **Cut line:** if it isn't done by the end of Oct 17, it moves after shared family widgets, or is dropped, rather than delaying deployment.

## Deployment with safeguards (Oct 18–19)

7. **Public demo deployment**
   - Host the Go API and the built frontend somewhere SQLite can live on a persistent disk.
   - Provide a demo family account (testing credentials go in the Devpost testing instructions).
   - Seed the demo family with a sample calendar and a chore list, so judges see those cards working without setting anything up.
   - Rate-limit describe-a-widget, refresh and the morning briefing, and cap daily spend.
   - API keys live only in the host's secrets.
   - The user creates the hosting account and enters the keys themselves.
   - Keep the Open-Meteo credit, and keep the demo non-commercial.
   - Why: judges can try the real product. Rate limits protect OpenRouter and Brave credit.

## Phase 2: Shared family widgets (Oct 20–22)

8. **Shared family widgets** (~3 days)
   - A parent can pin a widget to every member's page, or send a widget to one member's page.
   - Shared widgets are labeled and can't be edited by the people receiving them.
   - Reuse the parent → member placement that the chore checklist introduces.
   - Why: it makes Porchlight a *family* product rather than separate personal pages. It's the clearest family moment for the demo, and it reuses the existing widget and layout code.

**If time is left:** refine a described widget with a follow-up sentence ("only show launches this month"). Suggested widgets built from a member's interests (~½ day) would also help judges who land on an empty page.

## Not before the deadline

Homework review, a points-and-rewards ledger, a combined calendar for the whole family (beyond each member's own Today card), mobile app, external sign-in, multiple families, open-ended UI generation, sending real invite emails. Each is a large new system that wouldn't show well in a 3-minute demo. These stay on the v2 list in `devpost/scope.md`.

---

# v2: The Family Agent (from 2026-10-23)

On 2026-10-23 the hackathon submission was skipped in favor of building v2. Everything above is the hackathon roadmap, kept as history. Of its items, the chore checklist, public demo deployment and shared family widgets were never built. They can be planned again as v2 changes if still wanted.

## Decisions so far

- **Audience:** one household (two parents, kids aged 12 and 15), self-hosted on a homelab. SQLite and a single Go process. A product for other families is a long way off, but the guards should still be strong.
- **Go:** upgrade to the newest stable release. **Agent framework:** Genkit for Go (its fit is being checked before the `genkit-integration` change).
- **The family agent:** every member gets one.
  - For kids it's a tutor and a curiosity/creative companion. The rule "guide, don't answer" applies only to messages the model marks as assignments. A hint ladder enforced in Go (not only in the prompt) caps how much it gives away.
  - For parents it's an assistant that can also review the kids' conversations.
- **Oversight:**
  - Parents (admins) can see every kid conversation, and kids always see "Your parents can review these chats".
  - A fast pre-flight model flags risky kid messages for parent review and doesn't block them.
  - Possible crisis messages are flagged, and the kid's reply always includes crisis resources (call or text 988; text HOME to 741741).
  - Parents get an in-app review queue first; phone push comes later.
- **Tools:** kids get a restricted, safe set. Adults get standard tools such as web research.
- **Models:** prototyping may use OpenRouter free models or the MiniMax plan. Before the kids use the agent for real, their conversations must go only to no-retention, no-training providers. The kid-tutor change enforces this.
- **Cost:** no ceilings yet. Usage is tracked per member.
- **Shape:** separate chats with automatic titles, a chat page, and a small card on each homepage that opens it.
- **Quality:** a well-structured, tested codebase. Fake-model unit tests and route-level integration tests run in CI. An opt-in evaluation script checks real tutor behavior.

## Planned changes, in order

Each item is one OpenSpec change, each shippable and testable on its own. Changes 1 and 2 don't involve AI; they make the later ones cheaper and safer.

1. **`go-upgrade`:** the newest stable Go and refreshed dependencies. Re-test whether the `CGO_ENABLED=0` workaround is still needed.
2. **`v2-foundation`:**
   - Move business logic out of HTTP handlers into services the agent can call.
   - Replace `routes()`'s long parameter list with a dependencies struct.
   - Make the middleware streaming-ready (`statusRecorder.Unwrap`).
   - Add CI (Go tests and a frontend build), and track usage per member.
3. **`genkit-integration`:**
   - Wire Genkit in beside the existing `internal/llm`, with model config for OpenRouter and MiniMax.
   - Port one existing call (the morning briefing) to prove it.
   - Add a fake-model test kit and Dev UI tracing.
4. **`kid-tutor-agent`:**
   - Conversations stored in SQLite; personas for ages 12 and 15; the hint ladder.
   - Pre-flight flagging, crisis flags and resources, and the parent review queue with transcripts.
   - The kid notice and the no-retention gate.
5. **`agent-tools`:** the kid agent's tools: weather, my day, my picks, and kid-safe search (Explore only, at most 2 per message). Kids and parents see each tool use in the chat.
6. **`parent-agent`:** the parent assistant, its adult tools (web research, …), and tools for reviewing the kids' chats.

All six v2 changes are planned and built (see `openspec/changes/archive/`).

Later, as they become useful: phone push for flags, streaming polish, cost ceilings, long-term agent memory, homework photos, and the unbuilt hackathon items above.

## Series 2: Kinlight, then the homelab (from 2026-10-25)

Apply the identity in `brand/` to the app, then run it for the family on the homelab.

**Decided 2026-10-25:**
- **Name:** the app shows **Kinlight**, the working name from `brand/naming.md`. Code, the repo, the Go module and the database keep their names, so a later rename is a copy change. Clearance of the name is still open.
- **Typefaces:** Fraunces (with its soft axis) and Figtree, self-hosted.
- **Dark theme:** Porch light follows the device setting by default, with a Daybreak / Porch light / Automatic switch remembered per member.
- **Order:** the rebrand first, so the family's first real use is the finished identity.

**Planned changes, in order:**

1. **`kinlight-identity`:**
   - The name in the UI and in the agents' prompts.
   - The lantern logo, favicon, app icon and header lockup.
   - Fraunces and Figtree.
   - The brief's color tokens for both themes, and the theme switch.
   - Also: the agents' prompts refer to the kids by name instead of guessing pronouns.
2. **`lantern-moments`:** the motion system in `brand/lantern-ux.md`:
   - motion tokens;
   - kindle on sign-in;
   - a breathing glow instead of every spinner;
   - a card catching the light when it arrives;
   - card edges that fade as content ages;
   - the dusk theme change;
   - at most one lit thing per screen;
   - plain fades under reduced motion.
3. **`brand-surfaces`:**
   - brass hairlines and lantern-glass sheets;
   - wood on sign-in, empty states and the sidebar;
   - the porch illustration;
   - a voice pass over the app's copy (brief section 8).
4. **`homelab-deployment`:**
   - a release build and a service;
   - backups of the SQLite file;
   - the kids' zero-retention key;
   - the family's real accounts.

**Later, from the lantern doc:**
- "Off you go" needs morning steps, which aren't built (the chore checklist was never built).
- "Who's home" needs check-in presence.
- Pull to refresh, sound and haptics wait for a phone app.
