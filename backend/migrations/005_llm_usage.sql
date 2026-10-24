-- One row per AI model answer: how many tokens it used, for which feature
-- and step, and for whom. It never holds anything anyone wrote or read:
-- no prompts, replies, queries, interests, locations or calendar data.
--
-- member_id is whose content the call produced; viewer_id is who caused it
-- (a parent opening a kid's page). Both are NULL for a call that couldn't be
-- attributed. family_id is kept on the row so a family's summary needs no
-- join and can never pick up another family's rows.
CREATE TABLE llm_usage (
  id                INTEGER PRIMARY KEY,
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  family_id         INTEGER REFERENCES families(id),
  member_id         INTEGER REFERENCES users(id),
  viewer_id         INTEGER REFERENCES users(id),
  feature           TEXT NOT NULL,
  step              TEXT NOT NULL,
  model             TEXT NOT NULL,
  prompt_tokens     INTEGER NOT NULL,
  completion_tokens INTEGER NOT NULL,
  total_tokens      INTEGER NOT NULL
);
CREATE INDEX llm_usage_family_time ON llm_usage(family_id, created_at);
