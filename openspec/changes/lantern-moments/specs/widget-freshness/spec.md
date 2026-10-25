## ADDED Requirements

### Requirement: A card's age shows as light
Every card that shows an update time SHALL also show a warm top edge whose light fades with the card's age: brightest just after an update, gone after about six hours. It SHALL stay current while the page is open, as the label does, and SHALL be dimmer than the one full-strength glow on a screen. The update time in words SHALL remain, so the edge is never the only signal.

#### Scenario: Just refreshed
- **WHEN** Maya's Articles card was updated a minute ago
- **THEN** its top edge is warmly lit, and its label reads "Updated 1 min ago"

#### Scenario: Settled
- **WHEN** Maya's Local Events card was updated eight hours ago
- **THEN** its top edge shows no light, and its label reads "Updated 8h ago"

#### Scenario: Aging while open
- **WHEN** Maya's page stays open for three hours after a card refreshed
- **THEN** that card's edge has dimmed to about half, without a reload
