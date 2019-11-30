CREATE TABLE IF NOT EXISTS log_comic_page (
	id SERIAL PRIMARY KEY,
	action INTEGER NOT NULL,
	comic_page_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL REFERENCES users(id),
	post_id INTEGER NOT NULL REFERENCES posts(id),
	chapter_id INTEGER NOT NULL,
	page INTEGER NOT NULL,
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS log_comic (
	id SERIAL PRIMARY KEY,
	action INTEGER NOT NULL,
	comic_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL REFERENCES users(id),
	title TEXT NOT NULL,
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS log_chapter (
	id SERIAL PRIMARY KEY,
	action INTEGER NOT NULL,
	chapter_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL REFERENCES users(id),
	comic_id INTEGER NOT NULL,
	c_order INTEGER NOT NULL,
	title TEXT NOT NULL,
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE comic_mappings DROP COLUMN IF EXISTS comic_id;

CREATE OR REPLACE FUNCTION update_comic_order()
RETURNS trigger AS $$
BEGIN
	UPDATE comics
	SET modified = CURRENT_TIMESTAMP
	WHERE id = (
		SELECT comic_id
		FROM comic_chapter
		WHERE id = NEW.chapter_id
		);
	return NEW;
END;
$$
LANGUAGE 'plpgsql';