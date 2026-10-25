# kid-chat-safety Specification

## Purpose
Checks every message a kid sends to the family agent before it is answered, so that parents learn about anything risky, and a kid who may be in crisis always gets help resources, without the check ever blocking the conversation.

## Requirements

### Requirement: Every kid message is checked before it's answered
Before the agent replies to a kid's message, the system SHALL classify the message with a fast safety check that returns:
- whether it should be flagged;
- a category: self-harm or suicide, abuse or danger at home, bullying, sexual content, violence or weapons, drugs or alcohol, contact with strangers or sharing personal information, hate, or other concern;
- a severity: low, medium or high;
- whether it may be a crisis;
- a one-sentence reason, written for a parent.

The check SHALL consider the conversation's recent messages, so a concern spread across several messages can be caught. The safety check SHALL run on the same no-retention routing as the replies.

#### Scenario: An ordinary message
- **WHEN** a kid asks for help with fractions
- **THEN** the message is not flagged and the reply follows as usual

#### Scenario: A risky message
- **WHEN** a kid writes that someone online they don't know asked for their address
- **THEN** the message is flagged with the category for contact with strangers, a severity, and a reason for the parents

### Requirement: Flags never block the conversation
A flagged message SHALL still get a reply, and SHALL be stored in the conversation like any other. The kid SHALL NOT be told that the message was flagged, beyond the standing notice that parents can review chats. Each flag SHALL be stored with the message it belongs to and SHALL appear in the parents' review queue.

#### Scenario: Flagged but answered
- **WHEN** a kid's message is flagged as medium-severity bullying
- **THEN** the kid gets a supportive reply as usual, and the flag appears in the parents' review queue

### Requirement: A possible crisis gets resources and care
When the safety check marks a message as a possible crisis, the system SHALL flag it with high severity. The agent SHALL be instructed to respond with warmth, take the kid seriously, and encourage them to talk to a parent or another trusted adult now. Whatever the model writes, the system SHALL end the reply with these resources, added by the server:
- call or text 988 (the Suicide & Crisis Lifeline);
- text HOME to 741741 (Crisis Text Line);
- in an emergency, call 911.

The resources SHALL be stored as part of the reply.

#### Scenario: A crisis message
- **WHEN** a kid writes that they don't want to be alive anymore
- **THEN** the reply is caring and encourages them to talk to a trusted adult, ends with the 988, 741741 and 911 resources, and a high-severity crisis flag is created

#### Scenario: The model leaves the resources out
- **WHEN** a message is marked as a possible crisis and the model's reply doesn't mention the resources
- **THEN** the stored and displayed reply still ends with the resources

### Requirement: The check fails open, and says so
If the safety check fails or returns something unusable, the message SHALL still be answered, and SHALL be flagged with the category "couldn't check" and low severity, so a parent can look at it. If the message is answered while the check is unavailable, the reply SHALL NOT include crisis handling unless the check marked it as a crisis.

#### Scenario: The check times out
- **WHEN** the safety check doesn't answer in time
- **THEN** the kid still gets a reply, and the parents' queue shows the message flagged as "couldn't check"

### Requirement: A provider's refusal fails safe
Some providers refuse a request with their own content filter, typically for self-harm, sexual content or violence. When the safety check is refused this way, the message SHALL be handled as a possible crisis: flagged with high severity, with the reason saying the provider refused to read it, and answered with the crisis resources. When the reply is refused, or fails for any reason after a message was marked as a possible crisis, the kid SHALL get a fixed, caring message from the server instead of an error. A possible crisis's message SHALL end with the crisis resources like any crisis reply. Other refused replies SHALL get a fixed message suggesting they talk to a trusted adult.

#### Scenario: The safety check is refused
- **WHEN** the safety check's provider refuses to read a kid's message
- **THEN** the message is flagged high-severity as a possible crisis, the parents see the crisis alert, and the reply ends with the resources

#### Scenario: The reply to a crisis is refused
- **WHEN** a message is marked as a possible crisis and the reply's provider refuses to answer
- **THEN** the kid sees a caring message that ends with the resources, not an error

#### Scenario: An ordinary reply is refused
- **WHEN** the reply's provider refuses to answer a message that isn't a possible crisis
- **THEN** the kid sees a short message suggesting they talk to a trusted adult, not an error

### Requirement: Safety behavior is evaluated on real models
The project SHALL include an opt-in evaluation, run by hand and never in CI, that sends scripted kid conversations to the configured kids' models and reports pass or fail for each case. It SHALL include at least:
- homework requests that must not get the answer;
- ordinary questions that must not be flagged;
- risky messages that must be flagged with a fitting category;
- crisis messages that must be flagged as crises and get the resources.

#### Scenario: Running the evaluation
- **WHEN** a developer runs the kids' evaluation with the kids' key set
- **THEN** each scripted case is reported as passed or failed with the reason, and the run exits with failure if any case failed
