## MODIFIED Requirements

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
