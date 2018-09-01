CREATE TABLE comic_chapter(
    id INTEGER PRIMARY KEY,
    comic_id INTEGER NOT NULL,
    c_order INTEGER NOT NULL,
    title TEXT NOT NULL,
    FOREIGN KEY (comic_id) REFERENCES comics(id),
    CONSTRAINT unique_comic_chapter_constraint UNIQUE (comic_id, c_order)
);

ALTER TABLE comic_mappings ADD COLUMN chapter_id INTEGER NOT NULL REFERENCES comic_chapter(id) DEFAULT 0;