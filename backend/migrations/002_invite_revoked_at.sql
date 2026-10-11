-- Revoking an invite now marks it revoked instead of deleting its row.
--
-- invites.id is a plain INTEGER PRIMARY KEY, and SQLite gives a new row the
-- highest id in the table plus one. Once the newest invite was deleted, the
-- next invite got its id, so a repeated (or stale) "revoke invite 14" would
-- quietly revoke someone else's brand new invite. A revoked row stays in
-- the table, so its id is never handed out again.
ALTER TABLE invites ADD COLUMN revoked_at DATETIME;
