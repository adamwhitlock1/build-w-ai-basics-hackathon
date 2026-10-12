-- Widgets remember their results, so a page opens without searching again,
-- and described widgets can be refreshed.

-- Each member's latest Articles and Local Events. `inputs` is the interests
-- and location the result was picked for: when the member's profile no
-- longer matches it, the result is picked again. `payload` is the widget
-- JSON, as sent to the browser.
CREATE TABLE curated_widget_results (
  user_id    INTEGER NOT NULL REFERENCES users(id),
  kind       TEXT NOT NULL,       -- "articles" | "events"
  inputs     TEXT NOT NULL,
  payload    TEXT NOT NULL,       -- JSON
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (user_id, kind)
);

-- The sentence a widget was described with, so a refresh can shape new
-- results the way the first build did. Widgets made before this migration
-- keep '' and refresh using their title instead.
ALTER TABLE custom_widgets ADD COLUMN prompt TEXT NOT NULL DEFAULT '';

-- When the widget's items were last refreshed. NULL means never: they date
-- from created_at.
ALTER TABLE custom_widgets ADD COLUMN refreshed_at DATETIME;
