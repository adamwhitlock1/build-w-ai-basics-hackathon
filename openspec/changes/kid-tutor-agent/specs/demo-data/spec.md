## ADDED Requirements

### Requirement: Demo kids' ages and conversations
The Rivera demo family's kids SHALL have birth years: Jordan SHALL be 15 and Lily 9 when the demo is built. The demo SHALL include sample kid conversations, so that kid chat and parent review can be tested without spending anything:
- Jordan SHALL have a Homework help conversation in which the agent gives hints rather than the answer;
- Jordan SHALL have an Explore conversation;
- Lily SHALL have one Explore conversation, with one unreviewed medium-severity flag for contact with strangers.

The demo SHALL NOT contain a crisis flag, so the crisis alert shows only when one is created while testing. The second family's kid SHALL have no conversations. Rebuilding the demo SHALL restore these exactly.

#### Scenario: Parent review in the demo
- **WHEN** someone logs in as Maya after rebuilding the demo
- **THEN** the review queue shows Lily's one unreviewed flag, and no crisis alert is shown

#### Scenario: A kid's conversations in the demo
- **WHEN** someone logs in as Jordan
- **THEN** the Chat page lists Jordan's two sample conversations, and opening them costs no AI call

#### Scenario: Ages pick personas
- **WHEN** the demo is rebuilt
- **THEN** Jordan's birth year makes Jordan 15 (teen persona), and Lily's makes Lily 9 (younger persona)
