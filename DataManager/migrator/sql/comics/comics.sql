#posts

CREATE TABLE IF NOT EXISTS comics(
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    modified TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS comic_chapter(
    id SERIAL PRIMARY KEY,
    comic_id INT NOT NULL REFERENCES comics(id),
    c_order INT NOT NULL,
    title TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS comic_page(
    id SERIAL PRIMARY KEY,
    post_id INT NOT NULL REFERENCES posts(id),
    page INT NOT NULL,
    chapter_id INT NOT NULL REFERENCES comic_chapter(id)
);

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

CREATE TRIGGER "comic_modified_tr"
AFTER INSERT ON "comic_mappings"
FOR EACH ROW
EXECUTE PROCEDURE update_comic_order();
