#spine
#comics

CREATE TABLE log_comics (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	id INTEGER NOT NULL,
	action log_action NOT NULL,
	title TEXT NOT NULL
);

CREATE TABLE log_chapters (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	comic_id INTEGER NOT NULL,
	chapter_id INTEGER NOT NULL,
	c_order INTEGER NOT NULL,
	title TEXT NOT NULL
);

CREATE TABLE log_comic_page (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	comic_page_id INTEGER NOT NULL,
	chapter_id INTEGER NOT NULL,
	post_id INTEGER NOT NULL,
	page INTEGER NOT NULL
);

CREATE VIEW view_log_comic_page_diff AS
SELECT
	new.comic_page_id, new.action, new.log_id,
	new.chapter_id, new.post_id, new.page,
	old.log_id AS old_log_id,
	old.chapter_id AS old_chapter_id,
	old.post_id AS old_post_id,
	old.page AS old_page
FROM log_comic_page new
LEFT JOIN log_comic_page old
ON new.comic_page_id = old.comic_page_id
AND old.log_id = (
	SELECT MAX(log_id)
	FROM log_comic_page
	WHERE log_id < new.log_id
	AND comic_page_id = new.comic_page_id
);

