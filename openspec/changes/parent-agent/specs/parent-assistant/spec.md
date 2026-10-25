## Purpose

Gives each parent a private assistant that can research the web, read the family's day, and summarize what the kids have been chatting about, without ever changing anything, and without sending the kids' words anywhere that keeps them.

## ADDED Requirements

### Requirement: Only parents have an Assistant, and it's private
The Assistant SHALL be available only to parents. A kid's request to use it SHALL be refused. Each parent's assistant conversations SHALL be visible only to that parent: another parent in the same family, a kid, or anyone in another family SHALL get an answer as if the conversation doesn't exist. A parent SHALL be able to delete their own assistant conversations.

#### Scenario: A parent opens the Assistant
- **WHEN** Maya opens the Assistant
- **THEN** she sees her own assistant conversations, newest first, and can start a new one

#### Scenario: The other parent's conversation
- **WHEN** Sam asks for one of Maya's assistant conversations by id
- **THEN** the system answers as if it doesn't exist

#### Scenario: A kid tries the Assistant
- **WHEN** Jordan asks to start an assistant conversation
- **THEN** the request is refused, and nothing is stored or sent to any AI model

### Requirement: Two kinds of conversation
Each assistant conversation SHALL have a kind, chosen when it starts:
- **Ask anything** SHALL use the main AI provider key, and SHALL NOT be able to read the kids' chats.
- **About the kids** SHALL send every AI call through the kids' key with the same zero-data-retention, no-data-collection routing as kid chat, and SHALL be the only kind that can read the kids' chats.

When kid chat isn't set up, "About the kids" SHALL be unavailable, and the Assistant SHALL say why. The kind SHALL be shown on the conversation and SHALL NOT change after it starts.

#### Scenario: Asking about the kids
- **WHEN** Maya starts an "About the kids" conversation and asks "anything I should know about Lily's chats this week?"
- **THEN** the answer is based on Lily's flags and conversations, and every AI call for it used the kids' key with zero-retention routing

#### Scenario: Ask anything can't read the kids' chats
- **WHEN** Maya asks about Jordan's chats in an "Ask anything" conversation
- **THEN** the assistant has no way to read them, and suggests starting an "About the kids" conversation

#### Scenario: Kid chat isn't set up
- **WHEN** no kids' key is configured
- **THEN** only "Ask anything" can be started, and the Assistant explains that "About the kids" needs kid chat set up

### Requirement: The Assistant's tools
In both kinds, the Assistant SHALL be able to use:
- **web research:** web and news search, without the kids' filter, at most 4 searches per message;
- **the family's day:** today's and tomorrow's calendar events for the parent or any member of their family, read-only, in the parent's time zone;
- **weather** at the parent's saved location;
- **picks:** the parent's interests and page picks.

In "About the kids" it SHALL also be able to read the kids' chats: the family's review queue (flags with their reasons and messages), a kid's recent conversations with their flag counts, and a conversation's transcript. Every tool SHALL read only the parent's own family, decided by the server and never by the model. Tool results SHALL be treated as information, not instructions.

#### Scenario: The family's day
- **WHEN** Sam asks "what does everyone have tomorrow?"
- **THEN** the assistant reads each family member's calendar for tomorrow, and lists them by person

#### Scenario: Another family
- **WHEN** Maya asks about Alex, a member of another family
- **THEN** no tool can read Alex's data, and the assistant says there's no one by that name in her family

#### Scenario: Summarizing a kid's chats
- **WHEN** Maya asks in "About the kids" what Lily has been chatting about
- **THEN** the assistant reads Lily's conversations and flags, summarizes them, and mentions any unreviewed flag

### Requirement: Read-only
The Assistant SHALL NOT change anything: it SHALL NOT mark flags reviewed, delete conversations, change profiles, birth years, calendars or widgets, or send anything to anyone. Asked to, it SHALL say where in the app the parent can do it.

#### Scenario: Asked to mark a flag reviewed
- **WHEN** Maya asks the assistant to mark Lily's flag as reviewed
- **THEN** nothing changes, and the assistant points her to the Reviews page

### Requirement: Replies stream, show tool notes, and link sources
Replies SHALL appear as they're written. Each tool use SHALL show as a short note above the reply:
- "Searched: <query>";
- "Looked at the family's day";
- "Checked the weather";
- "Looked at your picks";
- "Read the kids' chats".

The notes SHALL be stored with the conversation. Links in assistant replies SHALL be clickable and SHALL open in a new tab. Only `http` and `https` links SHALL become clickable.

#### Scenario: Research with sources
- **WHEN** Maya asks for a good beginner robotics kit
- **THEN** "Searched: …" notes show above the reply, and the reply names its sources with clickable links

### Requirement: Assistant usage is recorded per parent
Every model call for the Assistant SHALL be recorded in AI usage under the `parent_chat` feature, attributed to the parent, with no message content.

#### Scenario: One message with a search
- **WHEN** Maya sends one message and the assistant searches once before answering
- **THEN** two usage entries are recorded for Maya under `parent_chat`, one per model call
