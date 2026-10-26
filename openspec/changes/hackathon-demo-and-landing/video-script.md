# Kinlight demo video script

An annotated draft for the 1–3 minute Devpost video. It's planned to run about **2:20**. The narration is a draft for you to say in your own words ([video-prep.md](video-prep.md) asks for that). Each scene lists what's on screen, what you do, what you say, and notes for the edit.

Everything shown comes from the hosted judge demo's fictional Rivera family ([docs/judge-demo.md](../../../docs/judge-demo.md)). Nothing comes from your real family.

## Before you record

- **Reset the demo** so the chores, chats and reviews are fresh. Check that the allowance is high enough for two or three rehearsals plus the take.
- **Use two browser windows, already signed in.** Use a separate profile for each so the sessions don't mix:
  - **Window A:** Maya (`maya@rivera.test`), the parent, on her homepage.
  - **Window B:** Jordan (`jordan@rivera.test`), age 15, on his homepage.
- **Keep passwords, bookmarks and notifications off screen.** Record at 16:9, with the browser zoom set to about 110–125% so cards read on a phone.
- **Rehearse the two live AI moments** (scenes 3 and 5) with the exact prompts below. Keep prompts that work. Model output varies from run to run, so don't promise identical words.
- **Pick a theme.** Dusk (dark) looks best for the lantern glow. Use the same theme in both windows.

## Scenes

### 1. The problem: 0:00–0:15

**Screen:** The signed-out landing page, then cut to Maya's homepage.

**Do:** Hold on the landing page for about 3 seconds, then cut to Maya's populated page.

**Say:**
> Every family runs on a dozen apps: the school calendar, the weather, the chore chart, and now AI chat for the kids. Kinlight puts it on one page per person, and every card on it is built by AI from a sentence.

**Notes:** The opening should explain who it's for before any feature appears. If the landing page isn't ready, open straight on Maya's page.

### 2. A page that's already done its homework: 0:15–0:35

**Screen:** Maya's homepage, top to bottom.

**Do:** Let the morning briefing sit on screen for a beat, then slowly scroll past Today, the weather card, Rocket Launches and Weeknight Dinner Ideas.

**Say:**
> This is Maya's page. The briefing up top is written by AI each morning from her calendar, the weather and the news she cares about. Below it are her day, her forecast, and cards picked from her interests: rocket launches and easy dinners.

**Notes:**
- The briefing is the one lit card (the lantern rule: one glowing thing per screen). Let the glow register before scrolling.
- Every card shows when it was updated. You don't need to say it; viewers will see it.
- The demo content is fictional. Don't call it real news.

### 3. Describe a card: 0:35–1:05 ★ live AI

**Screen:** Maya's page, the "describe a card" field, then the new card arriving.

**Do:**
1. Type **"show me stargazing events near Denver this month"**, or another prompt that worked in rehearsal.
2. Wait for the card. The lantern "thinking" glow breathes while the AI works.
3. Press **Arrange**, drag the new card up beside the briefing, widen it to half, then press **Done**.

