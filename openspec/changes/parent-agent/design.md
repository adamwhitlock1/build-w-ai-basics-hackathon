## Context

See proposal.md for why. What exists on `main`:
- **Kid chat:**
  - `internal/chat` holds the service, handler and `ToolSources`. It's built on `aikit.KidAgent`.
  - `aikit.KidTools` turns tools into Genkit dynamic tools per reply.
  - Its pieces:
    - the untrusted-data wrapping;
    - the tool notes;
    - the search cap;
    - no retry once a tool has run;
    - `Kit.stream` with the call config, which carries the no-retention routing on the kids' Kit.
  - Tables: `chat_conversations` (mode CHECK homework/explore), `chat_messages`, `chat_tool_calls` and `chat_flags`.
- **Two Kits in `main.go`:** the main one (main key, used for the briefing) and the kids' one (`OPENROUTER_KIDS_API_KEY`, `NoRetention`, nil when kid chat is off).
- **Parent reads of kid chat** are `chat.Service.Reviews`, `ListFor`, `Transcript` and `Alerts`, already permission-checked and scoped to the family.
- **The SQLite constraint:** `chat_conversations.mode` has a CHECK constraint. Widening it needs a table rebuild, and with `foreign_keys(1)` on, dropping the old table would cascade-delete `chat_messages` and `chat_flags`.

## Goals / Non-Goals

**Goals:**
- One tool-building path in `aikit`, shared by kids and parents, so the untrusted-data wrapping, notes, caps and retry rule can't drift apart.
- Kid data reaches only the kids' Kit. That's guaranteed by construction: the kids'-chats tool simply isn't built for an "Ask anything" conversation.
- Each parent's conversations are private by query: every lookup is scoped to the family and to the member.

**Non-Goals:**
- Actions: no tool writes anything.
- Sharing conversations between parents.
- Model-written titles for parent chats (the first message is enough), or a safety check on parents' messages.
- Long-term memory, or proactive summaries ("weekly digest"), which come later.

## Decisions

### 1. Separate tables for the Assistant
Migration `008_assistant.sql` adds three tables:
- `assistant_conversations`: `id`, `family_id`, `member_id`, `kind` (CHECK `anything` or `kids`), `title`, `created_at`, `updated_at`;
- `assistant_messages`: `id`, `conversation_id` (ON DELETE CASCADE), `role` (CHECK `parent` or `agent`), `text`, `created_at`;
- `assistant_tool_calls`: like `chat_tool_calls`.

Every store query takes both the family and the member.
- *Alternative:* widen `chat_conversations` with a table rebuild. That's rejected above. The two features also have different rules (flags and reviews against private conversations), and separate tables keep the code honest about that.

### 2. `aikit`: one toolset type, two agents
`KidTools` generalizes into an internal `toolset` builder that both agents use. The kid-facing API stays `KidTools`. `ParentTools` adds:

```go
type ParentTools struct {
    Search    func(ctx, query string) ([]SearchResult, error) // with links
    FamilyDay func(ctx, who string) (string, error)          // "" or "everyone", or a member's name
    Weather   func(ctx) (string, error)
    MyPicks   func(ctx) (string, error)
    KidsChats func(ctx, KidsChatsQuery) (string, error)        // nil in "Ask anything"
    OnTool    func(ToolNote)
}
```

- **The search cap** is 4 for parents.
- **Parent search results** keep their `url`, so replies can cite sources.
- **`kids_chats`** takes `{kid, conversation}`. Both are optional names:
  - no kid: the family's review queue and each kid's recent conversations;
  - a kid: that kid's conversations and flags;
  - a kid and a conversation title: that transcript, capped at the last 40 messages.

`ParentAgent.Reply` mirrors `KidAgent.Reply`: streaming, the tool loop, no retry once a tool has run, and step `parent_chat_reply`. Its system prompt covers:
- whom it serves;
- that it's read-only (and where in the app actions live);
- source citing;
- tool results as data;
- plain text with bare URLs.

`NewParentAgent(kit, model)` is used twice in `main`: once on the main Kit and once on the kids' Kit.

### 3. Routing by kind
The `assistant.Service` holds two agents:
- **`anything`**, on the main Kit;
- **`kids`**, on the kids' Kit, or nil when kid chat is off.

`Send` picks the agent from the conversation's kind and builds `ParentTools` with `KidsChats` set only for `kids`. Starting a `kids` conversation with no kids' agent is `ErrKidsOff`, which becomes a 409 with a clear message. Because the tool doesn't exist in an "Ask anything" call, no prompt or model can reach the kids' chats there.

### 4. Tool sources over existing services
`assistant.ToolSources`:
- **Web:** Brave Web and News with `sources.Everyone`, results mapped with URLs.
- **The family's day:** `ListFamilyMembers(parent.FamilyID)`, matched by first name (as the UI shows names) or "everyone". Then `Agenda.Day` per member, formatted by person. An unknown name gets "There's no one named X in your family".
- **Weather and picks:** the same as kid chat's `ToolSources`, for the parent. The shared formatting moves to small helpers in `chat`, exported for reuse, rather than being copied.
- **The kids' chats:** `chat.Service.Reviews`, `ListFor` and `Transcript`, with the parent as the caller, so permissions stay in one place. Kids are matched by first name within the family.

### 5. HTTP and frontend
- **Routes**, with `private` and the service checking that the caller is a parent (403 for kids):
  - `GET /api/me/assistant`, which returns `{conversations, kidsAvailable}`;
  - `POST /api/me/assistant` with `{kind}`;
  - `GET /api/me/assistant/{id}`;
  - `POST /api/me/assistant/{id}/messages` with `{text, timeZone}`, streamed as server-sent events (`message`, `tool`, `delta`, then `done` or `error`), exactly like kid chat;
  - `DELETE /api/me/assistant/{id}`.
- **The streaming helper** in the frontend gains a path parameter, so it can be reused.
- **Pages:** `/assistant` (the list, and "Ask anything" or "About the kids" with an explanation) and `/assistant/$id`.
- **The sidebar** gets an "Assistant" link for parents.
- **`MessageList`** gets a `linkify` prop: http(s) URLs become `<a target="_blank" rel="noopener noreferrer">`. It's used only for the Assistant, so kids' replies stay link-free.
- **Kid chat's refusal for parents** becomes "Kid chat is for kids. Try your Assistant.", with the same 403.

### 6. Model
`PARENT_CHAT_MODEL` defaults to `anthropic/claude-haiku-5.5`. It's the same id on both Kits, and it has zero-retention endpoints that don't block, which the kids' Kit needs.

## Risks / Trade-offs

- [A parent asks about the kids in "Ask anything"] → the tool isn't there, the prompt tells the assistant to suggest "About the kids", and the eval checks it.
- [Web research brings in injected instructions] → results are wrapped as data, the assistant is read-only (so there's nothing harmful it could do), and an eval case puts an injection in a fake page.
- [Transcripts are long] → capped at the last 40 messages, with each message cut to 500 characters, and the summary reads flags first.
- [Calendar data of every member goes to the main provider in "Ask anything"] → that's the same exposure the briefing already has (calendar facts go to the main key). Kids' *chats* never do.

## Migration Plan

Migration `008` only adds tables. Without the change, nothing else differs. The Assistant appears for parents on deploy, and "About the kids" only with a kids' key.
