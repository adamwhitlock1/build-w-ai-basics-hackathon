## Purpose

Introduces Kinlight to signed-out visitors and directs judges to the demo while keeping existing family navigation and private content protected.

## ADDED Requirements

### Requirement: Session-aware root page
At `/`, anonymous visitors SHALL see a public landing page and signed-in members SHALL see their existing homepage. Existing parent member links using `?as` and incomplete-family setup handling SHALL remain functional. Other protected routes and private APIs SHALL retain authentication. Anonymous loading and error states SHALL not render or fetch private homepage content.

#### Scenario: Returning parent
- **WHEN** a signed-in parent opens an existing `/?as=<member-id>` link
- **THEN** the corresponding authorized member view opens without an intervening landing page

#### Scenario: Anonymous visitor
- **WHEN** a visitor without a session opens `/`
- **THEN** the landing page appears without private family data and a family-sign-in action leads to login

### Requirement: Honest public introduction and links
The landing page SHALL explain personalized family pages, daily routines and AI help with parent oversight, using only fictional-family screenshots. It SHALL provide a configured demo link and family sign-in, and a video action only when a valid video URL is configured. Missing demo configuration SHALL display an unavailable state without a broken link. Public configuration SHALL expose only approved nonsecret fields. There SHALL be no public signup promotion.

#### Scenario: Video is not uploaded
- **WHEN** no video URL is configured
- **THEN** the video action is omitted and demo and family-login navigation remain usable

### Requirement: Accessible responsive presentation
The page SHALL use the existing Kinlight identity, support Daybreak and Porch light themes and reduced motion, and expose readable headings, descriptive image alternatives and visible keyboard focus. At phone and desktop widths it SHALL remain usable without horizontal overflow.

#### Scenario: Keyboard and mobile visit
- **WHEN** a visitor navigates at 390px width using a keyboard in either theme
- **THEN** all actions are reachable with visible focus, content stays within the viewport and text retains the project's contrast requirements
