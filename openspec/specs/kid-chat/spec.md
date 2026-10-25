# kid-chat Specification

## Purpose
Gives each kid in the family an AI they can talk to: a tutor that helps them learn without doing their assignments for them, and a companion for curiosity and creative projects. It speaks at their age, keeps their conversations, and only ever runs on providers that keep nothing.

## Requirements

### Requirement: Only kids chat with the family agent
Kid chat SHALL be available only to family members who aren't parents. A parent's request to start a conversation or send a message in kid chat SHALL be refused with a message pointing them to their own Assistant. A kid SHALL only ever see and use their own conversations.

#### Scenario: A kid opens Chat
- **WHEN** Jordan, a kid, opens the Chat page
- **THEN** Jordan sees their own conversations, newest first, and can start a new one

#### Scenario: A parent tries to chat
- **WHEN** Maya, a parent, asks to start a kid chat conversation for herself
- **THEN** the request is refused with a message pointing her to the Assistant, and nothing is stored or sent to any AI model

#### Scenario: Another kid's conversation
- **WHEN** Lily asks for one of Jordan's conversations by id
- **THEN** the system answers as if the conversation doesn't exist

### Requirement: Two conversation modes
Each conversation SHALL have a mode, chosen when it starts:
- **Homework help:** the agent SHALL act as a tutor.
- **Explore:** the agent SHALL act as a curiosity and creativity companion for questions, ideas, stories and projects. It SHALL still follow the homework rule below if the kid brings an assignment.

The mode SHALL be shown on the conversation and SHALL NOT change after the conversation starts.

#### Scenario: Starting in homework help
- **WHEN** a kid starts a conversation and picks Homework help
- **THEN** the conversation is marked Homework help, and the agent's first reply asks what they're working on and what they've tried

#### Scenario: Exploring
- **WHEN** a kid starts an Explore conversation and asks why the sky is blue
- **THEN** the agent explains at the kid's level and invites a follow-up question

### Requirement: The agent guides rather than answers assignments
When a kid's message looks like an assignment (a homework or test question, a worksheet problem, or an essay or report to write), the agent SHALL NOT give the final answer or write the work for them, in either mode. It SHALL help them climb a hint ladder, one step per turn:
1. ask what they've tried or what they think;
2. give a hint about the idea or the next step;
3. show a worked example of a similar but different problem;
4. check their answer and explain any mistake.

If a kid asks for the answer outright, the agent SHALL explain kindly that it's there to help them work it out, and give the next hint. It SHALL confirm a correct answer the kid reaches themselves.

#### Scenario: Asking for the answer
- **WHEN** a kid in Homework help writes "what's the answer to 3x + 5 = 20"
- **THEN** the reply doesn't contain the solution x = 5, and instead asks what they've tried or gives a first step

#### Scenario: Asking twice
- **WHEN** the kid replies "just tell me the answer"
- **THEN** the reply explains that it's there to help them work it out, gives the next hint, and still doesn't state x = 5

#### Scenario: Checking the kid's answer
- **WHEN** the kid later writes "is it x = 5?"
- **THEN** the agent confirms the answer is right and briefly says why

#### Scenario: Writing an essay
- **WHEN** a kid asks the agent to write their book report
- **THEN** the agent doesn't write it, and offers to help them plan, outline or improve their own draft

### Requirement: Age personas
The agent SHALL speak at the kid's age, using a younger persona for kids aged 12 and under and a teen persona for ages 13 to 17. Age SHALL come from the kid's birth year, which a parent can set in the kid's profile and the kid can't change. A kid with no birth year SHALL get the younger persona. The kid-safe guidance used elsewhere in Porchlight SHALL apply to every reply.

#### Scenario: A parent sets a birth year
- **WHEN** Maya sets Jordan's birth year so that Jordan is 15
- **THEN** Jordan's conversations use the teen persona from the next reply on

