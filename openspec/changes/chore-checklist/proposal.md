## Why

Kinlight is now running for the family, but it cannot yet track the everyday jobs parents assign. A small daily checklist gives each member a clear list and lets parents see what is finished without another chat or reminder system.

## What Changes

- Parents create, rename, and remove daily chores for an existing family member, including themselves and other parents. Each chore has one assignee and a short title.
- A fixed checklist card appears on the member’s homepage, outside the Arrange layout. A parent sees the same checklist when viewing that member’s page and gets a compact family progress summary on their own page.
- Members check and uncheck their own chores. Completion is immediate; parents can check or undo any family member’s chores. There is no approval queue.
- Every list starts unchecked for the new calendar day at midnight in America/Denver, including while a member is traveling. Prior-day completion records are retained internally; no history dashboard is included.
- Keep this first version manual and daily: no rewards, points, streaks, weekday schedules, due times, reminders, AI tools, shared widgets, or bulk assignment.
- In this planning PR, also correct the existing restore guide and record the owner-reported successful deployment and restore drill. These documentation edits do not claim automated production verification or implement chores.

## Capabilities

### New Capabilities

- `chore-checklist`: Parent-managed daily assignments, member completion, family progress, and household-local day rollover.

### Modified Capabilities

None. The deployment documentation cleanup clarifies the existing backup requirement; it does not change production behavior.

## Impact

Implementation will add SQLite migrations and a dedicated Go chore service/HTTP handler, reuse the existing parent/member authorization, and add typed API hooks and homepage cards in React. Existing Puck layouts, calendar time zones, chat agents, and AI usage are unchanged. No external service or new runtime dependency is needed. This PR contains only the proposal, design, delta spec, implementation tasks, and deployment documentation cleanup.
