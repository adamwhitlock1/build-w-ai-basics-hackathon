---
doc: scope
status: approved
---
# Porchlight

One line: a per-family-member homepage where every widget is AI-driven — pre-built ones are auto-curated from each member's interests, and any member can describe a new widget in plain language and have it built and rendered.

## The Unique Kernel
The widgets on the page are not hardcoded UI components. They are **AI-driven artifacts**: the pre-built ones (articles, local events) are curated per-user by an LLM against that member's interests, and the user can create a new widget just by typing a sentence ("show me news about space exploration") — the LLM picks from a small known menu of data sources, persists the spec, and the widget appears, populated. The "describe it in plain language and watch it appear on the page" beat is what makes this not just a feed reader.

## Who It's For
The learner's own household: the learner (admin), their spouse (admin or co-admin), their ~15-yr-old daughter, and their ~12-yr-old son. They want one place where each family member sees the things that matter to *them* and can arrange the page how they like. Today, that probably means a mix of news apps, calendars, and shared notes — none of which feel personal or unified per-person.

## The Core Loop
Open the app → land on your homepage, populated with curated articles and local events → drag widgets to rearrange the layout, persistence so it sticks → type a sentence in the "new widget" prompt → a new widget appears, populated with relevant data. They come back because their interests are tracked and the page becomes more theirs over time.

## Inspiration & Identity
- [OpenUI](https://www.openui.com) — the feel of AI-generated UI, where the user describes what they want and the interface builds it.
- [CopilotKit / OpenIntelligentUI](https://github.com/CopilotKit/OpenIntelligentUI) — same idea, the open-intelligent-UI thread of research the user wants to explore.
- Vibe: friendly, personal, modern. Not corporate. The drag-and-drop should feel satisfying; the natural-language widget creation should feel like a small magic trick. The "every widget is alive" energy — the page grows with the user.

## Why This Matters to the Learner
The learner wants to build toward a market-based concept eventually, but use it in their own household first to refine features and dogfood it daily. v1 is the daily-use test bed for the family; v2 is the market product. The PoC has to be good enough that the learner would actually open it on a Tuesday morning — otherwise it can't do its job as the v1 test bed.

## What "Working" Looks Like
A ~60-second screen recording with three beats that prove the kernel:

1. **Setup:** a parent signs in, creates the family, adds the spouse and the two kids as members, and sets interests for one of them (e.g., "space exploration, soccer, indie music").
2. **Customize:** that family member opens their homepage, sees the curated articles and local events widgets, and drags them into a new order. The layout persists.
3. **Describe-a-widget:** they type a sentence — "show me upcoming SpaceX launches" — and a new widget appears on the page, populated with live data.

The "oh, that's cool" beat is #3: a sentence in, a working widget out, no rebuilding the page required.

## The POC Boundary
In, tightly:
- Multi-user, per-family-member homepages, with 1–2 admin(s).
- Admin can create the family, invite members, and set each member's interests.
- Pre-built widget types: **articles** (curated by LLM from the member's interests) and **local events** (curated by LLM from interests + a location).
- Drag-and-drop layout, persisted per member.
- A "describe a widget" prompt that produces a new widget from a small, known menu of data sources (news search, events search, a generic web fetch, weather). The LLM picks the source and the display shape, and the widget renders.
- Go API backend, simple frontend, runs locally. No production deployment.
- The learner will use this Go stack to learn backend/API development with Go — that is the stated learning goal for the project.

## Later
Worth doing, just not now:
- Homework photo upload + admin review/check-off
- Gamification (points for chores, homework, admin-assigned tasks)
- Unified family calendar view
- Per-member schedule widget
- Daily fun / meme widget
- Truly open agentic UI generation (general "describe any UI" — research-grade, weeks of work)
- Multi-tenant / multi-family market features
- External auth providers
- Production deployment
- Mobile app

## Explicitly Cut
Named, each with a reason:
- **Daily fun / meme widget** — cut during scope to make room for the natural-language widget feature. Funny content is a nice-to-have, not part of the kernel.
- **Per-member schedule widget** — cut during scope. Schedule is a real feature, but the kernel is "AI-curated content + describe-a-widget"; a schedule widget is a calendar problem in disguise and isn't load-bearing for v1.
- **Homework review** — separate product surface (photo storage, review queue, notifications). Explicitly v2.
- **Gamification** — requires a points ledger, history, and reward logic. Explicitly v2.
- **Unified family calendar view** — large UI surface; not part of the personalized-homepage kernel. v2.
- **General agentic UI generation (OpenUI-style full open generation)** — research-grade; v1 implements a *constrained* version with a small menu of data sources, which still proves the kernel.
- **Multi-family / market features** — explicitly deferred per the learner's stated plan (v1 = own household; v2 = market).
- **External auth providers** — v1 uses simple local auth only; OAuth and SSO are v2.
- **Production deployment** — PoC runs locally for the afternoon; deployment is out of scope.
