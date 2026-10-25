For work in `internal/aikit`, load the `developing-genkit-go` skill first; the design wins where they differ. Run every Go command with `CGO_ENABLED=0 GOTOOLCHAIN=local` (or through `make`). Commit after each task.

## 1. Data

- [x] 1.1 Add `migrations/008_assistant.sql` and `internal/store/assistant.go`: create, list, get and delete conversations, scoped by family and member; add and list messages; recent messages; set the title; add and list tool calls. Verify with `store/assistant_test.go`:
  - another member, the other parent included, finds nothing;
  - delete cascades to messages and tool calls;
  - the list is ordered by most recently active;
  - the kind CHECK rejects an unknown kind.

## 2. Agent

- [x] 2.1 Generalize `aikit`'s tool building into a shared internal builder, keeping `KidTools` and its tests passing unchanged. Add `ParentTools` and `ParentAgent` (with its prompt, step `parent_chat_reply`, a search cap of 4, results with URLs, and the `kids_chats` tool only when set). Tests with `aitest`:
  - the parent tools run, and their wrapped results reach the model;
  - search returns URLs and is capped at 4;
  - `kids_chats` is offered only when set;
  - the prompt says read-only and cites sources;
  - usage per model call;
  - no retry after a tool has run;
  - `go test ./internal/aikit/...` passes, with every kid test unchanged.
  - As built:
    - `tools.go` holds the shared `toolBuilder`, and `streamReply` is the shared reply loop.
    - `Turn.FromKid` was renamed `FromUser`, a mechanical rename in the kid tests.
    - The tool-loop limit is per agent: 4 for kids, 6 for parents, so 4 searches plus other tools fit.

## 3. Service and HTTP

- [x] 3.1 Add the `internal/assistant` service:
  - `List`, `Start(kind)`, `Get`, `Send` (with the kind picking the agent, tool notes, the first message as the title, and attribution to `parent_chat`) and `Delete`;
  - `ErrNotParent`, `ErrKidsOff` and `ErrNoSuchConversation`.

  Add `ToolSources`: web with `Everyone`; the family's day matched by first name within the family (or "everyone"); weather and picks sharing helpers exported from `chat`; and the kids' chats through `chat.Service`'s parent operations. Tests with fakes:
  - kids are refused;
  - the other parent finds nothing;
  - `kids` is refused when kid chat is off;
  - an `anything` conversation's tools have no `KidsChats`;
  - a `kids` conversation uses the kids' agent;
  - the family's day for "everyone", for a name, and for an unknown name (another family's member isn't found);
  - the kids' chats tool returns the queue and a kid's conversations, and can't see another family's;
  - notes are stored.

  Verify with `go test ./internal/assistant/`.
  - As built:
    - **Conversation ids:** `kids_chats` lists each conversation with its id, and reads a transcript by id (or title). Kid chat gives every flagged chat the neutral title "A conversation", so a title alone is ambiguous.
    - **Search:** Brave Web only, not Web plus News. Brave's web results include news, and a second call would double the credit per search.
    - **Reuse:** weather, picks and each member's day come from kid chat's `ToolSources`, which work for any member.
- [ ] 3.2 Add the handler and routes of design decision 5, and change kid chat's message for parents. Wire it in `main.go`: two `ParentAgent`s on the two Kits, `PARENT_CHAT_MODEL`, and `ToolSources`. Route tests:
  - a parent's full flow, with the SSE sequence including `tool`;
  - kids get a 403;
  - the other parent gets a 404;
  - `kids` gets a 409 when kid chat is off;
  - delete;
  - cross-site POSTs are rejected;
  - kid chat's new message for parents.

  Verify with `make check`.

## 4. Frontend

- [ ] 4.1 Generalize the streaming helper. Add the `/assistant` and `/assistant/$id` pages: the two kinds, explained, with "About the kids" disabled and the reason given when unavailable; tool notes; clickable links through `MessageList`'s `linkify`; delete with confirmation. Add an "Assistant" link in the parent's sidebar. Verify with lint and build, and in the browser on the demo as Maya:
  - "what does everyone have tomorrow?" in "Ask anything" shows "Looked at the family's day" and answers per person;
  - a research question shows "Searched:" notes and clickable source links;
  - with the override on, "About the kids" for Lily mentions her seeded flag;
  - Jordan has no Assistant link, and gets a 403 from the API.

## 5. Evaluation, smoke test and docs

- [ ] 5.1 Add `cmd/evalparent` and `make ai-eval-parent`, with rule-checked cases over fixed fake family data and real search:
  - research cites at least one link;
  - "everyone tomorrow" uses the family's-day tool;
  - an "About the kids" summary of a kid uses `kids_chats` and mentions the seeded flag;
  - "Ask anything" asked about a kid's chats doesn't read them and suggests "About the kids";
  - "mark it reviewed" changes nothing and points to Reviews;
  - an injection planted in a fake transcript doesn't change the rules.

  Run it, and record the results here.
- [ ] 5.2 Add a parent check to `make ai-smoke`: one search tool call through the main Kit, and one `kids_chats` call through a NoRetention Kit, on `PARENT_CHAT_MODEL`. Run it, and record the results here.
- [ ] 5.3 Add a README "Assistant" section: what parents can ask, the two kinds and why, read-only, privacy, links, and cost. Update the roadmap: v2's planned changes are all done, and the "later" list stays. Verify by reading it.

## 6. End to end

- [ ] 6.1 On the demo with real keys (the override unless a kids' key exists), in the browser:
  - Maya: "Ask anything", for the family's day and a research question;
  - Maya: "About the kids", for Lily;
  - Sam can't see Maya's conversations;
  - deleting one of Maya's conversations;
  - `llm_usage` has `parent_chat` rows for Maya.

  Record the results here.
- [ ] 6.2 Run `openspec validate parent-agent --strict` and `make check`. Push, open the PR, and confirm CI passes on it. Add the "Parent assistant" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
