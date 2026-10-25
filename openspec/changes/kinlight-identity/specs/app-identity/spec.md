## Purpose

Gives the app one recognizable identity everywhere a family member meets it: the Kinlight name, the lantern mark, its typefaces, and a light and a dark theme that each member can choose, all readable in both themes.

## ADDED Requirements

### Requirement: The app is called Kinlight
Everything a family member reads SHALL name the app **Kinlight**:
- the browser tab title, the header, and the sign-in, sign-up and invite pages;
- the label on the agents' messages in kid chat and the Assistant;
- error messages and hints;
- empty-state prompts.

When the AI is told what app it's part of, it SHALL be told Kinlight, and replies SHALL NOT call the app Porchlight. Changing the name SHALL NOT change any API path, session, stored data or setting: existing sessions, environment variables and the database SHALL keep working unchanged.

#### Scenario: Signing in
- **WHEN** Maya opens the sign-in page
- **THEN** the tab title and the page say Kinlight, and "Porchlight" appears nowhere on it

#### Scenario: An invite
- **WHEN** Grandma opens her invite link
- **THEN** the page invites her to join the Rivera family on Kinlight

#### Scenario: Kid chat
- **WHEN** Jordan asks the kid agent what it's called
- **THEN** its replies are labeled Kinlight, and it calls itself Kinlight's family helper

#### Scenario: An existing session survives the rename
- **WHEN** Sam was signed in before the update and opens the app after it
- **THEN** he is still signed in and his page, widgets and chats are unchanged

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
