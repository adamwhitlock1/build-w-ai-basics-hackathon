# chore-checklist Specification

## Purpose

Give each family member a daily checklist of parent-assigned chores, with immediate completion and a consistent household day so parents can see progress without asking each person.

## Requirements

### Requirement: Parents manage daily assignments
Parents SHALL create, rename, and remove chores for any existing member of their own family, including themselves. Each chore SHALL have one assignee and a trimmed plain-text title of 1–120 Unicode characters. A member SHALL have at most 30 active chores. New chores SHALL appear unchecked immediately and repeat daily. The list SHALL keep creation order, including after renaming. Renaming SHALL preserve today's completion. Removing a chore SHALL hide it immediately from the checklist and current totals. Reassignment, custom ordering, and bulk assignment SHALL be outside this version.

#### Scenario: Assign and edit a chore
- **WHEN** a parent adds “Feed the dog” on a child's page and later renames it after the child checks it off
- **THEN** the same chore remains checked today, retains its position, and uses the new title

#### Scenario: Remove a chore
- **WHEN** a parent confirms removal of a checked chore
- **THEN** it disappears from the member's list and both completed and total counts adjust; it does not return tomorrow

#### Scenario: Invalid or excessive assignment
- **WHEN** a parent submits a blank title, a title longer than 120 characters, or a 31st active chore
- **THEN** the request is rejected with a specific validation message and the existing checklist remains unchanged

### Requirement: Immediate completion and undo
Members SHALL check and uncheck their own chores for today. Parents SHALL also check and uncheck chores for any member of their family. A successful check SHALL count immediately without parent approval. Repeating a request for the same desired state SHALL not double-count or toggle the chore. Simultaneous conflicting requests SHALL use the last successfully committed state, and subsequent reads SHALL show that state.

#### Scenario: A member finishes a chore
- **WHEN** a member checks an unchecked chore and the save succeeds
- **THEN** it stays checked after a reload and today's completed count increases by one

#### Scenario: A parent corrects a checkmark
- **WHEN** a parent unchecks a child's completed chore
- **THEN** it is incomplete for today on subsequent reads, with no approval queue or penalty

#### Scenario: A duplicate request
- **WHEN** the same request to set a chore completed is received twice
- **THEN** the chore is completed once and the count increases only once

### Requirement: Household midnight rollover
Every checklist SHALL use the America/Denver calendar date, independent of the viewer's device time zone. Each active chore SHALL start unchecked on a new household date. Daylight-saving transitions SHALL follow that time zone's calendar midnight rather than a fixed 24-hour interval. Rollover SHALL work after server downtime without a scheduled reset job. Prior-day completion records SHALL remain stored separately and SHALL not count toward today. This version SHALL offer no history view or past-day edits.

#### Scenario: Midnight with an open page
- **WHEN** the household date changes while a checklist is visible
- **THEN** the page requests the new day's checklist, and yesterday's checkmarks no longer appear as today's work

#### Scenario: Traveling or daylight-saving time
- **WHEN** a member views the list from another time zone or across a Denver daylight-saving transition
- **THEN** its day and reset boundary remain the same as those seen by the household in Denver

#### Scenario: Server restarts after midnight
- **WHEN** the server starts after being offline across one or more midnights
- **THEN** the current date's checklist is correct without replaying missed reset jobs

#### Scenario: A stale checkmark crosses midnight
- **WHEN** a completion request carries yesterday's displayed date but reaches the server after household midnight
- **THEN** it is rejected as stale, neither day's completion is changed, and the UI reloads today with an explanation

### Requirement: Family-scoped access
All chore endpoints SHALL require authentication. Kids SHALL only read and change their own completion state; they SHALL NOT create, rename, remove, or view other members' chores or the family summary. Parents SHALL access only their own family's chores. A nonexistent or out-of-family member/chore identifier SHALL return the same not-found response without exposing another family's data. Authorization SHALL be enforced on the server for every request, including mutations and summary reads.

#### Scenario: A kid attempts management
- **WHEN** a kid submits a create, rename, remove, family-summary, or other-member request directly
- **THEN** it is refused and no data changes

#### Scenario: Another family's identifier
- **WHEN** a parent supplies a member or chore identifier belonging to another family
- **THEN** the response is indistinguishable from a nonexistent identifier and no data changes

#### Scenario: A chore is removed before completion saves
- **WHEN** a completion request reaches the server after that chore has been removed
- **THEN** it is refused as not found and does not restore or complete the removed chore

### Requirement: Homepage checklist and parent progress
The member's homepage SHALL show a fixed “Today's chores” card outside the Arrange layout. Kids with no chores SHALL see “No chores assigned yet”; parents SHALL have an add action for themselves and on the member page they are viewing. Parents SHALL also see today's completed/total counts for each family member on their own page, with a link to that member's page. Zero chores SHALL read “No chores assigned”, not imply completion. Parent summaries and open checklists SHALL refresh on focus, after relevant local mutations, and at least every 60 seconds while visible; background tabs SHALL refresh when visible again. The household time zone SHALL be visible in the chore UI.

#### Scenario: Parent checks family progress
- **WHEN** a parent opens their homepage
- **THEN** they see each member's completed and assigned counts for the same Denver date and can open a member's checklist

#### Scenario: Someone else changes a checkmark
- **WHEN** a child finishes a chore while the parent's page stays open
- **THEN** the parent's summary reflects the saved change within 60 seconds while visible, or on return to the tab

#### Scenario: Existing customized homepage
- **WHEN** a member with a saved Arrange layout receives their first chore
- **THEN** the chore card appears without rewriting that layout, and rearranging or removing existing widgets does not remove assigned chores

### Requirement: Accessible and honest save states
Chore controls SHALL be keyboard-operable, have accessible labels and checked states, and work on a narrow screen in both themes. During a save, the affected control SHALL prevent duplicate interaction and convey progress. Failed requests SHALL preserve the last confirmed state, show a useful error, and allow retry. Loading and failed summary requests SHALL not display fabricated zero counts. Chore titles SHALL render as text, never markup. A delete action SHALL request confirmation naming the chore.

#### Scenario: A save fails
- **WHEN** a checkmark or title change fails because the network is unavailable
- **THEN** the UI keeps the last confirmed value and shows an error with a retry path

#### Scenario: Keyboard use
- **WHEN** a member navigates with the keyboard and activates a chore checkbox
- **THEN** they can complete it and perceive the saved or failed state without relying on color or animation
