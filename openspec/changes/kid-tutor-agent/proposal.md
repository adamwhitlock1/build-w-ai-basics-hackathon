## Why

The family agent is the reason for v2. The kids, 12 and 15, should get an AI they can ask anything, which helps them learn rather than doing their homework for them. The parents need to be able to see what it was asked, and to know at once if something is wrong. The groundwork is now in place: services, usage per member, and Genkit in `internal/aikit`. So the first agent can be built, with its safety rules from the start rather than added later.

## What Changes

- **Kid chat.** Kids get a Chat page, and a card on their homepage that leads to it. They can hold titled conversations in two modes:
  - **Homework help**, a tutor in the style of Khanmigo that guides rather than answers. For anything that looks like an assignment, it climbs a hint ladder: what have you tried, then a nudge, then a similar worked example, then checking their answer. It never hands over the final answer.
  - **Explore**, a curiosity and creativity companion for questions, ideas, stories and projects.

  The voice and reading level follow the kid's age: a younger persona up to 12, a teen persona from 13 to 17. Every chat screen shows "Your parents can review these chats". Replies stream in as they're written.
- **Kid ages.** Parents can set a kid's birth year in the profile panel, which picks the persona. Without one, the younger persona is used.
- **Pre-flight safety check.** Every kid message is first classified by a fast, small model, which returns structured flags: category, severity, and possible crisis. A flag never blocks the message; it goes to the parents' review queue.

  For a possible crisis, the reply is told to respond with care and to encourage talking to a trusted adult. The reply always ends with fixed crisis resources added by the server: call or text 988, or text HOME to 741741. If the check itself fails, the message still goes through and is flagged as "couldn't check".
- **Parent review.**
  - Parents get a Reviews page: a queue of flagged messages, crisis first, each with its full conversation, to mark as reviewed.
  - They can also browse and read every conversation of each kid, and delete one.
  - While a crisis flag is unreviewed, a prominent alert shows on every parent page. Phone push comes later.
- **No-retention gate.**
  - Kid traffic uses its own OpenRouter key, `OPENROUTER_KIDS_API_KEY`. Every kid call, the replies and the safety check alike, is routed only to zero-data-retention endpoints that don't collect data.
  - Without the key, kid chat is off. Kids see "Chat isn't set up yet", and nothing is sent anywhere.
  - A development-only override, with a startup warning, allows other models for prototyping with the demo family.
- **Storage.** Conversations, messages and flags are stored in SQLite, and kept until a parent deletes them. Token usage is recorded per kid, under a new `kid_chat` feature, with no content in the usage table.
- **Evaluation.** `make ai-eval-kids` is an opt-in script that runs scripted kid conversations against the real models and checks the behavior:
  - homework questions get hints, not answers;
  - risky and crisis messages are flagged;
  - crisis replies carry the resources.

  It never runs in CI.
- **Demo family.** Jordan and Lily get birth years. The demo gains sample kid conversations, including a flagged one, so the review queue can be tested without spending anything.

Out of scope, covered later: tools for the agent (`agent-tools`), the parent's own assistant (`parent-agent`), phone push, cost ceilings, long-term memory, and homework photos.

## Capabilities

### New Capabilities
- `kid-chat`: kids' conversations with the family agent: the two modes and the hint ladder, age personas, titles, streaming replies, the parent notice, storage, and the no-retention gate.
- `kid-chat-safety`: the pre-flight check on every kid message, its flags, crisis handling with fixed resources, and failing open while still flagging.
- `parent-review`: the parents' review queue, reading and deleting kids' conversations, and the unreviewed-crisis alert.

### Modified Capabilities
- `demo-data`: demo kids gain birth years, and the demo family gains sample conversations and a flagged message for testing the review queue.

## Impact

- **Backend:**
  - Migration `006`: kids' birth years, plus the `chat_conversations`, `chat_messages` and `chat_flags` tables.
  - A new `internal/chat` service package (conversations, the safety check, replies) built on a kids' `aikit.Kit`.
  - New routes under `/api/me/chats` for kids, `/api/members/{id}/chats` for parents, and `/api/family/reviews` and `/api/family/alerts` for parents.
  - A server-sent events reply stream.
  - `main.go` builds a second Kit from `OPENROUTER_KIDS_API_KEY`.
  - New variables: `OPENROUTER_KIDS_API_KEY`, `KIDS_CHAT_MODEL`, `KIDS_SAFETY_MODEL` and `PORCHLIGHT_KIDS_ALLOW_RETENTION`, the last for development only.
- **`internal/aikit`:**
  - Per-call OpenRouter routing (zero data retention, no data collection).
  - Streaming generation with usage tracking.
  - Structured classification on the scripted test model.
- **Frontend:**
  - The Chat page and the homepage chat card for kids.
  - The Reviews page, the kids' conversation lists and the crisis alert for parents.
  - The birth year field in the profile panel.
- **Tooling and docs:** `make ai-eval-kids`, live smoke checks for the kids' Kit, a README section on kid chat, safety and the gate, and seed data in `cmd/seed`.
- **Cost:** two model calls per kid message, a small one for the safety check and the reply itself, on paid zero-retention models. They're tracked per kid in `llm_usage`, with no ceiling yet.
