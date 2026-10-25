## MODIFIED Requirements

### Requirement: Parents see a kid-safe marker
When the viewer is a parent, a card whose content is kid-safe SHALL show a visible marker reading "Picked with <name>'s age in mind", where <name> is the kid whose page it is (as the app names members elsewhere). When the viewer is a kid, the marker SHALL NOT be shown. Cards that are not kid-safe SHALL NOT show the marker.

#### Scenario: Parent sees the marker on a kid's page
- **WHEN** a parent views Jordan's page
- **THEN** each kid-safe card shows a marker reading "Picked with Jordan's age in mind"

#### Scenario: Kid does not see the marker
- **WHEN** a kid views their own page
- **THEN** no kid-safe marker is shown, even though the content is kid-safe

#### Scenario: Old unfiltered widget has no marker
- **WHEN** a parent views a kid's page that includes a card saved before kid-safe mode existed
- **THEN** that card shows no kid-safe marker
