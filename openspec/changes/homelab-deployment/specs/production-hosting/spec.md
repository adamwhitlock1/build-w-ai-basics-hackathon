## Purpose

Lets Kinlight run safely for one family on a home server that's reachable from the web: one server for the app and its API, production settings that can't be mixed with development ones, sign-up and login that hold up on a public page, and backups that can be restored.

## ADDED Requirements

### Requirement: Production mode
The server SHALL run in production mode when its public address (`KINLIGHT_PUBLIC_URL`, an `https://` URL) is set. In production mode:
- session cookies SHALL be sent only over HTTPS;
- the AI provider SHALL be told the public address as the app's site;
- every response SHALL carry security headers, including a Content-Security-Policy that allows the app's own scripts, styles, fonts and images, and nothing else.

The server SHALL refuse to start in production mode while a development-only setting is on: dev calendar fixtures, the kids' retention override, or Genkit's reflection server. It SHALL also refuse to start if the public address isn't an `https://` URL. Without a public address, the server SHALL behave as it does in development today.

#### Scenario: A development setting in production
- **WHEN** the server starts with a public address and `KINLIGHT_DEV_FIXTURES=1`
- **THEN** it doesn't start, and says the fixtures are for development only

#### Scenario: The session cookie
- **WHEN** Maya logs in on `https://kinlight.codestuff.io`
- **THEN** her session cookie is marked Secure, HttpOnly and SameSite=Lax

#### Scenario: The pre-paint theme script
- **WHEN** a page loads in production
- **THEN** the theme script in the page runs under the Content-Security-Policy, and no inline script without its hash would

### Requirement: One server for the app and the API
The server SHALL serve the built web app and the API from the same address. Any path that isn't the API or a built file SHALL be answered with the app's page, so links to app routes (an invite, a chat) work when opened directly. API paths that don't exist SHALL still get the API's JSON 404. Built files whose names carry a content hash SHALL be cached for a long time, and the app's page SHALL not be cached.

#### Scenario: Opening an invite link
- **WHEN** Grandma opens `https://kinlight.codestuff.io/invite/<token>`
- **THEN** the app loads and shows her invite

#### Scenario: A missing API path
- **WHEN** a request asks for `/api/nothing-here`
- **THEN** it gets the API's JSON 404, not the app's page

### Requirement: Closed sign-up in production
In production mode, anyone SHALL be able to create the first account, which creates the family. Once any family exists, sign-up SHALL be refused, and new members SHALL join only through an invite. The sign-up page SHALL say so, and point to asking a parent for an invite. Without production mode, sign-up SHALL stay open.

#### Scenario: A stranger finds the page
- **WHEN** someone who isn't invited tries to sign up on the deployed app after the Riveras have set up
- **THEN** the sign-up is refused, nothing is created, and the page says to ask a parent for an invite

#### Scenario: The first run
- **WHEN** Maya signs up on a freshly deployed, empty Kinlight
- **THEN** her account and family are created, and she can invite the others

### Requirement: Login throttling
After 5 failed logins within 15 minutes, for one email or from one client address, further login attempts for it SHALL be refused for 15 minutes with a message saying to wait. That holds whether the password is right or not, and the refusal SHALL NOT reveal whether the email has an account. A successful login SHALL clear the count for that email. When the server is configured to sit behind Cloudflare, the client address SHALL come from Cloudflare's connecting-IP header; otherwise, from the connection.

#### Scenario: Guessing a password
- **WHEN** someone fails to log in as maya@rivera.test six times in a few minutes
- **THEN** the sixth attempt is refused with "Too many tries. Wait 15 minutes and try again.", even with the right password

### Requirement: Backups that restore
The deployed database SHALL be replicated continuously to S3-compatible storage while the server runs. When the server's database file is missing at start and a backup exists, it SHALL be restored before the server starts. The deployment docs SHALL describe how to restore by hand, and how to test the restore.

#### Scenario: A lost volume
- **WHEN** the database volume is lost and the container starts again
- **THEN** the database is restored from the backup, and the family's accounts, pages and chats are there

#### Scenario: A first deployment
- **WHEN** the container starts with no database and no backup
- **THEN** it starts with an empty database, and replication begins