#### Scenario: No birth year
- **WHEN** Lily has no birth year set and sends a message
- **THEN** the reply uses the younger persona

#### Scenario: A kid can't change their age
- **WHEN** Jordan tries to change their own birth year
- **THEN** the change is refused and the birth year stays as the parent set it

### Requirement: Conversations are titled and kept
Each conversation SHALL get a short title from its first message; when the safety check flags that message (or couldn't check it), the title SHALL be the neutral "A conversation" instead, so nothing upsetting shows in the kid's list. Each conversation SHALL keep every message, the kid's and the agent's, with the time it was sent. Conversations SHALL be kept until a parent deletes them; kids SHALL NOT be able to delete or edit them. The agent SHALL see the conversation so far when replying, up to a limit, after which only the most recent messages are used.

#### Scenario: A new conversation gets a title
- **WHEN** a kid starts a conversation with "how do volcanoes form?"
- **THEN** the conversation appears in their list with a short title such as "How volcanoes form"

#### Scenario: A flagged first message gets a neutral title
- **WHEN** a kid starts a conversation with a message the safety check flags as a possible crisis
- **THEN** the conversation is titled "A conversation", not after what the message says

#### Scenario: Coming back later
- **WHEN** a kid opens a conversation from last week
- **THEN** every message is there in order, and a new message continues the same conversation with the earlier messages as context

#### Scenario: A kid tries to delete
- **WHEN** a kid asks to delete one of their conversations
- **THEN** the request is refused and the conversation stays

### Requirement: Replies stream in
The agent's reply SHALL appear as it is written. If the reply fails partway, the kid SHALL see what arrived plus a short, kind message to try again, and only a complete reply SHALL be stored as the agent's message. The kid's message SHALL be stored whether or not the reply succeeds.

#### Scenario: A normal reply
- **WHEN** a kid sends a message
- **THEN** the reply's text appears progressively, and once finished it is stored in the conversation

#### Scenario: The provider fails
- **WHEN** the AI provider fails while replying
- **THEN** the kid sees a message to try again in a moment, their own message is still in the conversation, and no partial reply is stored

### Requirement: The parent notice is always shown
Every screen where a kid chats or reads a conversation, and the chat card on the kid's homepage, SHALL show the notice "Your parents can review these chats".

#### Scenario: Opening a conversation
- **WHEN** a kid opens any conversation
- **THEN** "Your parents can review these chats" is visible without scrolling

### Requirement: Kid chat runs only on no-retention providers
Every AI call for kid chat, both the replies and the safety check, SHALL go through the kids' own provider key, and SHALL be restricted to endpoints that keep no data and don't collect it for training. If the kids' key isn't configured, kid chat SHALL be off: kids SHALL see "Chat isn't set up yet. Ask a parent.", and no message SHALL be stored or sent anywhere. A development-only setting MAY allow other providers for prototyping. When it is on, the server SHALL log a warning at startup that kid chat isn't restricted to no-retention providers.

#### Scenario: Kids' key configured
- **WHEN** the kids' key is set and a kid sends a message
- **THEN** both the safety check and the reply are requested with zero-data-retention and no-data-collection routing, using the kids' key

#### Scenario: No kids' key
- **WHEN** the kids' key isn't set and a kid opens Chat
- **THEN** the page says "Chat isn't set up yet. Ask a parent.", and sending a message is refused without storing it

#### Scenario: Prototyping override
- **WHEN** the development-only override is on
- **THEN** the server logs a warning at startup that kid chat isn't restricted to no-retention providers

### Requirement: Kid chat usage is recorded per kid
Every model call for kid chat SHALL be recorded in AI usage under the kid-chat feature, with separate steps for the safety check and the reply, attributed to the kid. Usage entries SHALL contain no message content.

#### Scenario: One kid message
- **WHEN** Jordan sends one message and gets a reply
- **THEN** two usage entries are recorded for Jordan under the kid-chat feature, one for the safety check and one for the reply, with token counts and no text
