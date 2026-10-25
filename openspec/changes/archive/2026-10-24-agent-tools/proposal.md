## Why

The kid agent can only talk. It doesn't know the kid's day, the weather where they live, or anything newer than its training. A curious 12-year-old asking "what's happening with the Artemis launch?" gets a guess. Porchlight already gathers exactly this information for each kid's page, with kid-safe filtering. Giving the agent a small, safe set of tools makes it useful in the moment without opening it up to the whole web or to anyone else's data.

## What Changes

- **Four tools for the kid agent**, each acting only for the kid who is chatting:
  - **Weather:** the forecast for the kid's saved location, from the same free forecast source as their page.
  - **My day:** their own calendar's events for today and tomorrow, the same as their Today card. Read-only.
  - **My picks:** their saved interests, plus the Articles and "Happening near you" results already on their page. It runs no new search.
  - **Kid-safe search:** web search with strict safe-search, **in Explore only**. Never in Homework help, so it can't fetch answers. At most **2 searches per message**. The agent summarizes what it finds in its own words and doesn't quote pages at length.
- **Tool use is visible.**
  - The kid sees a small note in the chat as each tool is used, such as "Checked the weather" or "Searched: volcano facts".
  - Parents see the same notes in the transcript, with each search's exact query.
  - Tool use is stored with the conversation.
- **Tool results are treated as data.** Text from the web or a calendar can't change the agent's rules, and the agent is told so.
- **Time zone.** The kid's time zone travels with each message, so "today" means their today.
- **Usage.** Every model call in a tool loop is recorded under kid chat, as now. A search costs search credit, like a widget refresh.
- **Adult tools move to `parent-agent`.** Web research and the like arrive with the parent assistant that will use them. The roadmap is updated.

## Capabilities

### New Capabilities
- `kid-chat-tools`: the kid agent's tools, and who they may act for; search's limits; how tool use is shown to kids and parents and stored; and tool results as untrusted data.

### Modified Capabilities
None. Kid chat, the safety check and parent review keep their requirements. The new capability adds the tool notes to the transcripts they show.

## Impact

- **Backend:**
  - Migration `007`: a `chat_tool_calls` table.
  - `internal/aikit`: a kid toolset passed to `KidAgent.Reply`, which builds Genkit dynamic tools per call. It also handles the per-message search cap, the untrusted-data wrapping, the tool notes, and the tools part of the system prompt.
  - `internal/chat`: builds the toolset from the existing services (Agenda, the weather source, remembered picks, Brave with kids' safe-search), stores tool calls, streams a `tool` event, and returns tool calls in transcripts.
  - `internal/widgets`: a read-only way to get a member's remembered picks.
  - `POST /api/me/chats/{id}/messages` takes an optional `timeZone`.
- **Frontend:** tool notes in the kids' chat, live as they happen and after a reload, and in the parents' transcripts, with search queries.
- **Tooling and docs:**
  - `make ai-eval-kids` gains cases for tool use, for no search in Homework help, and for the search cap.
  - `make ai-smoke` gains a tool-calling check for the kids' model under zero-retention routing.
  - The README is updated.
- **Cost:** up to 2 Brave searches per Explore message that needs one, plus an extra model call per tool round. Both are visible per kid in usage.
- **Roadmap:** `openspec/roadmap.md`'s `agent-tools` and `parent-agent` entries.
