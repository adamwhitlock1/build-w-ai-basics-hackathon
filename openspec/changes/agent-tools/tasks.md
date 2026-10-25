For work in `internal/aikit`, load the `developing-genkit-go` skill first; the design wins where they differ. Run every Go command with `CGO_ENABLED=0 GOTOOLCHAIN=local` (or through `make`). Commit after each task.

## 1. Data

- [x] 1.1 Add `migrations/007_chat_tool_calls.sql` and the store methods to add tool calls, attach them to a reply, and list a conversation's tool calls. Deleting a conversation removes them. Verify with a `store/chat_test.go` test: add, attach, list in order, cascade on delete, and scoping by conversation.
- [x] 1.2 Add `widgets.RememberedPicks(ctx, results, userID)`, which returns the remembered articles and events items read-only (no search, no write), with empty lists when nothing is remembered. Verify with a `widgets` test: remembered items come back; with nothing remembered, empty lists; no source is called.
  - As built: `RememberedPicks(ctx, results, member, profile)`. It takes the member and profile because, like the cards and the briefing, picks count only for the current profile and the member's audience. The read-and-check logic is shared with the briefing in `rememberedWidgetItems`.

## 2. Tools in `internal/aikit`

- [x] 2.1 Add `aikit.KidTools` and `ToolNote`, and build the dynamic tools in `KidAgent.Reply`:
  - `ai.NewTool` per non-nil function, passed with `ai.WithTools` and `ai.WithMaxTurns(4)`;
  - outputs wrapped as untrusted data (design decision 2);
  - tool failures returned as `{"error": ...}`;
  - the search cap of 2;
  - `OnTool` called per use;
  - no retry once a tool has run.

  Add the tools block to `kidSystemPrompt`, listing only the offered tools. Tests with `aitest`:
  - a scripted tool request runs the tool, and its wrapped result reaches the next model request;
  - search runs at most twice, and a third is refused without calling `Search`;
  - a failing tool still gives a reply;
  - `OnTool` gets each use;
  - the Homework prompt doesn't mention search, and the Explore one does;
  - usage is recorded per model call in the loop;
  - a 429 after a tool ran isn't retried.

  Verify with `go test ./internal/aikit/...`.

## 3. Chat service and HTTP

- [x] 3.1 Add `chat.Tools` (the interface) and `chat.ToolSources` (the implementation over `Agenda`, the weather source, `RememberedPicks` and Brave Web with `sources.Kids`). In `Send`:
  - build `aikit.KidTools` per message, capturing the kid and the time zone, with `Search` only in Explore;
  - stream each note through a new callback;
  - store the notes with the reply (or with no reply when it failed).

  `Transcript` and `Get` return the tool calls. Tests with fakes:
  - Explore offers search, and Homework doesn't;
  - the tools read only the sending kid's data (a fake `Tools` records which member each call was for);
  - notes are stored with the reply, and kept when the reply fails;
  - transcripts include them.

  Verify with `go test ./internal/chat/`.
- [ ] 3.2 HTTP: `POST /api/me/chats/{id}/messages` accepts `timeZone`, and streams a `tool` event for each note before the reply's text. Kid and parent transcripts include `toolCalls`. Wire `ToolSources` in `main.go`. Route tests: the SSE sequence `message, tool, delta…, done` with a fake agent using a tool; the stored notes show in the kid's and the parent's transcripts; an unknown `timeZone` falls back to UTC. Verify with `make check`.

## 4. Frontend

- [ ] 4.1 Send `timeZone` (`Intl.DateTimeFormat().resolvedOptions().timeZone`) with each message. Handle the `tool` event, showing the notes live above the streaming reply. Show stored tool notes above their replies in the kid's conversation and the parent's transcript, with search queries. Verify with `npm run lint` and `npm run build`, and in the browser on the demo with the override: as Jordan, "do I need a jacket today?" shows "Checked the weather" and a forecast-based answer, and after a reload the note is still there; as Maya, the transcript shows the note.

## 5. Evaluation, smoke test and docs

- [ ] 5.1 Give `cmd/evalkids` real tool sources: a fixed fake weather, calendar and picks, plus real kid-safe Brave search. Add cases:
  - "do I need a jacket today?" uses the weather tool;
  - "what do I have after school?" uses `my_day`;
  - "what's new with the Artemis mission?" in Explore searches at most twice;
  - a Homework help question doesn't search;
  - an injection planted in the fake picks doesn't break the rules.

  Run it, and record the results here.
- [ ] 5.2 Extend `make ai-smoke`'s kids check with one tool call through the NoRetention Kit on the kids' default model. Run it, and record the results here.
- [ ] 5.3 Update the README's Kid chat section: the tools, Explore-only search and its cap, the notes kids and parents see, and the cost. Update `openspec/roadmap.md`: `agent-tools` is the kids' tools, and the adult tools move to `parent-agent`. Verify by reading both.

## 6. End to end

- [ ] 6.1 On the demo with real keys (the override, unless a kids' key exists), in the browser:
  - Jordan: a weather question, a "my day" question, and an Explore current-events question each show the right note and a sensible answer;
  - Jordan in Homework help: no search note;
  - Maya: Jordan's transcript shows the notes and search queries;
  - `llm_usage` has a row for each round of the tool loop.

  Record the results here.
- [ ] 6.2 Run `openspec validate agent-tools --strict` and `make check`. Push, open the PR, and confirm CI passes on it. Add the "Agent tools" line to "Post-PoC Additions" in `devpost/prd.md` and `devpost/spec.md`.
