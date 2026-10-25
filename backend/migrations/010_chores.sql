CREATE TABLE chores (
 id INTEGER PRIMARY KEY,
 family_id INTEGER NOT NULL REFERENCES families(id),
 assignee_user_id INTEGER NOT NULL REFERENCES users(id),
 title TEXT NOT NULL CHECK(length(trim(title)) BETWEEN 1 AND 120),
 created_by_user_id INTEGER NOT NULL REFERENCES users(id),
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 archived_at DATETIME
);
CREATE INDEX chores_member ON chores(family_id, assignee_user_id, archived_at);
CREATE TABLE chore_completions (
 chore_id INTEGER NOT NULL REFERENCES chores(id),
 local_date TEXT NOT NULL,
 completed_by_user_id INTEGER NOT NULL REFERENCES users(id),
 completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(chore_id, local_date)
);
