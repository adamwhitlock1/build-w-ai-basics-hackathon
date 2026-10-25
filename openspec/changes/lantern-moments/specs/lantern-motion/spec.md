## Purpose

Gives Kinlight's lantern its behavior: light warms up rather than switching on, marks what's alive or new, points at what changed and then gets out of the way, and never moves for anyone who has asked their device for less motion.

## ADDED Requirements

### Requirement: Kindle
When the app is opened in a browser tab, and right after a member signs in, the header lantern SHALL kindle: its glass warms from dark to amber over about 900 ms, with at most one small flicker, while its glow spreads. Moving between pages in the same tab SHALL NOT kindle it again.

#### Scenario: Opening the app
- **WHEN** Maya opens Kinlight in a new tab
- **THEN** the header lantern kindles once, then stays lit

#### Scenario: Moving around
- **WHEN** Maya goes from her page to Reviews and back
- **THEN** the lantern stays lit and doesn't kindle again

#### Scenario: Signing in
- **WHEN** Jordan signs in
- **THEN** the lantern on his page kindles once

### Requirement: Thinking breathes
Whenever the app is waiting, it SHALL show one breathing light: a small lantern whose light brightens and settles every 2.8 seconds, beside words saying what's happening. This covers:
- the AI writing a chat reply, an Assistant reply, a briefing or a described card;
- a page or a member's page loading;
- a form saving;
- a card refreshing.

No spinner or pulsing skeleton SHALL remain. Breathing SHALL be the only animation in the app that repeats. A wait SHALL always say what it's waiting for in text, and screen readers SHALL hear that text, not the animation.

#### Scenario: A chat reply
- **WHEN** Jordan sends a message and the reply hasn't started
- **THEN** a breathing light shows with "Thinking…" until the first words arrive

#### Scenario: A briefing being written
- **WHEN** Maya's briefing is being written
- **THEN** the briefing card shows a breathing light and "Writing your briefing…", with no pulsing placeholder rows

#### Scenario: Saving
- **WHEN** Sam submits the sign-in form
- **THEN** the button shows a breathing light and its busy label instead of a spinner

### Requirement: A card catches the light when it arrives
When a card is added to a page, or a card's content changes while the page is open (a refresh, or a briefing that finishes), the header lantern SHALL swing a few degrees and settle, and that card's top edge SHALL catch the light and then cool to normal over about 1.2 seconds. Cards that are already there when the page loads SHALL NOT catch. A new card SHALL still receive focus, as it does today, so it's announced and scrolled into view.

#### Scenario: Describing a new card
- **WHEN** Maya describes a new card and it appears
- **THEN** the lantern swings, the new card's top edge lights and cools, and the card has focus

#### Scenario: Refreshing a card
- **WHEN** Maya refreshes her Articles card and new articles land
- **THEN** that card's top edge catches the light, and no other card's does

#### Scenario: Opening a page
- **WHEN** Maya opens her page with six cards on it
- **THEN** no card catches the light and the lantern doesn't swing

### Requirement: Dusk
Changing between Daybreak and Porch light SHALL be shown as the porch light coming on or going off. Going to Porch light, night SHALL spread outward from the header lantern; going back to Daybreak, the light SHALL draw back in. It SHALL take about 1.1 seconds. It SHALL happen when a member changes the theme with the switch, or when Automatic follows a change in the device's setting, and SHALL NOT happen when a page loads or when a member signs in. Where the browser can't animate the change, the theme SHALL change at once.

#### Scenario: Choosing Porch light
- **WHEN** Sam picks Porch light in the evening
- **THEN** the dark theme spreads out from the header lantern across the page

#### Scenario: Loading a page
- **WHEN** Sam, who chose Porch light, opens the app
- **THEN** the page is Porch light from the first paint, with no dusk animation

### Requirement: One lit thing per screen
At most one element on a screen SHALL glow at full strength at a time. That's the first of:
1. the AI while it's writing;
2. a card that just arrived;
3. the morning briefing card, before noon in the member's time zone.

Freshness edges on cards are dimmer, and don't count. Light SHALL never be the only signal: everything that glows also says what it means in words (a status, a label or an update time). Errors, buttons and decoration SHALL never glow.

#### Scenario: Thinking while a card arrives
- **WHEN** a refreshed card lands while the Assistant is writing a reply on the same screen
- **THEN** only the Assistant's breathing light glows at full strength

#### Scenario: Morning
- **WHEN** Maya opens her page at 8 am and nothing is loading or arriving
- **THEN** the briefing card's top edge is lit, and nothing else glows at full strength

### Requirement: Reduced motion
When the device asks for reduced motion, every moment SHALL become a plain fade of the same length:
- no swing, flicker, spreading or movement;
- the kindle is a fade-in of the lantern's light;
- the dusk is a cross-fade between themes;
- the breathing light fades its brightness without moving.

Every animation SHALL change only opacity and transform, with clip-path allowed for the dusk alone.

#### Scenario: Reduced motion on
- **WHEN** Lily's tablet has reduced motion turned on and a new card arrives
- **THEN** the card's edge fades in and out, and the lantern doesn't swing
