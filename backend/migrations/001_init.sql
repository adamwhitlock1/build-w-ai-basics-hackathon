-- Kinlight's first schema: accounts, families, invites, sessions,
-- per-member profiles and layouts, and user-described widgets.

-- One row per user. Family membership is via family_id.
CREATE TABLE users (
  id            INTEGER PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  password_hash BLOB NOT NULL,
  family_id     INTEGER NOT NULL REFERENCES families(id),
  is_admin      BOOLEAN NOT NULL DEFAULT 0,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- The family unit.
CREATE TABLE families (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Pending invites. Token is the URL-safe secret in the invite link.
-- is_admin marks a co-parent invite (from the create-family intro or the sidebar).
CREATE TABLE invites (
  id          INTEGER PRIMARY KEY,
  family_id   INTEGER NOT NULL REFERENCES families(id),
  email       TEXT NOT NULL,
  token       TEXT NOT NULL UNIQUE,
  is_admin    BOOLEAN NOT NULL DEFAULT 0,
  expires_at  DATETIME NOT NULL,
  accepted_at DATETIME
);

-- Server-side sessions for alexedwards/scs (sqlite3store expects this exact shape).
CREATE TABLE sessions (
  token  TEXT PRIMARY KEY,
  data   BLOB NOT NULL,
  expiry REAL NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions(expiry);

-- Per-user profile data: interests (freeform) and location (freeform).
CREATE TABLE member_profiles (
  user_id    INTEGER PRIMARY KEY REFERENCES users(id),
  interests  TEXT NOT NULL DEFAULT '',
  location   TEXT NOT NULL DEFAULT '',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Per-user homepage layout. `puck_data` is the JSON tree Puck produces.
CREATE TABLE homepage_layouts (
  user_id    INTEGER PRIMARY KEY REFERENCES users(id),
  puck_data  TEXT NOT NULL,  -- JSON
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- User-described widgets. `render_spec` is the LLM's chosen shape + content.
-- `source` and `query` are stored so we can re-fetch on demand.
CREATE TABLE custom_widgets (
  id          INTEGER PRIMARY KEY,
  user_id     INTEGER NOT NULL REFERENCES users(id),
  name        TEXT NOT NULL,
  source      TEXT NOT NULL,        -- "news_search" | "events_search" | "web_search"
  query       TEXT NOT NULL,
  render_spec TEXT NOT NULL,        -- JSON
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
