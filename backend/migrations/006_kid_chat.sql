-- Kid chat: kids' conversations with the family agent, their messages, and
-- the safety flags parents review.

-- A kid's birth year, set by a parent; it picks the agent's persona. NULL
-- means not set (the younger persona is used).
ALTER TABLE member_profiles ADD COLUMN birth_year INTEGER;

-- One conversation. mode is fixed when it starts. family_id is kept on the
-- row so every query can be scoped to one family without a join.
CREATE TABLE chat_conversations (
  id         INTEGER PRIMARY KEY,
  family_id  INTEGER NOT NULL REFERENCES families(id),
  member_id  INTEGER NOT NULL REFERENCES users(id),
  mode       TEXT NOT NULL CHECK (mode IN ('homework', 'explore')),
  title      TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX chat_conversations_member_time ON chat_conversations(member_id, updated_at);

-- Every message, the kid's and the agent's. Only complete agent replies are
-- stored. Deleting a conversation deletes its messages.
CREATE TABLE chat_messages (
  id              INTEGER PRIMARY KEY,
  conversation_id INTEGER NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
  role            TEXT NOT NULL CHECK (role IN ('kid', 'agent')),
  text            TEXT NOT NULL,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX chat_messages_conversation ON chat_messages(conversation_id, id);

-- A kid message the safety check flagged (or couldn't check), for parents
-- to review. reviewed_at and reviewed_by are NULL until a parent marks it.
CREATE TABLE chat_flags (
  id              INTEGER PRIMARY KEY,
  message_id      INTEGER NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
  conversation_id INTEGER NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
  family_id       INTEGER NOT NULL REFERENCES families(id),
  member_id       INTEGER NOT NULL REFERENCES users(id),
  category        TEXT NOT NULL,
  severity        TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high')),
  crisis          BOOLEAN NOT NULL DEFAULT 0,
  reason          TEXT NOT NULL DEFAULT '',
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  reviewed_at     DATETIME,
  reviewed_by     INTEGER REFERENCES users(id)
);
CREATE INDEX chat_flags_family_review ON chat_flags(family_id, reviewed_at);
