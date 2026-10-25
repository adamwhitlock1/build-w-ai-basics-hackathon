## ADDED Requirements

### Requirement: Brass on the few things that matter
A 1 px brass hairline, a gradient from `#E7C98E` to `#B08D57`, SHALL edge the briefing card and the add-a-card button in both themes. Nothing else in the app SHALL use brass, and no element SHALL rely on the brass line to be found or understood.

#### Scenario: The briefing card
- **WHEN** Maya views her page in either theme
- **THEN** the briefing card has a brass hairline edge, and no other card does

### Requirement: Lantern glass on floating surfaces
Surfaces that float over the page SHALL be lantern glass: frosted with a background blur, tinted about 4% amber, and with a faint grain. That's the arrange toolbar, the card action bar in arrange mode, and the chat and Assistant composers when they stick to the bottom of the screen. Text and controls on glass SHALL meet the same WCAG AA contrast as on a plain surface, in both themes, whatever is behind the glass.

#### Scenario: The arrange toolbar in Porch light
- **WHEN** Sam arranges his page in Porch light and scrolls cards under the toolbar
- **THEN** the toolbar is frosted, warm glass, and its text stays readable over any card

### Requirement: Wood on brand surfaces only
A faint wood-grain texture SHALL appear only on brand surfaces: the welcome pages (sign-in, sign-up, invite and create-family), the sidebar, and empty states. It SHALL NOT appear behind cards, chats, forms or any other content. Text on a textured surface SHALL meet WCAG AA contrast in both themes. The welcome pages SHALL use porch boards as a layout rhythm: horizontal bands separated by a fine seam.

#### Scenario: Signing in
- **WHEN** Lily opens the sign-in page
- **THEN** the page's ground shows faint porch boards and grain, and the sign-in form sits on a plain surface

#### Scenario: A card's content
- **WHEN** Maya reads her Articles card
- **THEN** no grain or texture is behind the card's text

### Requirement: A slot for the porch illustration
The welcome pages SHALL show a porch illustration in a wide slot, and empty states in a smaller one. Until a commissioned illustration is added, a simple stand-in SHALL fill the slots, drawn from porch boards, the haint-blue ceiling and the lantern, with no people and no faces. The illustration SHALL follow the theme, SHALL be decorative (hidden from screen readers), and SHALL never be the only place information appears. Replacing the stand-in with a commissioned drawing in the documented format SHALL need no code change.

#### Scenario: Both themes
- **WHEN** the sign-in page is shown in Daybreak and then in Porch light
- **THEN** the porch illustration uses each theme's colors

#### Scenario: A commissioned scene arrives
- **WHEN** an illustrator's SVG, made to the documented format, replaces the stand-in file
- **THEN** it shows in every slot, in both themes, with no code changed

### Requirement: The voice of a good neighbor
Everything a family member reads SHALL be plain, warm, specific and brief:
- **The word.** Cards SHALL be called "cards", never "widgets", in labels, buttons, messages, hints, empty states and errors. Code, API paths and stored data are not affected.
- **Empty states.** Each SHALL say what's missing and what to do next, in a sentence.
- **Errors.** Each SHALL say what didn't work and what to do, with no status codes or internal terms.
- **Phrases.** The app SHALL never say "mental load", "in sync", "supercharge", "seamless" or "powered by AI".

A check in CI SHALL fail when visible text says "widget" or uses one of those phrases.

#### Scenario: Adding a card
- **WHEN** Maya opens the prompt to describe something new
- **THEN** the button reads "Add a card", and while it's built the status reads "Building your card…"

#### Scenario: A card that's gone
- **WHEN** a parent asks for a card that was removed
- **THEN** the message reads "That card doesn't exist."

#### Scenario: A banned word slips in
- **WHEN** a change adds the visible text "Add a widget"
- **THEN** the copy check fails in CI
