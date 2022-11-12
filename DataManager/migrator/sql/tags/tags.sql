#namespace

CREATE TABLE IF NOT EXISTS tags(
    id SERIAL PRIMARY KEY,
    tag VARCHAR(256) NOT NULL,
    namespace_id INTEGER NOT NULL REFERENCES namespaces(id),
    count INT NOT NULL DEFAULT 0,
    CONSTRAINT tag_namespace UNIQUE (tag, namespace_id)
);

CREATE VIEW tag AS
	SELECT id, tag, nspace AS namespace, count
	FROM tags
	JOIN namespaces n
	ON namespace_id = n.id;
