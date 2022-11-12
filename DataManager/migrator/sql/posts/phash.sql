#posts

CREATE TABLE IF NOT EXISTS phash(
    post_id INT PRIMARY KEY REFERENCES posts(id),
    h1 INT NOT NULL,
    h2 INT NOT NULL,
    h3 INT NOT NULL,
    h4 INT NOT NULL
);

CREATE INDEX phash_h1_index ON phash (h1);
CREATE INDEX phash_h2_index ON phash (h2);
CREATE INDEX phash_h3_index ON phash (h3);
CREATE INDEX phash_h4_index ON phash (h4);
