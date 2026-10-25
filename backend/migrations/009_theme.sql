-- Each member's theme: "auto" follows their device's light or dark setting;
-- "light" is always Daybreak and "dark" always Porch light. Only the member
-- sets it, so it lives on users rather than member_profiles, which parents
-- edit.
ALTER TABLE users ADD COLUMN theme TEXT NOT NULL DEFAULT 'auto' CHECK (theme IN ('auto', 'light', 'dark'));
