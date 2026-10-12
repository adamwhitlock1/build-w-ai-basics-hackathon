-- Each member's private calendar link (an iCal / ICS address), read by the
-- Today card. It's a secret: the server never sends it back to a browser.
-- '' means no calendar is connected.
ALTER TABLE member_profiles ADD COLUMN calendar_url TEXT NOT NULL DEFAULT '';
