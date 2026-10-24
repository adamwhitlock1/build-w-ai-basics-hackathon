# llm-usage Specification

## Purpose
Records how many AI tokens each family member's use of Porchlight consumes, broken down by feature, so that parents can later see where the usage goes and set limits, all without keeping anything anyone wrote or read.

## Requirements

### Requirement: Every AI call's usage is recorded
The system SHALL record one usage entry for every completed call to an AI model. Each entry SHALL contain:
- the time of the call;
- the model that answered;
- the feature it served (for example describing a widget, Articles, Local Events, the morning briefing or refreshing a widget);
- the step within that feature;
- the prompt, completion and total token counts the provider reported.

A call that is retried SHALL produce one entry per attempt that the provider answered. A call that fails before the provider answers (for example a network error) SHALL NOT produce an entry.

#### Scenario: Describing a widget records each AI step
- **WHEN** a member describes a widget and the system plans it and then shapes its search results with two AI calls
- **THEN** two usage entries are recorded for the describe-a-widget feature, one for each step, each with the model and the token counts the provider reported

#### Scenario: A call that never reaches the provider records nothing
- **WHEN** an AI call fails because the provider can't be reached
- **THEN** no usage entry is recorded for that attempt

### Requirement: Usage is attributed to the member it was for
Each usage entry SHALL name the family member whose content the call produced, and the member who caused it (the viewer). When a parent views a kid's page and that triggers an AI call, the entry SHALL be attributed to the kid as the member and to the parent as the viewer. A call that can't be attributed SHALL still be recorded with no member and feature `unattributed`, and SHALL be logged as a warning.

#### Scenario: A parent opening a kid's page
- **WHEN** a parent opens their kid's page and the kid's Articles card needs fresh AI-planned searches
- **THEN** the usage entry names the kid as the member and the parent as the viewer

#### Scenario: A member's own page
- **WHEN** a member's morning briefing is written on their own visit
- **THEN** the usage entry names that member as both the member and the viewer

### Requirement: Usage entries contain no message content
Usage entries SHALL NOT contain prompt text, reply text, search queries, interests, locations, calendar data, or any other content sent to or returned by the AI model. They SHALL contain only identifiers, names of features and steps, the model name, token counts and the time.

#### Scenario: A kid's widget request
- **WHEN** a kid describes a widget in a sentence
- **THEN** the recorded usage entries contain the token counts and labels, and nowhere contain the kid's sentence or the widget's results

### Requirement: Recording never breaks the feature
If recording a usage entry fails, the system SHALL log the failure and SHALL complete the member's request exactly as it would have otherwise. The member SHALL NOT see an error or a delay because of usage recording. A recording SHALL still be attempted when the member's request was cancelled after the provider answered.

#### Scenario: The usage table can't be written
- **WHEN** an AI call succeeds but its usage entry can't be saved
- **THEN** the member still gets their widget, and the failure appears in the server log

### Requirement: Usage can be summarized per member
The system SHALL be able to report, for one family and a time window, each member's total prompt, completion and total tokens and number of calls, broken down by feature. The summary SHALL include only that family's members.

#### Scenario: Summary for the last 7 days
- **WHEN** usage is summarized for the Rivera family over the last 7 days
- **THEN** each Rivera member with usage in that window appears with their totals per feature, and no member of another family appears
