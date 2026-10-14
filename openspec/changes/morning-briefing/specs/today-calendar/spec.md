## MODIFIED Requirements

### Requirement: The Today card on every page
The Today card SHALL be a permanent card, like Articles and Local Events: every page has exactly one, members can move and resize it, and it can't be removed. A page that has never been arranged SHALL show it right after the morning briefing card, at full width. A page arranged before the Today card existed SHALL get it added at the top, below the briefing card if the page has one. Kid-safe filtering SHALL NOT apply to a member's own calendar, and the Today card SHALL NOT show the "Kid-safe" marker.

#### Scenario: New member's page
- **WHEN** a member who has never arranged their page opens it
- **THEN** the morning briefing card is first, and the Today card is second, both across the full width of the page

#### Scenario: Page arranged before the card existed
- **WHEN** a member whose saved layout has no Today card opens their page
- **THEN** the Today card appears at the top, below the briefing card, and the rest of their layout is unchanged
