CREATE TABLE dbinfo(
    ver INTEGER NOT NULL
);
INSERT INTO dbinfo(ver) VALUES (0);

CREATE TABLE posts(
    id INTEGER PRIMARY KEY,
    multihash CHAR(46) UNIQUE NOT NULL,
    thumb_hash CHAR(46),
    uploader INTEGER
);

CREATE TABLE namespaces(
    id INTEGER PRIMARY KEY,
    nspace VARCHAR(64) NOT NULL
);

CREATE TABLE tags(
    id INTEGER PRIMARY KEY,
    tag VARCHAR(64) NOT NULL,
    namespace_id INTEGER NOT NULL,
    FOREIGN KEY(namespace_id) REFERENCES namespaces(id),
    CONSTRAINT tag_namespace UNIQUE (tag, namespace_id)
);