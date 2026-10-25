-- The parents' Assistant: each parent's private conversations, kept apart
-- from kid chat (whose tables carry flags and reviews, and whose mode CHECK
-- can't be widened without rebuilding the table).

-- kind is fixed when a conversation starts: 'anything' runs on the main
-- key; 'kids' runs on the kids' zero-retention key and can read kid chats.
CREATE TABLE assistant_conversations (
  id         INTEGER PRIMARY KEY,
  family_id  INTEGER NOT NULL REFERENCES families(id),
  member_id  INTEGER NOT NULL REFERENCES users(id),
  kind       TEXT NOT NULL CHECK (kind IN ('anything', 'kids')),
  title      TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX assistant_conversations_member_time ON assistant_conversations(member_id, updated_at);

CREATE TABLE assistant_messages (
  id              INTEGER PRIMARY KEY,
  conversation_id INTEGER NOT NULL REFERENCES assistant_conversations(id) ON DELETE CASCADE,
  role            TEXT NOT NULL CHECK (role IN ('parent', 'agent')),
  text            TEXT NOT NULL,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX assistant_messages_conversation ON assistant_messages(conversation_id, id);

-- Each tool the Assistant used while replying, shown as a note. message_id
-- is NULL when that reply failed.
CREATE TABLE assistant_tool_calls (
  id              INTEGER PRIMARY KEY,
  conversation_id INTEGER NOT NULL REFERENCES assistant_conversations(id) ON DELETE CASCADE,
  message_id      INTEGER REFERENCES assistant_messages(id) ON DELETE CASCADE,
  tool            TEXT NOT NULL,
  detail          TEXT NOT NULL DEFAULT '',
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX assistant_tool_calls_conversation ON assistant_tool_calls(conversation_id, id);
