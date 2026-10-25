# app-identity Specification

## Purpose
Gives the app one recognizable identity everywhere a family member meets it: the Kinlight name, the lantern mark, its typefaces, and a light and a dark theme that each member can choose, all readable in both themes.

## Requirements

### Requirement: The app is called Kinlight
Everything a family member reads SHALL name the app **Kinlight**:
- the browser tab title, the header, and the sign-in, sign-up and invite pages;
- the label on the agents' messages in kid chat and the Assistant;
- error messages and hints;
- empty-state prompts.

When the AI is told what app it's part of, it SHALL be told Kinlight, and replies SHALL NOT call the app Porchlight. The app's settings, session cookie and default database file SHALL use the Kinlight name. API paths and the data in the database SHALL be unchanged. The server SHALL refuse to start while a setting uses an old Porchlight name, which is any `PORCHLIGHT_*` variable or a database path naming `porchlight.db`. It SHALL say which name replaces it.

#### Scenario: Signing in
- **WHEN** Maya opens the sign-in page
- **THEN** the tab title and the page say Kinlight, and "Porchlight" appears nowhere on it

#### Scenario: An invite
- **WHEN** Grandma opens her invite link
- **THEN** the page invites her to join the Rivera family on Kinlight

#### Scenario: Kid chat
- **WHEN** Jordan asks the kid agent what it's called
- **THEN** its replies are labeled Kinlight, and it calls itself Kinlight's family helper

#### Scenario: An old setting
- **WHEN** the server starts with `PORCHLIGHT_DEV_FIXTURES=1` set
- **THEN** it doesn't start, and says to use `KINLIGHT_DEV_FIXTURES` instead

#### Scenario: An old database path
- **WHEN** the server starts with its database path set to `porchlight.db`
- **THEN** it doesn't start, and says to rename the file to `kinlight.db` and update the setting

### Requirement: The lantern mark
The browser tab icon, the home-screen icon (Apple touch icon) and the header SHALL show the lantern mark. The header SHALL show the lantern beside the Kinlight wordmark, followed by the family's name once the family has one. The mark SHALL be legible at 16 pixels and SHALL take its colors from the current theme, so it reads in both themes. Decorative copies of the mark SHALL be hidden from screen readers. The header SHALL still expose the name "Kinlight" as text to assistive technology.

#### Scenario: The header in both themes
- **WHEN** Maya views her page in Daybreak and then in Porch light
- **THEN** the header shows the lantern and the Kinlight wordmark, with the wordmark in each theme's ink and the lantern's glass lit

#### Scenario: The tab icon
- **WHEN** the app is open in a browser tab
- **THEN** the tab shows the lantern icon

### Requirement: Self-hosted typefaces
Headings SHALL be set in Fraunces and body and interface text in Figtree, served by the app itself. Loading any page SHALL NOT request fonts or font stylesheets from another host. Until the fonts load, text SHALL show in a fallback font rather than being invisible.

#### Scenario: No third-party font requests
- **WHEN** a page loads
- **THEN** every font file it uses comes from the app's own origin

### Requirement: Two themes
The app SHALL have two themes, Daybreak (light) and Porch light (dark), built from the same set of color roles:
- ground, surface, line;
- ink and muted ink;
- the UI color (haint blue) and its tint;
- light (amber), and a text-safe light;
- error (terracotta).

In both themes:
- Text and controls SHALL meet WCAG AA contrast: 4.5:1 for normal text and 3:1 for large text, focus indicators and control boundaries.
- Primary buttons, links and focus rings SHALL use the UI color.
- Amber SHALL be used for light (what's new, fresh or thinking), never as the fill of a button.
- No information SHALL be carried by color alone.
- The browser's own controls (scrollbars, form fields, date pickers) SHALL match the theme.

#### Scenario: Contrast is checked
- **WHEN** the color tokens change
- **THEN** a contrast check run in CI fails if any text, control or focus pairing in either theme falls below its WCAG AA ratio

#### Scenario: Primary buttons
- **WHEN** Jordan sees the button that sends a chat message
- **THEN** it's filled with the UI color, not amber, and its label meets AA contrast in both themes

#### Scenario: Porch light is warm
- **WHEN** Sam views his page in Porch light
- **THEN** the ground is the warm walnut of the brief, not blue-black, and every card, form, chat and editor surface uses the dark theme

### Requirement: Each member chooses a theme
Each member SHALL be able to choose Daybreak, Porch light or Automatic, and Automatic SHALL be the default. Automatic SHALL follow the device's light or dark setting, and SHALL change when that setting changes without a reload. The choice SHALL be saved on the server for that member, and SHALL apply on every device they sign in on. A member SHALL only be able to change their own choice, and an invalid choice SHALL be refused. Loading a page SHALL NOT show the other theme first and then switch.

#### Scenario: Choosing Porch light
- **WHEN** Maya picks Porch light on her laptop and later signs in on her phone
- **THEN** both show Porch light, whatever each device's setting is

#### Scenario: Automatic follows the device
- **WHEN** Jordan's choice is Automatic and his phone switches to dark mode
- **THEN** the app switches to Porch light without a reload

#### Scenario: No flash on load
- **WHEN** Sam, who chose Porch light, reloads his page
- **THEN** the page is drawn in Porch light from the first paint

#### Scenario: Someone else's choice
- **WHEN** a request tries to set another member's theme, or sets a theme that doesn't exist
- **THEN** it's refused, and no member's theme changes

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
