## Purpose

Provides judges a populated fictional family they can explore with live AI while keeping the real household's data, credentials and deployment separate.

## ADDED Requirements

### Requirement: Isolated production-protected demo
The hosted demo SHALL require explicit demo configuration and a marked, dedicated data location. It SHALL use separate storage, hostname and provider credentials from the household service. HTTPS cookies, security headers, origin protections, login throttling and child-safety rules SHALL remain enabled. Development-only settings SHALL remain forbidden in production. The hosted demo SHALL neither restore from nor write to household backups.

#### Scenario: Invalid target
- **WHEN** demo startup or provisioning targets unmarked existing data or a household database location
- **THEN** it refuses without modifying that data

#### Scenario: Demo is disabled
- **WHEN** the normal household server starts without hosted-demo mode
- **THEN** no demo provisioning, credential behavior or fixture override is enabled

### Requirement: Judge accounts and shared-demo notice
Judges SHALL log in with operator-provided preset parent or kid credentials using ordinary authentication. Testing instructions SHALL identify both roles. The demo SHALL visibly explain its shared, fictional and resettable nature and ask visitors not to enter personal information. Demo signup, invitations, credential changes and external calendar configuration SHALL be denied on the server. Parent and kid permissions SHALL remain enforced.

#### Scenario: Child experience
- **WHEN** a judge logs in as Jordan
- **THEN** Jordan's page and chats are available and another member's private APIs remain inaccessible

#### Scenario: Preserving access
- **WHEN** a demo visitor attempts an invitation, password change or external calendar write directly through an API
- **THEN** it is refused without changing shared access or making an external calendar request

### Requirement: Populated current fictional family
The hosted demo SHALL provide two parents and two kids with distinct dashboards, current synthetic calendars where assigned, seeded widget content, sample briefing content, daily chores and labeled sample chats including a parent-review example. Calendar dates SHALL follow the current demo date without requiring external feeds. Existing local demo commands SHALL retain their local-only safeguards.

#### Scenario: First visit
- **WHEN** a judge signs in after provisioning
- **THEN** the family pages are populated, sample content is identified, chores can be completed, and parents can review the seeded flag

### Requirement: Bounded live AI and search
The demo SHALL make real AI calls subject to configurable, persistent, atomic daily model-request and paid-search allowances. Every outbound attempt, including automated calls, retries, safety checks and tool follow-ups, SHALL consume allowance before dispatch. The demo SHALL bound concurrent AI operations and model output sizes. Allowances SHALL reset at midnight America/Denver and SHALL survive process restarts, logouts and content resets. Failed quota accounting SHALL prevent provider dispatch. Child safety SHALL never be bypassed to fit an allowance.

#### Scenario: Concurrent final allowance
- **WHEN** concurrent requests compete for the last model-request slot
- **THEN** at most one outbound model request uses that slot and the rest receive a limit outcome

#### Scenario: Exhaustion or mid-turn exhaustion
- **WHEN** any generation or search cannot reserve allowance
- **THEN** no corresponding provider call occurs, the UI explains the limit and next reset time, partial output is marked incomplete, and existing content remains browsable without automatic retry loops

#### Scenario: Reset is not extra credit
- **WHEN** demo content is reset or the process restarts after quota exhaustion
- **THEN** the current day's consumed allowance remains consumed

### Requirement: Controlled reset and redeployment
Only the operator SHALL reset hosted-demo content, using explicit target confirmation and verified demo identity while writes are stopped. A successful reset SHALL restore the prepared family and invalidate sessions without resetting quota. Failure SHALL preserve usable prior content. Routine redeployment SHALL preserve current demo data; there SHALL be no publicly callable reset operation.

#### Scenario: Reset rehearsal
- **WHEN** the operator resets a modified demo
- **THEN** prepared dashboards, chores and sample chats return, previous sessions expire, quota is retained, and household data is unchanged
