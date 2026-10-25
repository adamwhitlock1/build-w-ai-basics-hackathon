## Why

Kids have their family agent; parents have only buttons. A parent wants to ask things: "what does everyone have tomorrow?", "find a good beginner robotics kit and where to buy it", and, most of all, "anything I should know about Lily's chats this week?". The review queue shows flags one by one. An assistant that can read the family's day, research the web, and summarize the kids' chats gives parents the overview, and keeps the kids' words on the same no-retention providers that kid chat uses.

## What Changes

- **An Assistant for each parent.** It's a page in the parent's view, with conversations private to the parent who had them: the other parent can't read them. Kids can't use it. Replies stream in, and tool use shows as notes, as in kid chat. There's no safety pre-check on parents' messages.
- **Two kinds of conversation, chosen at the start:**
  - **Ask anything** runs on the main OpenRouter key. Its tools:
    - **web research**: web and news search with no kid filter, several searches per message, citing sources with clickable links;
    - **the family's day**: the parent's own calendar and each family member's, today and tomorrow, read-only;
    - **weather** for the parent's location;
    - **picks**: the parent's interests and page picks.
  - **About the kids** runs entirely on the kids' key with zero-retention routing, because it reads what the kids wrote. It has every tool above, plus **the kids' chats**: the review queue, a kid's recent conversations and their flags, and a conversation's transcript. It's only available when kid chat is set up.
- **Read-only.** The assistant summarizes and answers. Marking flags reviewed, deleting chats and changing settings stay in the app's own buttons.
- **Storage.** Assistant conversations, messages and tool notes get their own tables, kept until the parent deletes a conversation. A parent can delete their own assistant conversations.
- **Usage** is recorded per parent under a new `parent_chat` feature.
- **Kid chat's refusal for parents now points to the Assistant**, instead of saying it isn't available yet.
- **Evaluation and smoke tests.**
  - `make ai-eval-parent` checks web research with links, the family's day, kid summaries that come only from the kids' chats tool, and that "Ask anything" can't read kids' chats.
  - `make ai-smoke` checks a parent search tool call.

## Capabilities

### New Capabilities
- `parent-assistant`: the parents' Assistant: who may use it, privacy between parents, the two conversation kinds and their routing, the tools, read-only behavior, tool notes and links, storage, and usage.

### Modified Capabilities
- `kid-chat`: "Only kids chat with the family agent". A parent asking to use kid chat is now pointed to their own Assistant.

## Impact

- **Backend:**
  - Migration `008`: the `assistant_conversations`, `assistant_messages` and `assistant_tool_calls` tables.
  - `internal/aikit`: a `ParentAgent` with its prompt and tool set. The tool-building code is shared with `KidAgent`.
  - A new `internal/assistant` package: the service, handler and tool sources over the existing services. The kids'-chats tool reads through `chat.Service`'s parent operations.
  - Routes under `/api/me/assistant`, with `main.go` wiring for both Kits and the `PARENT_CHAT_MODEL` setting.
- **Frontend:** an Assistant page and conversation view for parents, an "Assistant" link in the sidebar, and clickable links in assistant replies.
- **Tooling and docs:** `make ai-eval-parent`, a parent check in `make ai-smoke`, and the README.
- **Cost:**
  - Parent searches use Brave credit, up to 4 per message.
  - "Ask anything" runs on the main key.
  - "About the kids" runs on the kids' key.
  - Usage is per parent.
