-- Kid chat tool use: each tool the agent used while replying, shown as a
-- note to the kid and in parents' transcripts. message_id is the reply it
-- informed, or NULL when that reply failed (the note is kept, so parents
-- still see what was searched). detail is a search's query, or ''.
CREATE TABLE chat_tool_calls (
  id              INTEGER PRIMARY KEY,
  conversation_id INTEGER NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
  message_id      INTEGER REFERENCES chat_messages(id) ON DELETE CASCADE,
  tool            TEXT NOT NULL,
  detail          TEXT NOT NULL DEFAULT '',
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX chat_tool_calls_conversation ON chat_tool_calls(conversation_id, id);