**Say** (while it's working):
> This is the original idea. I describe what I want in plain English. Behind the scenes, the AI picks a data source and a search, runs it on Brave, then turns only those results into the card. My API keys never leave the server.

**Say** (as you arrange):
> And I can put it wherever I want on my page.

**Notes:**
- This is the core "describe-to-interface" moment, so keep the real result on screen for at least 2 seconds.
- If the wait is long, cut it and add a small "sped up" or "wait trimmed" caption. Don't hide that it was trimmed.
- Backup prompt: **"weather for Breckenridge this week"**. It uses Open-Meteo, so no paid search.

### 4. Chores, across two people: 1:05–1:25

**Screen:** Window A (Maya), then Window B (Jordan), then back to A.

**Do:**
1. **Maya:** assign Jordan a chore, for example **"Take out the recycling"**.
2. **Switch to Jordan's window:** the chore is on his checklist. Check it off.
3. **Back to Maya:** family progress shows that Jordan has done it.

**Say:**
> Parents assign chores, and they show up on each kid's own page. Jordan checks it off on his phone, and Maya sees it without having to ask. That's the kind of nagging I wanted to take out of our house.

**Notes:**
- Switching windows makes it clear these are two real accounts, not a parent "viewing as" a kid.
- Personalize the last line if it's true for you. Otherwise, cut it.

### 5. A tutor, not an answer key: 1:25–1:55 ★ live AI

**Screen:** Window B (Jordan), the family agent's chat.

**Do:**
1. Start a new conversation in **Homework help** mode.
2. Type **"just tell me the answer to 3x − 5 = 16"**.
3. Let the reply stream in. It should ask for the first step instead of answering.
4. Optional, if there's time: reply **"add 5?"** and show it confirm the step.

**Say:**
> Each kid gets their own AI. In homework mode it's a tutor. It won't hand Jordan the answer; it walks him up one hint at a time. It talks to him at his age, every message gets a safety check first, and kids' chats only run on providers that keep nothing.

**Notes:**
- Keep the parent notice ("parents can see these chats") visible in the frame. It's part of the design, not fine print.
- If the reply gives the answer anyway, retake it. Don't use a take that contradicts the narration.
- Don't open the seeded "Sample: Solving 2x + 7 = 19" chat and present it as live. If you show it, say it's a sample.

### 6. Parents stay in the loop: 1:55–2:10

**Screen:** Window A (Maya), the **Reviews** page.

**Do:** Open Reviews. Show Lily's flagged item: someone in a drawing game asked where she lives. Open the transcript so the agent's reply is visible.

**Say:**
> And parents stay in the loop. When Lily, who's nine, mentioned that someone in a game asked where she lives, the agent told her to keep that private and talk to a parent, and it flagged it here for Maya. Possible crises go to the top, with resources for the kid right away.

**Notes:**
- **Required honesty:** this conversation is seeded. Put a small on-screen caption, "Sample conversation", or say "here's an example".
- Don't show a fake crisis. Describing how crises are handled is enough.

### 7. Close: 2:10–2:25

**Screen:** Back to Maya's full page, then the landing page, or an end card with the demo URL.

**Do:** Hold on the finished page with the new card in place. End on the URL.

**Say:**
> Kinlight runs on a home server for my own family, with a demo you can try at the link below. The biggest thing I learned building it was _[your one concrete lesson. Possible starting points: writing the spec before the code; designing the kid tutor's hint ladder; how much of "safe for kids" is product decisions, not just filters]_. Thanks for watching.

**Notes:** The rules ask what you learned, so this line matters. Say something true and specific.

## Timing at a glance

| Scene | Time | Live AI | Can cut if long |
| --- | --- | --- | --- |
| 1. The problem | 0:00–0:15 | | |
| 2. Homepage tour | 0:15–0:35 | | Shorten to 10 s |
| 3. Describe a card | 0:35–1:05 | ✔ | Trim the wait (with a caption) |
| 4. Chores | 1:05–1:25 | | |
| 5. Homework tutor | 1:25–1:55 | ✔ | Skip the optional second turn |
| 6. Reviews | 1:55–2:10 | | |
| 7. Close | 2:10–2:25 | | |

The narration is about 330 words, which fits in 2:20 at a relaxed pace, including the AI waits. If a take runs over 3:00, cut from scene 2 first, then the arranging in scene 3.

## Features deliberately left out

These features are real but don't fit in the time. Mention them in the Devpost write-up instead:
- the parent's private **Assistant**, which researches and summarizes the kids' chats, read-only;
- the **calendar** links and the Today card;
- kid-safe content marking;
- usage recorded per member;
- the light and dark themes, and reduced motion;
- the homelab deployment (Coolify, Cloudflare Tunnel and Litestream backups).
