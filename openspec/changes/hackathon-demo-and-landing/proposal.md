## Why

Kinlight is working for the owner's household, but judges need an understandable public introduction and a populated, safe environment to try it. The owner has resumed submission preparation for Build With AI: Basics; this supersedes the earlier decision to skip submission without rewriting the historical Devpost planning documents.

## What Changes

- Add a public landing page at `/` for signed-out visitors while preserving signed-in homepages and existing member links.
- Add a separately deployed, production-protected judge demo with fictional Rivera family accounts, prepared dashboards, current synthetic calendars, chores and clearly labeled sample conversations.
- Provide preset parent and kid credentials through submission testing instructions; retain ordinary password login and role permissions.
- Enable real AI responses with server-enforced usage limits and separate demo provider credentials. Exhaustion leaves existing content available.
- Provide an operator-only, repeatable demo reset, with no route or credentials capable of resetting the real household database.
- Include a video preparation checklist, roughly two-minute shot sequence, judge-access checks and submission handoff. Recording, publishing and submitting remain owner actions.

## Capabilities

### New Capabilities
- `hosted-judge-demo`: isolated demo configuration, seeded experience, login, usage limits and reset lifecycle.
- `public-landing`: signed-out introduction and navigation to the demo, video and family sign-in.

### Modified Capabilities
- None. Existing local demo commands remain local-only; hosted-demo provisioning is a separate explicit operation. Production protections, real-family backup requirements and child safety remain unchanged.

## Impact

Backend configuration and startup, demo seed helpers, calendar fixture selection, outbound AI/search boundaries, and demo-only write restrictions; frontend routing and public/demo UI; separate deployment configuration and operator documentation. No new framework, public signup, payments, general multi-family hosting or new family features. The production family database, volume, port 1337 and backup path must remain untouched by demo provisioning and reset.

Planning only. Implementation, infrastructure provisioning, live AI checks, video upload and submission are distinct later steps.
