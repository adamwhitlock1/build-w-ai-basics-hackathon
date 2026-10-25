# parent-review Specification

## Purpose
Lets parents see what their kids ask the family agent: a review queue of flagged messages, crisis first, full transcripts of every conversation, a hard-to-miss alert for possible crises, and the ability to delete conversations.

## Requirements

### Requirement: Parents have a review queue
Parents SHALL have a Reviews page listing their family's flagged kid messages. Unreviewed flags SHALL come first: crisis flags, then the rest by severity (high, medium, low), newest first within each group. Each entry SHALL show:
- the kid;
- the time;
- the category and severity, and whether it's a possible crisis;
- the safety check's reason;
- the flagged message.

It SHALL link to the full conversation with that message highlighted. A parent SHALL be able to mark a flag as reviewed, which records who reviewed it and when, and moves it to a reviewed section. Any parent in the family SHALL see the same queue. Kids SHALL NOT see the queue or any flags.

#### Scenario: Crisis first
- **WHEN** a family has an unreviewed low-severity flag from yesterday and an unreviewed crisis flag from last week
- **THEN** the crisis flag is listed first

#### Scenario: Marking reviewed
- **WHEN** Maya marks a flag as reviewed
- **THEN** it moves to the reviewed section showing "Reviewed by Maya" and the time, and Sam sees it as reviewed too

#### Scenario: A kid asks for the queue
- **WHEN** Jordan requests the review queue
- **THEN** the request is refused, and no flags are revealed

#### Scenario: Another family's flags
- **WHEN** a parent of another family requests the queue
- **THEN** none of the Rivera family's flags appear

### Requirement: Parents can read every conversation
A parent SHALL be able to list each kid's conversations (title, mode, last activity, and how many unreviewed flags each has), and to read any conversation in full, with flagged messages marked. Reading a conversation SHALL NOT change it, and SHALL NOT be visible to the kid.

#### Scenario: Reading Jordan's chats
- **WHEN** Maya opens Jordan's conversations
- **THEN** she sees every conversation Jordan has had, and can open any of them to read every message, with flagged ones marked

#### Scenario: Members of another family
- **WHEN** a parent requests the conversations of a kid in another family
- **THEN** the system answers as if the kid doesn't exist

### Requirement: Parents can delete a conversation
A parent SHALL be able to delete a kid's conversation after confirming. Deleting it SHALL permanently remove its messages and flags. Usage entries SHALL remain, since they contain no content.

#### Scenario: Deleting
- **WHEN** Maya deletes one of Jordan's conversations and confirms
- **THEN** it disappears from Jordan's list and hers, its flags leave the review queue, and AI usage totals are unchanged

### Requirement: An unreviewed crisis is hard to miss
While any crisis flag in the family is unreviewed, every page a parent opens SHALL show a prominent alert naming the kid. It SHALL link to the flag in the review queue. The alert SHALL disappear once every crisis flag has been reviewed. The parents' Reviews link SHALL show the number of unreviewed flags.

#### Scenario: A new crisis flag
- **WHEN** Jordan's message is flagged as a possible crisis and Sam opens any page
- **THEN** Sam sees an alert at the top saying that Jordan may need help now, linking to the flag

#### Scenario: Reviewed
- **WHEN** a parent marks the crisis flag as reviewed
- **THEN** the alert no longer shows for either parent
