## Purpose

Makes sure the content Porchlight finds for a kid member is suitable for someone under 18, and lets parents see which cards were made that way.

## ADDED Requirements

### Requirement: Kid members get kid-safe content
A family member who is not a parent (admin) SHALL be treated as a kid. All content the system finds or generates for a kid SHALL be produced in kid-safe mode. Kid-safe mode has two parts:
- every web, news and events search runs at the search provider's strictest safe-search level;
- every AI step that chooses searches or chooses, extracts or rewrites items is told the member is under 18, and is told to leave out content unsuitable for them: sexual content, graphic violence, drugs, alcohol, gambling, hateful content, and adults-only events or venues.

Content for parents SHALL be produced exactly as before this change.

#### Scenario: Kid's Articles card uses strict search
- **WHEN** a kid loads their Articles card
- **THEN** every news search behind it is made with strict safe-search
- **AND** the AI step that plans those searches is given the under-18 guidance

#### Scenario: Kid's "Happening near you" card filters events
- **WHEN** a kid loads their "Happening near you" card
- **THEN** every events search is made with strict safe-search
- **AND** the AI steps that plan the searches and extract the events are given the under-18 guidance

#### Scenario: Kid describes a widget
- **WHEN** a kid creates a widget by describing it
- **THEN** the search for that widget is made with strict safe-search
- **AND** the AI steps that choose the widget's search and shape its items are given the under-18 guidance

#### Scenario: Parent content is unchanged
- **WHEN** a parent loads their own cards or describes a widget for themselves
- **THEN** searches are made without a safe-search override
- **AND** no under-18 guidance is given to the AI

#### Scenario: Fallback still uses strict search
- **WHEN** an AI step for a kid fails and the system falls back to built-in searches or raw search results
- **THEN** those searches are still made with strict safe-search

### Requirement: The page owner decides kid-safety
Whether content is kid-safe SHALL depend on the member the content is for, not on who is viewing it.

#### Scenario: Parent views their kid's page
- **WHEN** a parent opens their kid's page and the kid's Articles or "Happening near you" card loads
- **THEN** the card is produced in kid-safe mode

#### Scenario: Kid views their own page
- **WHEN** a kid opens their own page
- **THEN** their cards are produced in kid-safe mode

### Requirement: Widget responses report kid-safety
Every curated widget response and saved widget response SHALL include a boolean `kidSafe`. It SHALL be true only if the content was produced in kid-safe mode. A saved widget SHALL keep the `kidSafe` value of its latest build or refresh. Widgets saved before this change SHALL report `kidSafe: false` until they are refreshed.

#### Scenario: Curated card for a kid
- **WHEN** a kid's Articles card is returned
- **THEN** the response has `kidSafe: true`

#### Scenario: Curated card for a parent
- **WHEN** a parent's Articles card is returned
- **THEN** the response has `kidSafe: false`

#### Scenario: Saved widget keeps its value
- **WHEN** a kid creates a described widget and it is later loaded from the homepage
- **THEN** both the create response and the later response have `kidSafe: true`

#### Scenario: Widget saved before the feature
- **WHEN** a widget saved before this change is loaded, and it hasn't been refreshed since
- **THEN** its response has `kidSafe: false`

### Requirement: Parents see a kid-safe marker
When the viewer is a parent, a card whose content is kid-safe SHALL show a visible "Kid-safe" marker. When the viewer is a kid, the marker SHALL NOT be shown. Cards that are not kid-safe SHALL NOT show the marker.

#### Scenario: Parent sees the marker on a kid's page
- **WHEN** a parent views their kid's page
- **THEN** each kid-safe card shows a "Kid-safe" marker

#### Scenario: Kid does not see the marker
- **WHEN** a kid views their own page
- **THEN** no "Kid-safe" marker is shown, even though the content is kid-safe

#### Scenario: Old unfiltered widget has no marker
- **WHEN** a parent views a kid's page that includes a widget saved before this change
- **THEN** that widget shows no "Kid-safe" marker

### Requirement: Remembered results respect kid-safety
A remembered (cached) curated result SHALL be sent again only to a member it was produced for: in kid-safe mode for a kid, and not in kid-safe mode for a parent. Any other remembered result SHALL be treated as missing and picked again in the member's mode. This includes results remembered before this change, which were not produced in kid-safe mode.

#### Scenario: Kid's result remembered before the feature
- **WHEN** a kid loads a curated card whose remembered result was produced before this change
- **THEN** the card is picked again in kid-safe mode, and that result is remembered in place of the old one

#### Scenario: Kid's kid-safe result is reused
- **WHEN** a kid loads a curated card whose remembered result was produced in kid-safe mode and is still usable
- **THEN** it is sent without searching again

#### Scenario: Parent's result is reused
- **WHEN** a parent loads a curated card whose remembered result was produced for them and is still usable
- **THEN** it is sent without searching again

### Requirement: Refreshing a kid's widget is kid-safe
Refreshing a described widget SHALL produce its new items in the mode of the widget's owner. After a kid's widget is refreshed, it SHALL report `kidSafe: true`, even if it was saved before this change.

#### Scenario: Kid refreshes an old widget
- **WHEN** a kid refreshes a widget they saved before this change
- **THEN** its search is made with strict safe-search, its items are shaped with the under-18 guidance, and it now reports `kidSafe: true`

#### Scenario: Parent refreshes their widget
- **WHEN** a parent refreshes their own widget
- **THEN** it is refreshed exactly as before this change and reports `kidSafe: false`
