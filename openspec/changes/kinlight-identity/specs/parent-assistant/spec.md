## ADDED Requirements

### Requirement: The Assistant refers to the kids by name
When the Assistant talks about a kid or another family member, it SHALL refer to them by their name, and SHALL NOT guess their pronouns from a name. It MAY use pronouns a parent used for that person in the conversation.

#### Scenario: Summarizing the kids' chats
- **WHEN** Maya asks how the kids are doing with their chats
- **THEN** the reply talks about Jordan and Lily by name, and doesn't use "he", "she", "his" or "her" for a kid unless Maya did
